package mission

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	credentialAccessTokenType = "mission-credential+jws"
	credentialTokenUse        = "credential_access"
	projectionGrantTokenUse   = "projection_grant"
	credentialTokenType       = "mission_credential_access"
)

func (s *Service) ExchangeProjectionToken(req ExchangeProjectionTokenRequest) (CredentialAccessTokenResponse, error) {
	projectionToken := strings.TrimSpace(req.ProjectionToken)
	if projectionToken == "" {
		return CredentialAccessTokenResponse{}, fmt.Errorf("projection_token is required")
	}
	nonce := strings.TrimSpace(req.Nonce)
	if nonce == "" {
		return CredentialAccessTokenResponse{}, fmt.Errorf("nonce is required")
	}
	grantPayload, err := VerifyProjectionToken(projectionToken, s.artifactKey)
	if err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	projection, err := s.projections.GetProjection(grantPayload.ProjectionID)
	if err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	if projection.Token != projectionToken {
		return CredentialAccessTokenResponse{}, fmt.Errorf("projection token does not match broker state")
	}
	if projectionExchangeNonceUsed(projection, nonce) {
		return CredentialAccessTokenResponse{}, ErrConflict
	}
	mission, err := s.missions.GetMission(projection.MissionRef)
	if err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	now := s.clock.Now()
	if err := validateProjectionForBroker(projection, mission, req.Actor, now); err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	if rule, ok, err := s.matchingActiveContainmentForProjection(projection, now); err != nil {
		return CredentialAccessTokenResponse{}, err
	} else if ok {
		return CredentialAccessTokenResponse{}, fmt.Errorf("projection blocked by containment rule %s", rule.RuleID)
	}
	scopes, err := requestedCredentialScopes(req.RequestedScopes, projection)
	if err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	audience, toolName, resource, operation, err := requestedCredentialBinding(req, projection, mission)
	if err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	expiresAt := boundedBrokerExpiry(now, req.TTLSeconds, projection.ExpiresAt, mission.Lifecycle.ExpiresAt)
	exchangeID := newID("cex")
	jti := newID("ctok")
	payload := CredentialAccessTokenPayload{
		JTI:            jti,
		Issuer:         "auth-scope",
		Subject:        req.Actor.AgentInstanceID,
		Audience:       audience,
		TokenUse:       credentialTokenUse,
		ExchangeID:     exchangeID,
		ProjectionID:   projection.ProjectionID,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		TenantID:       mission.TenantID,
		Type:           projection.Type,
		Actor:          req.Actor,
		Agent:          mission.Agent,
		AuthorityHash:  HashDecisionContext(map[string]any{"authority_region": mission.AuthorityRegion}),
		Scopes:         scopes,
		ToolName:       toolName,
		Resource:       resource,
		Operation:      operation,
		Confirmation:   projectionConfirmation(req.Actor),
		IssuedAt:       now,
		NotBefore:      now,
		ExpiresAt:      expiresAt,
	}
	accessToken, err := SignCredentialAccessToken(payload, s.artifactKey)
	if err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	record := CredentialExchangeRecord{
		ExchangeID:      exchangeID,
		ProjectionID:    projection.ProjectionID,
		JTI:             jti,
		Nonce:           nonce,
		Actor:           req.Actor,
		Scopes:          scopes,
		Audience:        audience,
		ToolName:        toolName,
		Resource:        cloneActionResource(resource),
		Operation:       operation,
		AccessTokenHash: hashAccessToken(accessToken),
		Status:          ProjectionStatusActive,
		IssuedAt:        now,
		ExpiresAt:       expiresAt,
	}
	projection.ExchangeRecords = append(projection.ExchangeRecords, record)
	if err := s.projections.UpdateProjection(projection); err != nil {
		return CredentialAccessTokenResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    projection.MissionRef,
		TenantID:      projection.TenantID,
		Type:          "mission.credential_exchanged",
		Actor:         map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID, "key_thumbprint": req.Actor.KeyThumbprint},
		Payload:       map[string]any{"projection_id": projection.ProjectionID, "exchange_id": exchangeID, "scopes": scopes, "audience": audience, "tool_name": toolName, "operation": operation, "expires_at": expiresAt},
		VersionBefore: projection.MissionVersion,
		VersionAfter:  projection.MissionVersion,
		OccurredAt:    now,
	})
	return CredentialAccessTokenResponse{
		AccessToken:    accessToken,
		TokenType:      credentialTokenType,
		ExchangeID:     exchangeID,
		JTI:            jti,
		ProjectionID:   projection.ProjectionID,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		TenantID:       mission.TenantID,
		Actor:          req.Actor,
		Scopes:         scopes,
		Audience:       audience,
		ToolName:       toolName,
		Resource:       cloneActionResource(resource),
		Operation:      operation,
		ExpiresIn:      int(expiresAt.Sub(now).Seconds()),
		ExpiresAt:      expiresAt,
	}, nil
}

func (s *Service) VerifyCredentialAccessToken(req VerifyCredentialAccessTokenRequest) VerifyCredentialAccessTokenResponse {
	if strings.TrimSpace(req.Token) == "" {
		return VerifyCredentialAccessTokenResponse{Valid: false, Error: "token is required"}
	}
	payload, err := VerifyCredentialAccessTokenPayload(req.Token, s.artifactKey)
	if err != nil {
		return VerifyCredentialAccessTokenResponse{Valid: false, Error: err.Error()}
	}
	projection, err := s.projections.GetProjection(payload.ProjectionID)
	if err != nil {
		return VerifyCredentialAccessTokenResponse{Valid: false, Payload: payload, Error: err.Error()}
	}
	record, ok := findCredentialExchangeRecord(projection, payload.ExchangeID, payload.JTI)
	if !ok {
		return VerifyCredentialAccessTokenResponse{Valid: false, Payload: payload, Projection: &projection, Error: "credential exchange not found"}
	}
	if record.AccessTokenHash != "" && record.AccessTokenHash != hashAccessToken(req.Token) {
		return VerifyCredentialAccessTokenResponse{Valid: false, Payload: payload, Projection: &projection, Exchange: &record, Error: "credential token does not match broker state"}
	}
	mission, err := s.missions.GetMission(projection.MissionRef)
	if err != nil {
		return VerifyCredentialAccessTokenResponse{Valid: false, Payload: payload, Projection: &projection, Exchange: &record, Error: err.Error()}
	}
	now := s.clock.Now()
	if err := validateCredentialAccessToken(projection, record, payload, mission, req, now); err != nil {
		return VerifyCredentialAccessTokenResponse{Valid: false, Payload: payload, Projection: &projection, Exchange: &record, Error: err.Error()}
	}
	if rule, ok, err := s.matchingActiveContainmentForProjection(projection, now); err != nil {
		return VerifyCredentialAccessTokenResponse{Valid: false, Payload: payload, Projection: &projection, Exchange: &record, Error: err.Error()}
	} else if ok {
		return VerifyCredentialAccessTokenResponse{Valid: false, Payload: payload, Projection: &projection, Exchange: &record, Error: "credential blocked by containment rule " + rule.RuleID}
	}
	return VerifyCredentialAccessTokenResponse{Valid: true, Payload: payload, Projection: &projection, Exchange: &record}
}

func validateProjectionForBroker(projection Projection, mission Mission, actor Actor, now time.Time) error {
	if projection.Status == ProjectionStatusRevoked || !projection.RevokedAt.IsZero() {
		return fmt.Errorf("projection revoked")
	}
	if now.After(projection.ExpiresAt) {
		return fmt.Errorf("projection expired")
	}
	if err := ensureMissionUsableForActor(mission, actor, projection.MissionVersion, now); err != nil {
		return err
	}
	if !actorsEqual(projection.Actor, actor) {
		return fmt.Errorf("projection actor mismatch")
	}
	return nil
}

func validateCredentialAccessToken(projection Projection, record CredentialExchangeRecord, payload CredentialAccessTokenPayload, mission Mission, req VerifyCredentialAccessTokenRequest, now time.Time) error {
	if payload.TokenUse != credentialTokenUse {
		return fmt.Errorf("credential token_use mismatch")
	}
	if payload.ProjectionID != projection.ProjectionID || payload.ExchangeID != record.ExchangeID || payload.JTI != record.JTI {
		return fmt.Errorf("credential binding mismatch")
	}
	if record.Status == ProjectionStatusRevoked || !record.RevokedAt.IsZero() {
		return fmt.Errorf("credential revoked")
	}
	if projection.Status == ProjectionStatusRevoked || !projection.RevokedAt.IsZero() {
		return fmt.Errorf("projection revoked")
	}
	if now.Before(payload.NotBefore) {
		return fmt.Errorf("credential not active yet")
	}
	if !now.Before(payload.ExpiresAt) || now.After(record.ExpiresAt) {
		return fmt.Errorf("credential expired")
	}
	if now.After(projection.ExpiresAt) {
		return fmt.Errorf("projection expired")
	}
	if err := ensureMissionUsableForActor(mission, payload.Actor, payload.MissionVersion, now); err != nil {
		return err
	}
	if !actorsEqual(projection.Actor, payload.Actor) || !actorsEqual(record.Actor, payload.Actor) {
		return fmt.Errorf("credential actor mismatch")
	}
	if actorProvided(req.Actor) && !actorsEqual(req.Actor, payload.Actor) {
		return fmt.Errorf("credential actor mismatch")
	}
	if req.Audience != "" && req.Audience != payload.Audience {
		return fmt.Errorf("credential audience mismatch")
	}
	if req.ToolName != "" && req.ToolName != payload.ToolName {
		return fmt.Errorf("credential tool mismatch")
	}
	if req.Operation != "" && req.Operation != payload.Operation {
		return fmt.Errorf("credential operation mismatch")
	}
	if req.Resource != nil && !actionResourcesEqual(req.Resource, payload.Resource) {
		return fmt.Errorf("credential resource mismatch")
	}
	if !stringSubset(payload.Scopes, projectionAllowedScopes(projection)) {
		return fmt.Errorf("credential scope exceeds projection")
	}
	return nil
}

func requestedCredentialScopes(requested []string, projection Projection) ([]string, error) {
	scopes := cleanSortedStrings(requested)
	allowed := projectionAllowedScopes(projection)
	if len(scopes) == 0 {
		return allowed, nil
	}
	if !stringSubset(scopes, allowed) {
		return nil, fmt.Errorf("requested scopes exceed projection scope")
	}
	return scopes, nil
}

func requestedCredentialBinding(req ExchangeProjectionTokenRequest, projection Projection, mission Mission) (string, string, *ActionResource, string, error) {
	audience := strings.TrimSpace(req.Audience)
	if audience == "" {
		audience = projection.Audience
	}
	if projection.Audience != "" && audience != projection.Audience {
		return "", "", nil, "", fmt.Errorf("requested audience does not match projection")
	}
	toolName := strings.TrimSpace(req.ToolName)
	if toolName == "" {
		toolName = projection.ToolName
	}
	if projection.ToolName != "" && toolName != projection.ToolName {
		return "", "", nil, "", fmt.Errorf("requested tool does not match projection")
	}
	resource := cleanActionResource(req.Resource)
	if resource == nil {
		resource = cloneActionResource(projection.Resource)
	}
	if projection.Resource != nil && !actionResourcesEqual(resource, projection.Resource) {
		return "", "", nil, "", fmt.Errorf("requested resource does not match projection")
	}
	operation := strings.TrimSpace(req.Operation)
	if operation == "" {
		operation = projection.Operation
	}
	if projection.Operation != "" && operation != projection.Operation {
		return "", "", nil, "", fmt.Errorf("requested operation does not match projection")
	}
	if resource != nil || operation != "" {
		if resource == nil || resource.Type == "" || resource.ID == "" || operation == "" {
			return "", "", nil, "", fmt.Errorf("credential resource.type, resource.id, and operation are required when binding a credential to a tool action")
		}
		action := Action{Type: "tool_call", Name: toolName, Resource: *resource, Operation: operation}
		if !actionInScope(mission.AuthorityRegion, action) {
			return "", "", nil, "", fmt.Errorf("requested credential action is outside mission authority")
		}
	}
	return audience, toolName, resource, operation, nil
}

func projectionRequestedScopes(requested []string, claims map[string]any, authority AuthorityRegion) []string {
	scopes := cleanSortedStrings(requested)
	if len(scopes) > 0 {
		return scopes
	}
	scopes = scopesFromProjectionClaims(claims)
	if len(scopes) > 0 {
		return scopes
	}
	return scopesFromAuthority(authority)
}

func projectionAllowedScopes(projection Projection) []string {
	scopes := cleanSortedStrings(projection.Scopes)
	if len(scopes) > 0 {
		return scopes
	}
	scopes = scopesFromProjectionClaims(projection.Claims)
	if len(scopes) > 0 {
		return scopes
	}
	if projection.Resource != nil && projection.Operation != "" {
		return []string{scopeForAction(*projection.Resource, projection.Operation)}
	}
	return []string{"mission:" + projection.MissionRef + ":access"}
}

func scopesFromProjectionClaims(claims map[string]any) []string {
	scopes := append(stringListFromAny(claims["scope"]), stringListFromAny(claims["scopes"])...)
	return cleanSortedStrings(scopes)
}

func scopesFromAuthority(authority AuthorityRegion) []string {
	scopes := make([]string, 0)
	for _, grant := range authority.Resources {
		for _, action := range grant.Actions {
			scopes = append(scopes, scopeForAction(ActionResource{Type: grant.Type, ID: grant.ID}, action))
		}
	}
	return cleanSortedStrings(scopes)
}

func scopeForAction(resource ActionResource, operation string) string {
	return strings.TrimSpace(resource.Type) + ":" + strings.TrimSpace(resource.ID) + ":" + strings.TrimSpace(operation)
}

func stringListFromAny(value any) []string {
	switch typed := value.(type) {
	case string:
		return splitScopeString(typed)
	case []string:
		return cleanSortedStrings(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, splitScopeString(text)...)
			}
		}
		return cleanSortedStrings(values)
	default:
		return nil
	}
}

func splitScopeString(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	return cleanSortedStrings(fields)
}

func projectionAudience(audience string, projectionType string) string {
	audience = strings.TrimSpace(audience)
	if audience != "" {
		return audience
	}
	switch projectionType {
	case ProjectionTypeOAuthClaims:
		return "oauth"
	case ProjectionTypeMCPContext:
		return "mcp"
	case ProjectionTypeToolGatewayToken:
		return "tool-gateway"
	default:
		return "projection"
	}
}

func projectionConfirmation(actor Actor) map[string]string {
	confirmation := map[string]string{
		"agent_instance_id": actor.AgentInstanceID,
		"client_id":         actor.ClientID,
	}
	if actor.KeyThumbprint != "" {
		confirmation["key_thumbprint"] = actor.KeyThumbprint
	}
	return confirmation
}

func boundedBrokerExpiry(now time.Time, requestedSeconds int, projectionExpiresAt time.Time, missionExpiresAt time.Time) time.Time {
	seconds := requestedSeconds
	if seconds <= 0 {
		seconds = defaultCredentialTTLSeconds
	}
	if seconds > maxCredentialTTLSeconds {
		seconds = maxCredentialTTLSeconds
	}
	expiresAt := now.Add(time.Duration(seconds) * time.Second)
	if !projectionExpiresAt.IsZero() && projectionExpiresAt.Before(expiresAt) {
		expiresAt = projectionExpiresAt
	}
	if !missionExpiresAt.IsZero() && missionExpiresAt.Before(expiresAt) {
		expiresAt = missionExpiresAt
	}
	return expiresAt
}

func projectionExchangeNonceUsed(projection Projection, nonce string) bool {
	for _, record := range projection.ExchangeRecords {
		if record.Nonce == nonce {
			return true
		}
	}
	return false
}

func findCredentialExchangeRecord(projection Projection, exchangeID string, jti string) (CredentialExchangeRecord, bool) {
	for _, record := range projection.ExchangeRecords {
		if record.ExchangeID == exchangeID && record.JTI == jti {
			return record, true
		}
	}
	return CredentialExchangeRecord{}, false
}

func cleanActionResource(resource *ActionResource) *ActionResource {
	if resource == nil {
		return nil
	}
	cleaned := &ActionResource{Type: strings.TrimSpace(resource.Type), ID: strings.TrimSpace(resource.ID)}
	if cleaned.Type == "" && cleaned.ID == "" {
		return nil
	}
	return cleaned
}

func cloneActionResource(resource *ActionResource) *ActionResource {
	if resource == nil {
		return nil
	}
	cloned := *resource
	return &cloned
}

func actionResourcesEqual(a *ActionResource, b *ActionResource) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Type == b.Type && a.ID == b.ID
}

func stringSubset(values []string, allowed []string) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	for _, value := range values {
		if _, ok := allowedSet[value]; !ok {
			return false
		}
	}
	return true
}

func actorProvided(actor Actor) bool {
	return actor.AgentInstanceID != "" || actor.ClientID != "" || actor.KeyThumbprint != ""
}

func hashAccessToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sha256:" + base64.RawURLEncoding.EncodeToString(sum[:])
}

func SignCredentialAccessToken(payload CredentialAccessTokenPayload, key []byte) (string, error) {
	if len(key) == 0 {
		return "", errors.New("credential key is required")
	}
	headerJSON, err := json.Marshal(decisionArtifactHeader{
		Type:      credentialAccessTokenType,
		Algorithm: "HS256",
		KeyID:     "local",
	})
	if err != nil {
		return "", fmt.Errorf("marshal credential header: %w", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal credential payload: %w", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyCredentialAccessTokenPayload(token string, key []byte) (CredentialAccessTokenPayload, error) {
	if len(key) == 0 {
		return CredentialAccessTokenPayload{}, errors.New("credential key is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return CredentialAccessTokenPayload{}, errors.New("credential token must have three parts")
	}
	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return CredentialAccessTokenPayload{}, fmt.Errorf("decode credential signature: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signingInput))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return CredentialAccessTokenPayload{}, errors.New("credential signature mismatch")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return CredentialAccessTokenPayload{}, fmt.Errorf("decode credential header: %w", err)
	}
	var header decisionArtifactHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return CredentialAccessTokenPayload{}, fmt.Errorf("unmarshal credential header: %w", err)
	}
	if header.Algorithm != "HS256" || header.Type != credentialAccessTokenType {
		return CredentialAccessTokenPayload{}, errors.New("unsupported credential header")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return CredentialAccessTokenPayload{}, fmt.Errorf("decode credential payload: %w", err)
	}
	var payload CredentialAccessTokenPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return CredentialAccessTokenPayload{}, fmt.Errorf("unmarshal credential payload: %w", err)
	}
	return payload, nil
}

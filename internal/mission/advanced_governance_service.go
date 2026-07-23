package mission

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) CreateProjection(ref string, req CreateProjectionRequest) (ProjectionResponse, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return ProjectionResponse{}, err
	}
	now := s.clock.Now()
	if err := ensureMissionUsableForActor(mission, req.Actor, req.MissionVersionSeen, now); err != nil {
		return ProjectionResponse{}, err
	}
	if rule, ok, err := s.matchingActiveContainmentForEvaluation(mission, req.Actor, Action{Type: "projection", Name: req.Type, Resource: ActionResource{Type: "mission", ID: mission.MissionRef}, Operation: "project"}, now); err != nil {
		return ProjectionResponse{}, err
	} else if ok {
		return ProjectionResponse{}, fmt.Errorf("projection blocked by containment rule %s", rule.RuleID)
	}
	projectionType := strings.TrimSpace(req.Type)
	if !supportedProjectionType(projectionType) {
		return ProjectionResponse{}, fmt.Errorf("unsupported projection type %q", req.Type)
	}
	expiresAt := boundedExpiry(now, req.TTLSeconds, defaultProjectionTTLSeconds, mission.Lifecycle.ExpiresAt)
	claims := req.Claims
	if claims == nil {
		claims = projectionClaims(mission)
	}
	scopes := projectionRequestedScopes(req.Scopes, claims, mission.AuthorityRegion)
	audience := projectionAudience(req.Audience, projectionType)
	toolName := strings.TrimSpace(req.ToolName)
	resource := cleanActionResource(req.Resource)
	operation := strings.TrimSpace(req.Operation)
	if resource != nil || operation != "" {
		action := Action{Type: "tool_call", Name: toolName, Operation: operation}
		if resource != nil {
			action.Resource = *resource
		}
		if action.Resource.Type == "" || action.Resource.ID == "" || action.Operation == "" {
			return ProjectionResponse{}, fmt.Errorf("projection resource.type, resource.id, and operation are required when binding a projection to a tool action")
		}
		if !actionInScope(mission.AuthorityRegion, action) {
			return ProjectionResponse{}, fmt.Errorf("projection action is outside mission authority")
		}
	}
	projectionID := newID("mprj")
	payload := ProjectionPayload{
		JTI:            newID("pjti"),
		Issuer:         "auth-scope",
		Subject:        req.Actor.AgentInstanceID,
		Audience:       audience,
		TokenUse:       "projection_grant",
		ProjectionID:   projectionID,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		TenantID:       mission.TenantID,
		Type:           projectionType,
		Actor:          req.Actor,
		Agent:          mission.Agent,
		AuthorityHash:  HashDecisionContext(map[string]any{"authority_region": mission.AuthorityRegion}),
		Scopes:         scopes,
		ToolName:       toolName,
		Resource:       resource,
		Operation:      operation,
		Confirmation:   projectionConfirmation(req.Actor),
		Claims:         claims,
		IssuedAt:       now,
		NotBefore:      now,
		ExpiresAt:      expiresAt,
	}
	token, err := SignProjectionToken(payload, s.artifactKey)
	if err != nil {
		return ProjectionResponse{}, err
	}
	projection := Projection{
		ProjectionID:   projectionID,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		TenantID:       mission.TenantID,
		Type:           projectionType,
		Actor:          req.Actor,
		Scopes:         scopes,
		Audience:       audience,
		ToolName:       toolName,
		Resource:       resource,
		Operation:      operation,
		Claims:         claims,
		Token:          token,
		Status:         ProjectionStatusActive,
		IssuedAt:       now,
		ExpiresAt:      expiresAt,
	}
	if err := s.projections.SaveProjection(projection); err != nil {
		return ProjectionResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.projection_created",
		Actor:         map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID, "key_thumbprint": req.Actor.KeyThumbprint},
		Payload:       map[string]any{"projection_id": projection.ProjectionID, "projection_type": projection.Type, "scopes": projection.Scopes, "audience": projection.Audience, "tool_name": projection.ToolName, "operation": projection.Operation},
		VersionBefore: mission.Version,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	})
	return projectionResponse(projection), nil
}

func (s *Service) GetProjectionStatus(id string) (ProjectionStatusResponse, error) {
	projection, err := s.projections.GetProjection(id)
	if err != nil {
		return ProjectionStatusResponse{}, err
	}
	return projectionStatusResponse(projection, s.clock.Now()), nil
}

func (s *Service) VerifyProjection(req VerifyProjectionRequest) VerifyProjectionResponse {
	if strings.TrimSpace(req.Token) == "" {
		return VerifyProjectionResponse{Valid: false, Error: "token is required"}
	}
	payload, err := VerifyProjectionToken(req.Token, s.artifactKey)
	if err != nil {
		return VerifyProjectionResponse{Valid: false, Error: err.Error()}
	}
	projection, err := s.projections.GetProjection(payload.ProjectionID)
	if err != nil {
		return VerifyProjectionResponse{Valid: false, Payload: payload, Error: err.Error()}
	}
	if projection.Status == ProjectionStatusRevoked || !projection.RevokedAt.IsZero() {
		return VerifyProjectionResponse{Valid: false, Payload: payload, Projection: &projection, Error: "projection revoked"}
	}
	if rule, ok, err := s.matchingActiveContainmentForProjection(projection, s.clock.Now()); err != nil {
		return VerifyProjectionResponse{Valid: false, Payload: payload, Projection: &projection, Error: err.Error()}
	} else if ok {
		return VerifyProjectionResponse{Valid: false, Payload: payload, Projection: &projection, Error: "projection blocked by containment rule " + rule.RuleID}
	}
	if s.clock.Now().After(projection.ExpiresAt) {
		return VerifyProjectionResponse{Valid: false, Payload: payload, Projection: &projection, Error: "projection expired"}
	}
	return VerifyProjectionResponse{Valid: true, Payload: payload, Projection: &projection}
}

func (s *Service) RevokeProjection(id string, req StateChangeRequest) (ProjectionStatusResponse, error) {
	projection, err := s.projections.GetProjection(id)
	if err != nil {
		return ProjectionStatusResponse{}, err
	}
	if projection.Status != ProjectionStatusRevoked {
		now := s.clock.Now()
		projection.Status = ProjectionStatusRevoked
		projection.RevokedAt = now
		for index := range projection.ExchangeRecords {
			if projection.ExchangeRecords[index].Status != ProjectionStatusRevoked {
				projection.ExchangeRecords[index].Status = ProjectionStatusRevoked
				projection.ExchangeRecords[index].RevokedAt = now
			}
		}
		if err := s.projections.UpdateProjection(projection); err != nil {
			return ProjectionStatusResponse{}, err
		}
		_ = s.events.AppendEvent(Event{
			EventID:       newID("mev"),
			MissionRef:    projection.MissionRef,
			TenantID:      projection.TenantID,
			Type:          "mission.projection_revoked",
			Actor:         req.Actor,
			Payload:       map[string]any{"projection_id": projection.ProjectionID, "reason": req.Reason, "revoked_exchange_count": len(projection.ExchangeRecords)},
			VersionBefore: projection.MissionVersion,
			VersionAfter:  projection.MissionVersion,
			OccurredAt:    projection.RevokedAt,
			CausationID:   req.CausationID,
		})
	}
	return projectionStatusResponse(projection, s.clock.Now()), nil
}

func (s *Service) CreateMissionLease(ref string, req CreateLeaseRequest) (LeaseResponse, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return LeaseResponse{}, err
	}
	now := s.clock.Now()
	if err := ensureMissionUsableForActor(mission, req.Actor, req.MissionVersionSeen, now); err != nil {
		return leaseDenied(mission.MissionRef, mission.Version, err.Error()), nil
	}
	if rule, ok, err := s.matchingActiveContainmentForEvaluation(mission, req.Actor, Action{Type: "lease", Name: "mission_lease", Resource: ActionResource{Type: "mission", ID: mission.MissionRef}, Operation: "lease"}, now); err != nil {
		return LeaseResponse{}, err
	} else if ok {
		return leaseDenied(mission.MissionRef, mission.Version, "lease blocked by containment rule "+rule.RuleID), nil
	}
	lease := MissionLease{
		LeaseID:        newID("mlse"),
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		TenantID:       mission.TenantID,
		Actor:          req.Actor,
		CreatedAt:      now,
		ExpiresAt:      boundedExpiry(now, req.TTLSeconds, defaultLeaseTTLSeconds, mission.Lifecycle.ExpiresAt),
	}
	if err := s.projections.SaveMissionLease(lease); err != nil {
		return LeaseResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.lease_created",
		Actor:         map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID, "key_thumbprint": req.Actor.KeyThumbprint},
		Payload:       map[string]any{"lease_id": lease.LeaseID, "expires_at": lease.ExpiresAt},
		VersionBefore: mission.Version,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	})
	return leaseAllowed(lease), nil
}

func (s *Service) RefreshMissionLease(id string, req RefreshLeaseRequest) (LeaseResponse, error) {
	lease, err := s.projections.GetMissionLease(id)
	if err != nil {
		return LeaseResponse{}, err
	}
	mission, err := s.missions.GetMission(lease.MissionRef)
	if err != nil {
		return LeaseResponse{}, err
	}
	now := s.clock.Now()
	if !actorsEqual(lease.Actor, req.Actor) {
		return leaseDenied(lease.MissionRef, lease.MissionVersion, "lease actor mismatch"), nil
	}
	if now.After(lease.ExpiresAt) {
		return leaseDenied(lease.MissionRef, lease.MissionVersion, "lease expired"), nil
	}
	if err := ensureMissionUsableForActor(mission, req.Actor, lease.MissionVersion, now); err != nil {
		return leaseDenied(mission.MissionRef, mission.Version, err.Error()), nil
	}
	if rule, ok, err := s.matchingActiveContainmentForEvaluation(mission, req.Actor, Action{Type: "lease", Name: "mission_lease", Resource: ActionResource{Type: "mission", ID: mission.MissionRef}, Operation: "lease"}, now); err != nil {
		return LeaseResponse{}, err
	} else if ok {
		return leaseDenied(mission.MissionRef, mission.Version, "lease blocked by containment rule "+rule.RuleID), nil
	}
	if lease.MissionVersion != mission.Version {
		return leaseDenied(mission.MissionRef, mission.Version, "mission version changed"), nil
	}
	lease.RefreshedAt = now
	lease.ExpiresAt = boundedExpiry(now, req.TTLSeconds, defaultLeaseTTLSeconds, mission.Lifecycle.ExpiresAt)
	if err := s.projections.UpdateMissionLease(lease); err != nil {
		return LeaseResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.lease_refreshed",
		Actor:         map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID, "key_thumbprint": req.Actor.KeyThumbprint},
		Payload:       map[string]any{"lease_id": lease.LeaseID, "expires_at": lease.ExpiresAt},
		VersionBefore: mission.Version,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	})
	return leaseAllowed(lease), nil
}

func (s *Service) CreateApprovalRule(req ApprovalRule) (ApprovalRule, error) {
	rule := req
	if rule.RuleID == "" {
		rule.RuleID = newID("apr")
	}
	if rule.AppliesTo == "" {
		rule.AppliesTo = ApprovalAppliesExpansion
	}
	if rule.AppliesTo != ApprovalAppliesExpansion {
		return ApprovalRule{}, fmt.Errorf("unsupported approval applies_to %q", rule.AppliesTo)
	}
	if rule.RequiredApprovals <= 0 {
		rule.RequiredApprovals = 1
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = s.clock.Now()
	}
	if err := s.approvals.SaveApprovalRule(rule); err != nil {
		return ApprovalRule{}, err
	}
	return rule, nil
}

func (s *Service) ListApprovalRules() ([]ApprovalRule, error) {
	return s.approvals.ListApprovalRules()
}

func (s *Service) SubmitExpansionApproval(id string, req SubmitExpansionApprovalRequest) (SubmitExpansionApprovalResponse, error) {
	return s.SubmitExpansionApprovalContext(context.Background(), id, req)
}

func (s *Service) SubmitExpansionApprovalContext(ctx context.Context, id string, req SubmitExpansionApprovalRequest) (SubmitExpansionApprovalResponse, error) {
	if req.Approver.Subject == "" {
		return SubmitExpansionApprovalResponse{}, fmt.Errorf("approver.subject is required")
	}
	expansion, err := s.governance.GetExpansionRequest(id)
	if err != nil {
		return SubmitExpansionApprovalResponse{}, err
	}
	if expansion.Status != ExpansionStatusPending {
		return SubmitExpansionApprovalResponse{}, fmt.Errorf("expansion request is already %s", expansion.Status)
	}
	mission, err := s.missions.GetMission(expansion.MissionRef)
	if err != nil {
		return SubmitExpansionApprovalResponse{}, err
	}
	if err := s.ensureExpansionApprovalNotContained(ctx, mission, expansion, s.clock.Now()); err != nil {
		return SubmitExpansionApprovalResponse{}, err
	}
	rule, matched, err := s.matchingApprovalRule(expansion)
	if err != nil {
		return SubmitExpansionApprovalResponse{}, err
	}
	required := 1
	if matched {
		required = rule.RequiredApprovals
		if !approverAllowed(rule, req.Approver) {
			return SubmitExpansionApprovalResponse{}, fmt.Errorf("approver is not permitted by approval rule")
		}
	}
	now := s.clock.Now()
	evidence := req.ApprovalEvidence
	evidence.Approver = req.Approver
	if evidence.ApprovedAt.IsZero() {
		evidence.ApprovedAt = now
	}
	record := ApprovalRecord{
		ApprovalID:       newID("aprv"),
		RuleID:           rule.RuleID,
		TargetType:       ApprovalTargetExpansion,
		TargetID:         expansion.ExpansionID,
		TenantID:         expansion.TenantID,
		Approver:         req.Approver,
		ApprovalEvidence: evidence,
		Reason:           strings.TrimSpace(req.Reason),
		CreatedAt:        now,
	}
	if err := s.approvals.SaveApprovalRecord(record); err != nil {
		return SubmitExpansionApprovalResponse{}, err
	}
	records, err := s.approvals.ListApprovalRecords(ApprovalTargetExpansion, expansion.ExpansionID)
	if err != nil {
		return SubmitExpansionApprovalResponse{}, err
	}
	received := len(records)
	response := SubmitExpansionApprovalResponse{
		ExpansionID:       expansion.ExpansionID,
		Status:            expansion.Status,
		RuleID:            rule.RuleID,
		ApprovalsRequired: required,
		ApprovalsReceived: received,
		MissionRef:        expansion.MissionRef,
		MissionVersion:    expansion.MissionVersionSeen,
	}
	if received < required {
		_ = s.events.AppendEvent(Event{
			EventID:       newID("mev"),
			MissionRef:    expansion.MissionRef,
			TenantID:      expansion.TenantID,
			Type:          "mission.expansion_partial_approval",
			Actor:         map[string]any{"subject": req.Approver.Subject, "issuer": req.Approver.Issuer},
			Payload:       map[string]any{"expansion_id": expansion.ExpansionID, "rule_id": rule.RuleID, "approvals_received": received, "approvals_required": required},
			VersionBefore: expansion.MissionVersionSeen,
			VersionAfter:  expansion.MissionVersionSeen,
			OccurredAt:    now,
		})
		return response, nil
	}
	approved, err := s.decideExpansionRequest(ctx, id, ExpansionDecisionRequest{
		Approver:         req.Approver,
		ApprovalEvidence: evidence,
		Reason:           req.Reason,
	}, ExpansionStatusApproved, false)
	if err != nil {
		return SubmitExpansionApprovalResponse{}, err
	}
	response.Status = approved.Status
	response.MissionVersion = approved.MissionVersion
	return response, nil
}

func (s *Service) matchingApprovalRule(expansion ExpansionRequest) (ApprovalRule, bool, error) {
	rules, err := s.approvals.ListApprovalRules()
	if err != nil {
		return ApprovalRule{}, false, err
	}
	for _, rule := range rules {
		if approvalRuleMatchesExpansion(rule, expansion) {
			return rule, true, nil
		}
	}
	return ApprovalRule{}, false, nil
}

func ensureMissionUsableForActor(mission Mission, actor Actor, versionSeen int, now time.Time) error {
	if expired(mission, now) {
		return fmt.Errorf("mission expired")
	}
	if mission.State != StateActive {
		return fmt.Errorf("mission not active")
	}
	if !actorMatches(mission, actor) {
		return fmt.Errorf("actor not authorized for mission")
	}
	if versionSeen > 0 && versionSeen < mission.Version {
		return fmt.Errorf("mission version stale")
	}
	return nil
}

func supportedProjectionType(projectionType string) bool {
	return projectionType == ProjectionTypeOAuthClaims || projectionType == ProjectionTypeMCPContext || projectionType == ProjectionTypeToolGatewayToken
}

func boundedExpiry(now time.Time, requestedSeconds int, defaultSeconds int, missionExpiresAt time.Time) time.Time {
	seconds := requestedSeconds
	if seconds <= 0 {
		seconds = defaultSeconds
	}
	if defaultSeconds == defaultLeaseTTLSeconds && seconds > maxLeaseTTLSeconds {
		seconds = maxLeaseTTLSeconds
	}
	expiresAt := now.Add(time.Duration(seconds) * time.Second)
	if !missionExpiresAt.IsZero() && missionExpiresAt.Before(expiresAt) {
		return missionExpiresAt
	}
	return expiresAt
}

func projectionClaims(mission Mission) map[string]any {
	return map[string]any{
		"mission_ref":      mission.MissionRef,
		"mission_version":  mission.Version,
		"tenant_id":        mission.TenantID,
		"authority_region": mission.AuthorityRegion,
		"agent":            mission.Agent,
	}
}

func projectionResponse(projection Projection) ProjectionResponse {
	return ProjectionResponse{
		ProjectionID:   projection.ProjectionID,
		MissionRef:     projection.MissionRef,
		MissionVersion: projection.MissionVersion,
		Type:           projection.Type,
		Status:         projection.Status,
		Scopes:         append([]string(nil), projection.Scopes...),
		Audience:       projection.Audience,
		ToolName:       projection.ToolName,
		Resource:       cloneActionResource(projection.Resource),
		Operation:      projection.Operation,
		Token:          projection.Token,
		ExpiresAt:      projection.ExpiresAt,
	}
}

func projectionStatusResponse(projection Projection, now time.Time) ProjectionStatusResponse {
	status := projection.Status
	if status == ProjectionStatusActive && now.After(projection.ExpiresAt) {
		status = ProjectionStatusExpired
	}
	return ProjectionStatusResponse{
		ProjectionID:   projection.ProjectionID,
		MissionRef:     projection.MissionRef,
		MissionVersion: projection.MissionVersion,
		TenantID:       projection.TenantID,
		Type:           projection.Type,
		Status:         status,
		ExpiresAt:      projection.ExpiresAt,
		RevokedAt:      projection.RevokedAt,
		ExchangeCount:  len(projection.ExchangeRecords),
	}
}

func leaseAllowed(lease MissionLease) LeaseResponse {
	return LeaseResponse{
		LeaseID:        lease.LeaseID,
		MissionRef:     lease.MissionRef,
		MissionVersion: lease.MissionVersion,
		Decision:       DecisionAllow,
		ReasonCodes:    []string{"MISSION_LEASE_ACTIVE"},
		HumanReason:    "The mission lease is active.",
		ExpiresAt:      lease.ExpiresAt,
	}
}

func leaseDenied(ref string, version int, reason string) LeaseResponse {
	return LeaseResponse{
		MissionRef:     ref,
		MissionVersion: version,
		Decision:       DecisionDeny,
		ReasonCodes:    []string{"MISSION_LEASE_INVALID"},
		HumanReason:    reason,
	}
}

func actorsEqual(a Actor, b Actor) bool {
	return a.AgentInstanceID == b.AgentInstanceID && a.ClientID == b.ClientID && a.KeyThumbprint == b.KeyThumbprint
}

func approvalRuleMatchesExpansion(rule ApprovalRule, expansion ExpansionRequest) bool {
	if rule.AppliesTo != "" && rule.AppliesTo != ApprovalAppliesExpansion {
		return false
	}
	if rule.TenantID != "" && rule.TenantID != expansion.TenantID {
		return false
	}
	if rule.ResourceType != "" && rule.ResourceType != "*" && rule.ResourceType != expansion.Action.Resource.Type {
		return false
	}
	if rule.ResourceID != "" && rule.ResourceID != "*" && rule.ResourceID != expansion.Action.Resource.ID {
		return false
	}
	if rule.Operation != "" && rule.Operation != "*" && rule.Operation != expansion.Action.Operation {
		return false
	}
	if rule.RiskLevel != "" && fmt.Sprint(expansion.Context["risk"]) != rule.RiskLevel {
		return false
	}
	return true
}

func approverAllowed(rule ApprovalRule, approver Principal) bool {
	if len(rule.AllowedSubjects) > 0 && !contains(rule.AllowedSubjects, approver.Subject) {
		return false
	}
	if len(rule.AllowedIssuers) > 0 && !contains(rule.AllowedIssuers, approver.Issuer) {
		return false
	}
	return true
}

func approvalTargetKey(targetType string, targetID string) string {
	return targetType + "\x00" + targetID
}

func SignProjectionToken(payload ProjectionPayload, key []byte) (string, error) {
	if len(key) == 0 {
		return "", errors.New("projection key is required")
	}
	headerJSON, err := json.Marshal(decisionArtifactHeader{
		Type:      "mission-projection+jws",
		Algorithm: "HS256",
		KeyID:     "local",
	})
	if err != nil {
		return "", fmt.Errorf("marshal projection header: %w", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal projection payload: %w", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyProjectionToken(token string, key []byte) (ProjectionPayload, error) {
	if len(key) == 0 {
		return ProjectionPayload{}, errors.New("projection key is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ProjectionPayload{}, errors.New("projection token must have three parts")
	}
	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return ProjectionPayload{}, fmt.Errorf("decode projection signature: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signingInput))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return ProjectionPayload{}, errors.New("projection signature mismatch")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ProjectionPayload{}, fmt.Errorf("decode projection header: %w", err)
	}
	var header decisionArtifactHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return ProjectionPayload{}, fmt.Errorf("unmarshal projection header: %w", err)
	}
	if header.Algorithm != "HS256" || header.Type != "mission-projection+jws" {
		return ProjectionPayload{}, errors.New("unsupported projection header")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ProjectionPayload{}, fmt.Errorf("decode projection payload: %w", err)
	}
	var payload ProjectionPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return ProjectionPayload{}, fmt.Errorf("unmarshal projection payload: %w", err)
	}
	return payload, nil
}

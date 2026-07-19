package okta

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Clock interface {
	Now() time.Time
}

type Store interface {
	SaveAppBinding(AppBinding) error
	GetAppBinding(string) (AppBinding, error)
	UpdateAppBinding(AppBinding) error
	ListAppBindings() ([]AppBinding, error)
}

type Evaluator interface {
	Evaluate(string, EvaluationRequest, map[string]any) (EvaluationResponse, error)
}

type EventSink interface {
	AppendEvent(Event) error
}

type Config struct {
	Store      Store
	Evaluator  Evaluator
	Events     EventSink
	Clock      Clock
	NewID      func(string) string
	IsConflict func(error) bool
}

type Service struct {
	store      Store
	evaluator  Evaluator
	events     EventSink
	clock      Clock
	newID      func(string) string
	isConflict func(error) bool
}

func NewService(config Config) *Service {
	isConflict := config.IsConflict
	if isConflict == nil {
		isConflict = func(error) bool { return false }
	}
	return &Service{
		store:      config.Store,
		evaluator:  config.Evaluator,
		events:     config.Events,
		clock:      config.Clock,
		newID:      config.NewID,
		isConflict: isConflict,
	}
}

func (s *Service) CreateAppBinding(req CreateAppBindingRequest, actor Principal) (AppBinding, error) {
	issuer, err := NormalizeIssuer(req.Issuer)
	if err != nil {
		return AppBinding{}, err
	}
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		return AppBinding{}, fmt.Errorf("client_id is required")
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		return AppBinding{}, fmt.Errorf("mission_ref is required")
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	groupMatchMode := strings.ToLower(strings.TrimSpace(req.GroupMatchMode))
	if groupMatchMode == "" {
		groupMatchMode = GroupMatchAny
	}
	if groupMatchMode != GroupMatchAny && groupMatchMode != GroupMatchAll {
		return AppBinding{}, fmt.Errorf("group_match_mode must be any or all")
	}
	groupClaim := strings.TrimSpace(req.GroupClaim)
	if groupClaim == "" {
		groupClaim = "groups"
	}
	subjectClaim := strings.TrimSpace(req.SubjectClaim)
	if subjectClaim == "" {
		subjectClaim = "sub"
	}
	scopeClaim := strings.TrimSpace(req.ScopeClaim)
	if scopeClaim == "" {
		scopeClaim = "scp"
	}

	now := s.now()
	binding := AppBinding{
		BindingID:             s.id("okb"),
		TenantID:              strings.TrimSpace(req.TenantID),
		Issuer:                issuer,
		AuthorizationServerID: firstString(strings.TrimSpace(req.AuthorizationServerID), AuthorizationServerID(issuer)),
		DiscoveryURL:          DiscoveryURL(issuer),
		JWKSURI:               JWKSURI(issuer),
		ClientID:              clientID,
		AppID:                 strings.TrimSpace(req.AppID),
		AppLabel:              strings.TrimSpace(req.AppLabel),
		MissionRef:            missionRef,
		RequiredGroups:        CleanStringList(req.RequiredGroups),
		AdminGroups:           CleanStringList(req.AdminGroups),
		AllowedSubjects:       CleanStringList(req.AllowedSubjects),
		GroupClaim:            groupClaim,
		SubjectClaim:          subjectClaim,
		ScopeClaim:            scopeClaim,
		GroupMatchMode:        groupMatchMode,
		Status:                AppBindingStatusActive,
		Metadata:              CloneStringMap(req.Metadata),
		CreatedBy:             actor,
		CreatedAt:             now,
	}
	if err := s.store.SaveAppBinding(binding); err != nil {
		return AppBinding{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		TenantID:   binding.TenantID,
		MissionRef: binding.MissionRef,
		Type:       "okta.app_bound",
		Actor:      map[string]any{"subject": actor.Subject, "issuer": actor.Issuer},
		Payload: map[string]any{
			"binding_id":   binding.BindingID,
			"issuer":       binding.Issuer,
			"client_id":    binding.ClientID,
			"mission_ref":  binding.MissionRef,
			"group_claim":  binding.GroupClaim,
			"groups_bound": len(binding.RequiredGroups),
		},
		OccurredAt: now,
	})
	return binding, nil
}

func (s *Service) ListAppBindings() ([]AppBinding, error) {
	return s.store.ListAppBindings()
}

func (s *Service) ResolveAuthorityContext(req ResolveAuthorityContextRequest) (AuthorityContextResponse, error) {
	issuerHint, err := NormalizeIssuer(firstString(req.Issuer, StringClaim(req.Claims, "iss")))
	if err != nil {
		return AuthorityContextResponse{}, err
	}
	clientIDHint := firstString(req.ClientID, StringClaim(req.Claims, "cid"), StringClaim(req.Claims, "client_id"), AudienceClaim(req.Claims))
	if clientIDHint == "" {
		return AuthorityContextResponse{}, fmt.Errorf("client_id is required")
	}

	binding, ok, err := s.bindingFor(issuerHint, clientIDHint, strings.TrimSpace(req.MissionRef), strings.TrimSpace(req.TenantID))
	if err != nil {
		return AuthorityContextResponse{}, err
	}
	if !ok {
		return s.deniedResponse(AppBinding{}, issuerHint, clientIDHint, "", nil, nil, []string{"okta_no_matching_binding"}, "No active Okta binding matches the issuer and client."), nil
	}

	issuer, clientID, subject, groups, scopes, err := ExtractClaimContext(req, binding)
	if err != nil {
		return AuthorityContextResponse{}, err
	}
	reasons := make([]string, 0, 2)
	if len(binding.AllowedSubjects) > 0 && !ContainsString(binding.AllowedSubjects, subject) {
		reasons = append(reasons, "okta_subject_not_allowed")
	}
	if !GroupRequirementSatisfied(binding.RequiredGroups, groups, binding.GroupMatchMode) {
		reasons = append(reasons, "okta_required_group_missing")
	}

	now := s.now()
	binding.LastResolvedAt = now
	binding.LastSubject = subject
	if len(reasons) > 0 {
		binding.LastResolutionStatus = ResolutionStatusDenied
		_ = s.store.UpdateAppBinding(binding)
		resp := s.deniedResponse(binding, issuer, clientID, subject, groups, scopes, reasons, "Okta claims do not satisfy the mission binding.")
		s.appendResolutionEvent(binding, resp, now)
		return resp, nil
	}

	context := CloneContext(req.Context)
	admin := HasAny(groups, binding.AdminGroups)
	context["okta.issuer"] = issuer
	context["okta.authorization_server_id"] = binding.AuthorizationServerID
	context["okta.client_id"] = clientID
	context["okta.app_id"] = binding.AppID
	context["okta.app_label"] = binding.AppLabel
	context["okta.subject"] = subject
	context["okta.groups"] = groups
	context["okta.scopes"] = scopes
	context["okta.binding_id"] = binding.BindingID
	context["okta.admin"] = admin
	context["okta.required_groups"] = binding.RequiredGroups
	context["okta.group_match_mode"] = binding.GroupMatchMode

	resp := AuthorityContextResponse{
		Accepted:    true,
		Status:      ResolutionStatusAccepted,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		Issuer:      issuer,
		ClientID:    clientID,
		Subject:     subject,
		Groups:      groups,
		Scopes:      scopes,
		Admin:       admin,
		ReasonCodes: []string{"okta_binding_satisfied"},
		HumanReason: "Okta claims satisfy the active mission binding.",
		Context:     context,
		ResolvedAt:  now.Format(time.RFC3339),
	}
	if req.Evaluation != nil {
		if s.evaluator == nil {
			return AuthorityContextResponse{}, fmt.Errorf("okta evaluator is not configured")
		}
		decision, err := s.evaluator.Evaluate(binding.MissionRef, *req.Evaluation, context)
		if err != nil {
			return AuthorityContextResponse{}, err
		}
		resp.Evaluation = &decision
	}

	binding.LastResolutionStatus = ResolutionStatusAccepted
	_ = s.store.UpdateAppBinding(binding)
	s.appendResolutionEvent(binding, resp, now)
	return resp, nil
}

func (s *Service) bindingFor(issuer string, clientID string, missionRef string, tenantID string) (AppBinding, bool, error) {
	bindings, err := s.store.ListAppBindings()
	if err != nil {
		return AppBinding{}, false, err
	}
	for _, binding := range bindings {
		if binding.Status != AppBindingStatusActive {
			continue
		}
		if binding.Issuer != issuer || binding.ClientID != clientID {
			continue
		}
		if missionRef != "" && binding.MissionRef != missionRef {
			continue
		}
		if tenantID != "" && binding.TenantID != tenantID {
			continue
		}
		return binding, true, nil
	}
	return AppBinding{}, false, nil
}

func (s *Service) deniedResponse(binding AppBinding, issuer string, clientID string, subject string, groups []string, scopes []string, reasons []string, humanReason string) AuthorityContextResponse {
	return AuthorityContextResponse{
		Accepted:    false,
		Status:      ResolutionStatusDenied,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		Issuer:      issuer,
		ClientID:    clientID,
		Subject:     subject,
		Groups:      groups,
		Scopes:      scopes,
		ReasonCodes: reasons,
		HumanReason: humanReason,
		ResolvedAt:  s.now().Format(time.RFC3339),
	}
}

func (s *Service) appendResolutionEvent(binding AppBinding, resp AuthorityContextResponse, occurredAt time.Time) {
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "okta.authority_context_resolved",
		Actor:      map[string]any{"subject": resp.Subject, "issuer": resp.Issuer},
		Payload: map[string]any{
			"binding_id":   resp.BindingID,
			"client_id":    resp.ClientID,
			"subject":      resp.Subject,
			"accepted":     resp.Accepted,
			"status":       resp.Status,
			"reason_codes": resp.ReasonCodes,
			"admin":        resp.Admin,
		},
		OccurredAt: occurredAt,
	})
}

func (s *Service) appendEvent(event Event) {
	if s.events != nil {
		_ = s.events.AppendEvent(event)
	}
}

func (s *Service) now() time.Time {
	if s.clock == nil {
		return time.Now().UTC()
	}
	return s.clock.Now()
}

func (s *Service) id(prefix string) string {
	if s.newID == nil {
		return prefix
	}
	return s.newID(prefix)
}

func IsConflict(conflict error) func(error) bool {
	return func(err error) bool {
		return errors.Is(err, conflict)
	}
}

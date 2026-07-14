package entra

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

type Clock = contract.Clock

type Store interface {
	SaveAppRegistration(AppRegistration) error
	GetAppRegistration(string) (AppRegistration, error)
	UpdateAppRegistration(AppRegistration) error
	ListAppRegistrations() ([]AppRegistration, error)
}

type Evaluator = contract.Evaluator

type EventSink = contract.EventSink

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

func (s *Service) CreateAppRegistration(req CreateAppRegistrationRequest, actor Principal) (AppRegistration, error) {
	issuer, err := NormalizeIssuer(req.Issuer)
	if err != nil {
		return AppRegistration{}, err
	}
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		return AppRegistration{}, fmt.Errorf("client_id is required")
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		return AppRegistration{}, fmt.Errorf("mission_ref is required")
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	groupMatchMode := strings.ToLower(strings.TrimSpace(req.GroupMatchMode))
	if groupMatchMode == "" {
		groupMatchMode = GroupMatchAny
	}
	if groupMatchMode != GroupMatchAny && groupMatchMode != GroupMatchAll {
		return AppRegistration{}, fmt.Errorf("group_match_mode must be any or all")
	}
	groupClaim := strings.TrimSpace(req.GroupClaim)
	if groupClaim == "" {
		groupClaim = "groups"
	}
	subjectClaim := strings.TrimSpace(req.SubjectClaim)
	if subjectClaim == "" {
		subjectClaim = "sub"
	}
	rolesClaim := strings.TrimSpace(req.RolesClaim)
	if rolesClaim == "" {
		rolesClaim = "roles"
	}

	now := s.now()
	registration := AppRegistration{
		RegistrationID:  s.id("enr"),
		TenantID:        strings.TrimSpace(req.TenantID),
		TenantName:      strings.TrimSpace(req.TenantName),
		Issuer:          issuer,
		DiscoveryURL:    DiscoveryURL(issuer),
		JWKSURI:         JWKSURI(issuer),
		ClientID:        clientID,
		AppID:           strings.TrimSpace(req.AppID),
		AppName:         strings.TrimSpace(req.AppName),
		MissionRef:      missionRef,
		RequiredGroups:  CleanStringList(req.RequiredGroups),
		AdminGroups:     CleanStringList(req.AdminGroups),
		AllowedSubjects: CleanStringList(req.AllowedSubjects),
		GroupClaim:      groupClaim,
		SubjectClaim:    subjectClaim,
		RolesClaim:      rolesClaim,
		GroupMatchMode:  groupMatchMode,
		Status:          AppRegistrationStatusActive,
		Metadata:        CloneStringMap(req.Metadata),
		CreatedBy:       actor,
		CreatedAt:       now,
	}
	if err := s.store.SaveAppRegistration(registration); err != nil {
		return AppRegistration{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		TenantID:   registration.TenantID,
		MissionRef: registration.MissionRef,
		Type:       "entra.app_registered",
		Actor:      map[string]any{"subject": actor.Subject, "issuer": actor.Issuer},
		Payload: map[string]any{
			"registration_id": registration.RegistrationID,
			"issuer":          registration.Issuer,
			"client_id":       registration.ClientID,
			"mission_ref":     registration.MissionRef,
			"group_claim":     registration.GroupClaim,
			"groups_bound":    len(registration.RequiredGroups),
		},
		OccurredAt: now,
	})
	return registration, nil
}

func (s *Service) ListAppRegistrations() ([]AppRegistration, error) {
	return s.store.ListAppRegistrations()
}

func (s *Service) ResolveAuthorityContext(req ResolveAuthorityContextRequest) (AuthorityContextResponse, error) {
	issuerHint, err := NormalizeIssuer(firstString(req.Issuer, StringClaim(req.Claims, "iss")))
	if err != nil {
		return AuthorityContextResponse{}, err
	}
	clientIDHint := firstString(req.ClientID, StringClaim(req.Claims, "appid"), StringClaim(req.Claims, "azp"), StringClaim(req.Claims, "client_id"), AudienceClaim(req.Claims))
	if clientIDHint == "" {
		return AuthorityContextResponse{}, fmt.Errorf("client_id is required")
	}

	registration, ok, err := s.registrationFor(issuerHint, clientIDHint, strings.TrimSpace(req.MissionRef), strings.TrimSpace(req.TenantID))
	if err != nil {
		return AuthorityContextResponse{}, err
	}
	if !ok {
		return s.deniedResponse(AppRegistration{}, issuerHint, clientIDHint, "", nil, nil, []string{"entra_no_matching_registration"}, "No active Entra registration matches the issuer and client."), nil
	}

	issuer, clientID, subject, groups, roles, err := ExtractClaimContext(req, registration)
	if err != nil {
		return AuthorityContextResponse{}, err
	}
	reasons := make([]string, 0, 2)
	if len(registration.AllowedSubjects) > 0 && !ContainsString(registration.AllowedSubjects, subject) {
		reasons = append(reasons, "entra_subject_not_allowed")
	}
	if !GroupRequirementSatisfied(registration.RequiredGroups, groups, registration.GroupMatchMode) {
		reasons = append(reasons, "entra_required_group_missing")
	}

	now := s.now()
	registration.LastResolvedAt = now
	registration.LastSubject = subject
	if len(reasons) > 0 {
		registration.LastResolutionStatus = ResolutionStatusDenied
		_ = s.store.UpdateAppRegistration(registration)
		resp := s.deniedResponse(registration, issuer, clientID, subject, groups, roles, reasons, "Entra claims do not satisfy the registration requirements.")
		s.appendResolutionEvent(registration, resp, now)
		return resp, nil
	}

	context := CloneContext(req.Context)
	admin := HasAny(groups, registration.AdminGroups)
	context["entra.issuer"] = issuer
	context["entra.client_id"] = clientID
	context["entra.app_id"] = registration.AppID
	context["entra.app_name"] = registration.AppName
	context["entra.subject"] = subject
	context["entra.groups"] = groups
	context["entra.roles"] = roles
	context["entra.registration_id"] = registration.RegistrationID
	context["entra.admin"] = admin
	context["entra.required_groups"] = registration.RequiredGroups
	context["entra.group_match_mode"] = registration.GroupMatchMode

	resp := AuthorityContextResponse{
		Accepted:       true,
		Status:         ResolutionStatusAccepted,
		RegistrationID: registration.RegistrationID,
		TenantID:       registration.TenantID,
		MissionRef:     registration.MissionRef,
		Issuer:         issuer,
		ClientID:       clientID,
		Subject:        subject,
		Groups:         groups,
		Roles:          roles,
		Admin:          admin,
		ReasonCodes:    []string{"entra_registration_satisfied"},
		HumanReason:    "Entra claims satisfy the active registration requirements.",
		Context:        context,
		ResolvedAt:     now.Format(time.RFC3339),
	}
	if req.Evaluation != nil {
		if s.evaluator == nil {
			return AuthorityContextResponse{}, fmt.Errorf("entra evaluator is not configured")
		}
		evalReq := *req.Evaluation
		evalReq.MissionRef = registration.MissionRef
		evalReq.Context = context
		decision, err := s.evaluator.Evaluate(evalReq)
		if err != nil {
			return AuthorityContextResponse{}, err
		}
		resp.Evaluation = &decision
	}

	registration.LastResolutionStatus = ResolutionStatusAccepted
	_ = s.store.UpdateAppRegistration(registration)
	s.appendResolutionEvent(registration, resp, now)
	return resp, nil
}

func (s *Service) registrationFor(issuer string, clientID string, missionRef string, tenantID string) (AppRegistration, bool, error) {
	registrations, err := s.store.ListAppRegistrations()
	if err != nil {
		return AppRegistration{}, false, err
	}
	for _, reg := range registrations {
		if reg.Status != AppRegistrationStatusActive {
			continue
		}
		if reg.Issuer != issuer || reg.ClientID != clientID {
			continue
		}
		if missionRef != "" && reg.MissionRef != missionRef {
			continue
		}
		if tenantID != "" && reg.TenantID != tenantID {
			continue
		}
		return reg, true, nil
	}
	return AppRegistration{}, false, nil
}

func (s *Service) deniedResponse(registration AppRegistration, issuer string, clientID string, subject string, groups []string, roles []string, reasons []string, humanReason string) AuthorityContextResponse {
	return AuthorityContextResponse{
		Accepted:       false,
		Status:         ResolutionStatusDenied,
		RegistrationID: registration.RegistrationID,
		TenantID:       registration.TenantID,
		MissionRef:     registration.MissionRef,
		Issuer:         issuer,
		ClientID:       clientID,
		Subject:        subject,
		Groups:         groups,
		Roles:          roles,
		ReasonCodes:    reasons,
		HumanReason:    humanReason,
		ResolvedAt:     s.now().Format(time.RFC3339),
	}
}

func (s *Service) appendResolutionEvent(registration AppRegistration, resp AuthorityContextResponse, occurredAt time.Time) {
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: registration.MissionRef,
		TenantID:   registration.TenantID,
		Type:       "entra.authority_context_resolved",
		Actor:      map[string]any{"subject": resp.Subject, "issuer": resp.Issuer},
		Payload: map[string]any{
			"registration_id": resp.RegistrationID,
			"client_id":       resp.ClientID,
			"subject":         resp.Subject,
			"accepted":        resp.Accepted,
			"status":          resp.Status,
			"reason_codes":    resp.ReasonCodes,
			"admin":           resp.Admin,
		},
		OccurredAt: occurredAt,
	})
}

func (s *Service) appendEvent(event Event) {
	contract.AppendEvent(s.events, event)
}

func (s *Service) now() time.Time {
	return contract.Now(s.clock)
}

func (s *Service) id(prefix string) string {
	return contract.NewID(s.newID, prefix)
}

func IsConflict(conflict error) func(error) bool {
	return func(err error) bool {
		return errors.Is(err, conflict)
	}
}

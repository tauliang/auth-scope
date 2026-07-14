package servicenow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

var (
	ErrTicketBindingNotFound = errors.New("ticket binding not found")
	ErrInvalidState          = errors.New("invalid ticket state")
	ErrMissionRefRequired    = errors.New("mission reference is required")
	ErrInstanceURLRequired   = errors.New("instance URL is required")
)

type Store interface {
	SaveTicketBinding(TicketBinding) error
	GetTicketBinding(string) (TicketBinding, error)
	UpdateTicketBinding(TicketBinding) error
	ListTicketBindings() ([]TicketBinding, error)
	DeleteTicketBinding(string) error
}

type Evaluator = contract.Evaluator
type EventSink = contract.EventSink
type Clock = contract.Clock

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

func (s *Service) CreateTicketBinding(req CreateTicketBindingRequest, actor Principal) (TicketBinding, error) {
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		return TicketBinding{}, ErrMissionRefRequired
	}
	instanceURL := strings.TrimSpace(req.InstanceURL)
	if instanceURL == "" {
		return TicketBinding{}, ErrInstanceURLRequired
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	state := strings.TrimSpace(req.State)
	if state == "" {
		state = "new"
	}
	groupMatchMode := strings.ToLower(strings.TrimSpace(req.GroupMatchMode))
	if groupMatchMode == "" {
		groupMatchMode = contract.MatchAny
	}
	if groupMatchMode != contract.MatchAny && groupMatchMode != contract.MatchAll {
		return TicketBinding{}, fmt.Errorf("group_match_mode must be any or all")
	}
	groupClaim := strings.TrimSpace(req.GroupClaim)
	if groupClaim == "" {
		groupClaim = "groups"
	}
	subjectClaim := strings.TrimSpace(req.SubjectClaim)
	if subjectClaim == "" {
		subjectClaim = "sub"
	}

	now := s.now()
	binding := TicketBinding{
		BindingID:            s.id("snb"),
		TenantID:             strings.TrimSpace(req.TenantID),
		InstanceURL:          instanceURL,
		ServiceNowSysID:      strings.TrimSpace(req.ServiceNowSysID),
		ServiceNowNumber:     strings.TrimSpace(req.ServiceNowNumber),
		State:                state,
		MissionRef:           missionRef,
		AssignmentGroup:      strings.TrimSpace(req.AssignmentGroup),
		CallerID:             strings.TrimSpace(req.CallerID),
		RequiredGroups:       cleanStringList(req.RequiredGroups),
		AdminGroups:          cleanStringList(req.AdminGroups),
		AllowedSubjects:      cleanStringList(req.AllowedSubjects),
		GroupClaim:           groupClaim,
		SubjectClaim:         subjectClaim,
		GroupMatchMode:       groupMatchMode,
		Status:               TicketBindingStatusActive,
		Metadata:             cloneStringMap(req.Metadata),
		CreatedBy:            actor,
		CreatedAt:            now,
		LastResolutionStatus: "",
	}

	if err := s.store.SaveTicketBinding(binding); err != nil {
		return TicketBinding{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "servicenow.ticket_bound",
		Actor:      map[string]any{"subject": actor.Subject, "issuer": actor.Issuer},
		Payload: map[string]any{
			"binding_id":         binding.BindingID,
			"servicenow_sys_id":  binding.ServiceNowSysID,
			"service_now_number": binding.ServiceNowNumber,
			"state":              binding.State,
			"mission_ref":        binding.MissionRef,
		},
		OccurredAt: now,
	})
	return binding, nil
}

func (s *Service) GetTicketBinding(bindingID string) (TicketBinding, error) {
	binding, err := s.store.GetTicketBinding(strings.TrimSpace(bindingID))
	if err != nil {
		return TicketBinding{}, err
	}
	if binding.BindingID == "" {
		return TicketBinding{}, ErrTicketBindingNotFound
	}
	return binding, nil
}

func (s *Service) ListTicketBindings() ([]TicketBinding, error) {
	return s.store.ListTicketBindings()
}

func (s *Service) UpdateTicketStatus(bindingID string, newState string) (TicketBinding, error) {
	binding, err := s.GetTicketBinding(bindingID)
	if err != nil {
		return TicketBinding{}, err
	}

	if !validTicketState(newState) {
		return TicketBinding{}, ErrInvalidState
	}

	now := s.now()
	binding.State = strings.TrimSpace(newState)
	binding.LastResolvedAt = now

	if err := s.store.UpdateTicketBinding(binding); err != nil {
		return TicketBinding{}, err
	}

	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "servicenow.ticket_status_updated",
		Payload: map[string]any{
			"binding_id": binding.BindingID,
			"state":      binding.State,
		},
		OccurredAt: now,
	})

	return binding, nil
}

func (s *Service) ResolveAuthorityContext(req ResolveAuthorityContextRequest) (AuthorityContextResponse, error) {
	binding, ok, err := s.bindingFor(req)
	if err != nil {
		return AuthorityContextResponse{}, err
	}
	if !ok {
		return s.deniedResponse(TicketBinding{}, strings.TrimSpace(req.Subject), cleanStringList(req.Groups), []string{"servicenow_no_matching_binding"}, "No active ServiceNow ticket binding matches the request."), nil
	}

	subject := firstString(strings.TrimSpace(req.Subject), stringClaim(req.Claims, binding.SubjectClaim))
	groups := cleanStringList(firstStringSlice(req.Groups, stringSliceClaim(req.Claims, binding.GroupClaim)))
	reasons := make([]string, 0, 2)
	if len(binding.AllowedSubjects) > 0 && !containsString(binding.AllowedSubjects, subject) {
		reasons = append(reasons, "servicenow_subject_not_allowed")
	}
	if !stringRequirementSatisfied(binding.RequiredGroups, groups, binding.GroupMatchMode) {
		reasons = append(reasons, "servicenow_required_group_missing")
	}

	now := s.now()
	binding.LastResolvedAt = now
	binding.LastSubject = subject
	if len(reasons) > 0 {
		binding.LastResolutionStatus = ResolutionStatusDenied
		_ = s.store.UpdateTicketBinding(binding)
		resp := s.deniedResponse(binding, subject, groups, reasons, "ServiceNow subject does not satisfy the ticket binding requirements.")
		s.appendResolutionEvent(binding, resp, now)
		return resp, nil
	}

	context := cloneContext(req.Context)
	admin := containsAny(groups, binding.AdminGroups)
	context["servicenow.instance_url"] = binding.InstanceURL
	context["servicenow.sys_id"] = binding.ServiceNowSysID
	context["servicenow.number"] = binding.ServiceNowNumber
	context["servicenow.state"] = binding.State
	context["servicenow.assignment_group"] = binding.AssignmentGroup
	context["servicenow.caller_id"] = binding.CallerID
	context["servicenow.subject"] = subject
	context["servicenow.groups"] = groups
	context["servicenow.binding_id"] = binding.BindingID
	context["servicenow.admin"] = admin
	context["servicenow.required_groups"] = binding.RequiredGroups
	context["servicenow.group_match_mode"] = binding.GroupMatchMode

	resp := AuthorityContextResponse{
		Accepted:    true,
		Status:      ResolutionStatusAccepted,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		Subject:     subject,
		Groups:      groups,
		Admin:       admin,
		ReasonCodes: []string{"servicenow_binding_satisfied"},
		HumanReason: "ServiceNow subject satisfies the active ticket binding requirements.",
		Context:     context,
		ResolvedAt:  now.Format(time.RFC3339),
	}
	if req.Evaluation != nil {
		if s.evaluator == nil {
			return AuthorityContextResponse{}, fmt.Errorf("servicenow evaluator is not configured")
		}
		evalReq := *req.Evaluation
		evalReq.MissionRef = binding.MissionRef
		evalReq.Context = context
		decision, err := s.evaluator.Evaluate(evalReq)
		if err != nil {
			return AuthorityContextResponse{}, err
		}
		resp.Evaluation = &decision
	}

	binding.LastResolutionStatus = ResolutionStatusAccepted
	_ = s.store.UpdateTicketBinding(binding)
	s.appendResolutionEvent(binding, resp, now)
	return resp, nil
}

func (s *Service) DeleteTicketBinding(bindingID string) error {
	binding, err := s.GetTicketBinding(bindingID)
	if err != nil {
		return err
	}

	if err := s.store.DeleteTicketBinding(strings.TrimSpace(bindingID)); err != nil {
		return err
	}

	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "servicenow.ticket_binding_deleted",
		Payload: map[string]any{
			"binding_id": strings.TrimSpace(bindingID),
		},
		OccurredAt: s.now(),
	})

	return nil
}

func (s *Service) bindingFor(req ResolveAuthorityContextRequest) (TicketBinding, bool, error) {
	bindings, err := s.store.ListTicketBindings()
	if err != nil {
		return TicketBinding{}, false, err
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	tenantID := strings.TrimSpace(req.TenantID)
	sysID := requestedSysID(req)
	for _, binding := range bindings {
		if binding.Status != TicketBindingStatusActive {
			continue
		}
		if missionRef != "" && binding.MissionRef != missionRef {
			continue
		}
		if tenantID != "" && binding.TenantID != tenantID {
			continue
		}
		if sysID != "" && binding.ServiceNowSysID != "" && binding.ServiceNowSysID != sysID {
			continue
		}
		return binding, true, nil
	}
	return TicketBinding{}, false, nil
}

func (s *Service) deniedResponse(binding TicketBinding, subject string, groups []string, reasons []string, humanReason string) AuthorityContextResponse {
	return AuthorityContextResponse{
		Accepted:    false,
		Status:      ResolutionStatusDenied,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		Subject:     subject,
		Groups:      groups,
		ReasonCodes: reasons,
		HumanReason: humanReason,
		ResolvedAt:  s.now().Format(time.RFC3339),
	}
}

func (s *Service) appendResolutionEvent(binding TicketBinding, resp AuthorityContextResponse, occurredAt time.Time) {
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "servicenow.authority_context_resolved",
		Actor:      map[string]any{"subject": resp.Subject},
		Payload: map[string]any{
			"binding_id":   resp.BindingID,
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
	contract.AppendEvent(s.events, event)
}

func (s *Service) now() time.Time {
	return contract.Now(s.clock)
}

func (s *Service) id(prefix string) string {
	return contract.NewID(s.newID, prefix)
}

func validTicketState(state string) bool {
	switch strings.TrimSpace(state) {
	case "new", "in_progress", "on_hold", "resolved", "closed":
		return true
	default:
		return false
	}
}

func requestedSysID(req ResolveAuthorityContextRequest) string {
	if req.Evaluation != nil && req.Evaluation.Action.Resource.Type == "servicenow_ticket" {
		return strings.TrimSpace(req.Evaluation.Action.Resource.ID)
	}
	for _, key := range []string{"servicenow.sys_id", "servicenow_sys_id", "sys_id"} {
		if value := stringContext(req.Context, key); value != "" {
			return value
		}
	}
	return ""
}

func stringContext(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	return stringValue(values[key])
}

func firstString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringClaim(claims map[string]any, key string) string {
	if key == "" || claims == nil {
		return ""
	}
	return stringValue(claims[key])
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func firstStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func stringSliceClaim(claims map[string]any, key string) []string {
	if key == "" || claims == nil {
		return nil
	}
	switch typed := claims[key].(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringValue(item); text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		return strings.Fields(typed)
	default:
		return nil
	}
}

func cleanStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned
}

func containsString(values []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return false
	}
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == want {
			return true
		}
	}
	return false
}

func containsAny(values []string, wants []string) bool {
	for _, want := range wants {
		if containsString(values, want) {
			return true
		}
	}
	return false
}

func stringRequirementSatisfied(required []string, got []string, mode string) bool {
	if len(required) == 0 {
		return true
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == contract.MatchAll {
		for _, requiredValue := range required {
			if !containsString(got, requiredValue) {
				return false
			}
		}
		return true
	}
	return containsAny(got, required)
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func cloneContext(values map[string]any) map[string]any {
	clone := map[string]any{}
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func IsConflict(conflict error) func(error) bool {
	return func(err error) bool {
		return errors.Is(err, conflict)
	}
}

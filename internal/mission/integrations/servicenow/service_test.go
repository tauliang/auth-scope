package servicenow

import (
	"errors"
	"testing"
	"time"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type memoryStore struct {
	bindings  map[string]TicketBinding
	createErr error
	listErr   error
	updateErr error
	deleteErr error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{bindings: map[string]TicketBinding{}}
}

func (s *memoryStore) SaveTicketBinding(binding TicketBinding) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *memoryStore) GetTicketBinding(bindingID string) (TicketBinding, error) {
	binding, ok := s.bindings[bindingID]
	if !ok {
		return TicketBinding{}, nil
	}
	return binding, nil
}

func (s *memoryStore) ListTicketBindings() ([]TicketBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := []TicketBinding{}
	for _, binding := range s.bindings {
		result = append(result, binding)
	}
	return result, nil
}

func (s *memoryStore) UpdateTicketBinding(binding TicketBinding) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *memoryStore) DeleteTicketBinding(bindingID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.bindings, bindingID)
	return nil
}

type evaluator struct {
	gotReq   EvaluationRequest
	response EvaluationResponse
	err      error
}

func (e *evaluator) Evaluate(req EvaluationRequest) (EvaluationResponse, error) {
	e.gotReq = req
	if e.err != nil {
		return EvaluationResponse{}, e.err
	}
	return e.response, nil
}

type eventSink struct {
	events []Event
	err    error
}

func (s *eventSink) AppendEvent(event Event) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, event)
	return nil
}

func newTestService(store *memoryStore, evaluator Evaluator, events *eventSink) *Service {
	return NewService(Config{
		Store:     store,
		Evaluator: evaluator,
		Events:    events,
		Clock:     fixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
		NewID: func(prefix string) string {
			return prefix + "_test"
		},
	})
}

func TestServiceNowTicketBindingLifecycle(t *testing.T) {
	store := newMemoryStore()
	evaluator := &evaluator{response: EvaluationResponse{
		Decision:       "allow",
		MissionRef:     "mref_123",
		MissionVersion: 2,
		ReasonCodes:    []string{"in_scope"},
	}}
	events := &eventSink{}
	service := newTestService(store, evaluator, events)

	binding, err := service.CreateTicketBinding(CreateTicketBindingRequest{
		TenantID:        "demo",
		InstanceURL:     "https://acme.service-now.com",
		ServiceNowSysID: "sys_123",
		State:           "new",
		MissionRef:      "mref_123",
		AssignmentGroup: "Change Approvers",
		CallerID:        "agent@example.com",
		RequiredGroups:  []string{"Change Approvers"},
		AdminGroups:     []string{"CAB Admins"},
		AllowedSubjects: []string{"agent@example.com"},
		GroupClaim:      "groups",
		SubjectClaim:    "sub",
		GroupMatchMode:  "any",
		Metadata:        map[string]string{"change": "standard"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateTicketBinding: %v", err)
	}
	if binding.BindingID == "" || binding.Status != TicketBindingStatusActive {
		t.Fatalf("unexpected binding identity: %#v", binding)
	}
	if binding.CreatedAt.IsZero() || binding.CreatedAt != time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC) {
		t.Fatalf("created_at was not set from clock: %#v", binding.CreatedAt)
	}
	if len(events.events) != 1 || events.events[0].Type != "servicenow.ticket_bound" {
		t.Fatalf("create event not published: %#v", events.events)
	}

	got, err := service.GetTicketBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetTicketBinding: %v", err)
	}
	if got.ServiceNowSysID != "sys_123" || got.MissionRef != "mref_123" {
		t.Fatalf("unexpected stored binding: %#v", got)
	}

	byMission, err := service.ListTicketBindings()
	if err != nil {
		t.Fatalf("ListTicketBindingsByMissionRef: %v", err)
	}
	if len(byMission) != 1 || byMission[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected mission binding list: %#v", byMission)
	}

	updated, err := service.UpdateTicketStatus(binding.BindingID, "in_progress")
	if err != nil {
		t.Fatalf("UpdateTicketStatus: %v", err)
	}
	if updated.State != "in_progress" || updated.LastResolvedAt.IsZero() {
		t.Fatalf("ticket status was not updated: %#v", updated)
	}
	if len(events.events) != 2 || events.events[1].Type != "servicenow.ticket_status_updated" {
		t.Fatalf("status update event not published: %#v", events.events)
	}

	resolved, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		TenantID:   "demo",
		MissionRef: "mref_123",
		Subject:    "agent@example.com",
		Groups:     []string{"Change Approvers"},
		Context:    map[string]any{"risk": "low"},
		Evaluation: &EvaluationRequest{
			MissionVersionSeen: 1,
			Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "agent"},
			Action: EvaluationAction{
				Type:      "ticket_change",
				Resource:  EvaluationActionResource{Type: "servicenow_ticket", ID: "sys_123"},
				Operation: "update",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveAuthorityContext: %v", err)
	}
	if !resolved.Accepted || evaluator.gotReq.MissionRef != "mref_123" {
		t.Fatalf("unexpected authority response: resp=%#v req=%#v", resolved, evaluator.gotReq)
	}
	if evaluator.gotReq.Context["servicenow.sys_id"] != "sys_123" {
		t.Fatalf("expected ServiceNow context to be forwarded to evaluator: %#v", evaluator.gotReq.Context)
	}
	if len(events.events) != 3 || events.events[2].Type != "servicenow.authority_context_resolved" {
		t.Fatalf("resolve event not published: %#v", events.events)
	}

	if err := service.DeleteTicketBinding(binding.BindingID); err != nil {
		t.Fatalf("DeleteTicketBinding: %v", err)
	}
	if _, ok := store.bindings[binding.BindingID]; ok {
		t.Fatalf("binding still present after delete: %#v", store.bindings[binding.BindingID])
	}
	if len(events.events) != 4 || events.events[3].Type != "servicenow.ticket_binding_deleted" {
		t.Fatalf("delete event not published: %#v", events.events)
	}
}

func TestServiceNowValidationAndErrorPaths(t *testing.T) {
	service := newTestService(newMemoryStore(), &evaluator{}, &eventSink{})

	if _, err := service.CreateTicketBinding(CreateTicketBindingRequest{InstanceURL: "https://acme.service-now.com"}, Principal{}); !errors.Is(err, ErrMissionRefRequired) {
		t.Fatalf("missing mission err = %v, want ErrMissionRefRequired", err)
	}
	if _, err := service.CreateTicketBinding(CreateTicketBindingRequest{MissionRef: "mref_123"}, Principal{}); !errors.Is(err, ErrInstanceURLRequired) {
		t.Fatalf("missing instance URL err = %v, want ErrInstanceURLRequired", err)
	}
	if _, err := service.GetTicketBinding("missing"); !errors.Is(err, ErrTicketBindingNotFound) {
		t.Fatalf("missing binding err = %v, want ErrTicketBindingNotFound", err)
	}

	storeErr := errors.New("store failed")
	store := newMemoryStore()
	store.createErr = storeErr
	service = newTestService(store, &evaluator{}, &eventSink{})
	if _, err := service.CreateTicketBinding(CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com"}, Principal{}); !errors.Is(err, storeErr) {
		t.Fatalf("create store err = %v, want wrapped store error", err)
	}

	eventErr := errors.New("event failed")
	service = newTestService(newMemoryStore(), &evaluator{}, &eventSink{err: eventErr})
	if _, err := service.CreateTicketBinding(CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com"}, Principal{}); err != nil {
		t.Fatalf("event sink errors should not fail binding creation: %v", err)
	}

	store = newMemoryStore()
	events := &eventSink{}
	service = newTestService(store, &evaluator{}, events)
	binding, err := service.CreateTicketBinding(CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com", State: "new"}, Principal{})
	if err != nil {
		t.Fatalf("CreateTicketBinding setup: %v", err)
	}
	if _, err := service.UpdateTicketStatus(binding.BindingID, "not_a_state"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("invalid state err = %v, want ErrInvalidState", err)
	}

	evalErr := errors.New("evaluation failed")
	store = newMemoryStore()
	service = newTestService(store, &evaluator{err: evalErr}, &eventSink{})
	_, err = service.CreateTicketBinding(CreateTicketBindingRequest{
		MissionRef:     "mref_123",
		InstanceURL:    "https://acme.service-now.com",
		RequiredGroups: []string{"Change Approvers"},
	}, Principal{})
	if err != nil {
		t.Fatalf("CreateTicketBinding eval setup: %v", err)
	}
	if _, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		MissionRef: "mref_123",
		Subject:    "agent@example.com",
		Groups:     []string{"Change Approvers"},
		Evaluation: &EvaluationRequest{Action: EvaluationAction{Resource: EvaluationActionResource{Type: "servicenow_ticket"}}},
	}); !errors.Is(err, evalErr) {
		t.Fatalf("resolve evaluator err = %v, want wrapped evaluator error", err)
	}

	store = newMemoryStore()
	store.deleteErr = storeErr
	service = newTestService(store, &evaluator{}, &eventSink{})
	binding, err = service.CreateTicketBinding(CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com", State: "new"}, Principal{})
	if err != nil {
		t.Fatalf("CreateTicketBinding delete setup: %v", err)
	}
	if err := service.DeleteTicketBinding(binding.BindingID); !errors.Is(err, storeErr) {
		t.Fatalf("delete store err = %v, want wrapped store error", err)
	}
}

func TestServiceNowResolveAuthorityContextDeniesAndParsesClaims(t *testing.T) {
	store := newMemoryStore()
	events := &eventSink{}
	service := newTestService(store, nil, events)

	denied, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		TenantID:   "demo",
		MissionRef: "mref_missing",
		Subject:    " agent@example.com ",
		Groups:     []string{" Mission Operators ", "", "mission operators"},
		Context:    map[string]any{"sys_id": " sys_404 "},
	})
	if err != nil {
		t.Fatalf("ResolveAuthorityContext no binding: %v", err)
	}
	if denied.Accepted || denied.Status != ResolutionStatusDenied || !containsString(denied.ReasonCodes, "servicenow_no_matching_binding") {
		t.Fatalf("no-match response = %#v, want denied no matching binding", denied)
	}
	if denied.Subject != "agent@example.com" || len(denied.Groups) != 1 || denied.Groups[0] != "Mission Operators" {
		t.Fatalf("no-match identity context = %#v, want trimmed subject and deduplicated groups", denied)
	}

	binding := TicketBinding{
		BindingID:        "snb_1",
		TenantID:         "demo",
		InstanceURL:      "https://acme.service-now.com",
		ServiceNowSysID:  "sys_123",
		ServiceNowNumber: "CHG0010001",
		State:            "new",
		MissionRef:       "mref_123",
		RequiredGroups:   []string{"CAB", "Approvers"},
		AdminGroups:      []string{"Admins"},
		AllowedSubjects:  []string{"allowed@example.com"},
		GroupClaim:       "roles",
		SubjectClaim:     "user",
		GroupMatchMode:   "all",
		Status:           TicketBindingStatusActive,
	}
	store.bindings[binding.BindingID] = binding

	denied, err = service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		TenantID:   "demo",
		MissionRef: "mref_123",
		Claims: map[string]any{
			"user":  " other@example.com ",
			"roles": "CAB Admins",
		},
		Context: map[string]any{"servicenow_sys_id": " sys_123 "},
	})
	if err != nil {
		t.Fatalf("ResolveAuthorityContext denied binding: %v", err)
	}
	if denied.Accepted || denied.BindingID != binding.BindingID {
		t.Fatalf("binding denial response = %#v, want denied bound response", denied)
	}
	if !containsString(denied.ReasonCodes, "servicenow_subject_not_allowed") || !containsString(denied.ReasonCodes, "servicenow_required_group_missing") {
		t.Fatalf("binding denial reasons = %#v, want subject and group failures", denied.ReasonCodes)
	}
	if store.bindings[binding.BindingID].LastResolutionStatus != ResolutionStatusDenied {
		t.Fatalf("binding resolution status = %q, want denied", store.bindings[binding.BindingID].LastResolutionStatus)
	}
	if len(events.events) != 1 || events.events[0].Type != "servicenow.authority_context_resolved" {
		t.Fatalf("denial event not emitted: %#v", events.events)
	}

	binding.AllowedSubjects = nil
	store.bindings[binding.BindingID] = binding
	accepted, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		TenantID:   "demo",
		MissionRef: "mref_123",
		Claims: map[string]any{
			"user":  " allowed@example.com ",
			"roles": []string{"CAB", "Approvers", "Admins"},
		},
		Context: map[string]any{"servicenow.sys_id": " sys_123 "},
	})
	if err != nil {
		t.Fatalf("ResolveAuthorityContext accepted binding: %v", err)
	}
	if !accepted.Accepted || !accepted.Admin || accepted.Subject != "allowed@example.com" {
		t.Fatalf("accepted response = %#v, want admin authority context from claims", accepted)
	}
	if accepted.Context["servicenow.sys_id"] != "sys_123" || accepted.Context["servicenow.number"] != "CHG0010001" {
		t.Fatalf("accepted context = %#v, want ServiceNow metadata", accepted.Context)
	}
}

func TestServiceNowIsConflictClassifier(t *testing.T) {
	conflict := errors.New("conflict")
	classifier := IsConflict(conflict)
	if !classifier(conflict) {
		t.Fatalf("classifier returned false for configured conflict error")
	}
	if classifier(errors.New("other")) {
		t.Fatalf("classifier returned true for unrelated error")
	}
}

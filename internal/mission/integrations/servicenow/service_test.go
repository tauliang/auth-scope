package servicenow

import (
	"context"
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
	bindings  map[string]*TicketBinding
	createErr error
	listErr   error
	updateErr error
	deleteErr error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{bindings: map[string]*TicketBinding{}}
}

func (s *memoryStore) CreateTicketBinding(_ context.Context, binding *TicketBinding) error {
	if s.createErr != nil {
		return s.createErr
	}
	clone := *binding
	s.bindings[binding.BindingID] = &clone
	return nil
}

func (s *memoryStore) GetTicketBindingByID(_ context.Context, bindingID string) (*TicketBinding, error) {
	binding := s.bindings[bindingID]
	if binding == nil {
		return nil, nil
	}
	clone := *binding
	return &clone, nil
}

func (s *memoryStore) GetTicketBindingByMissionRefAndSysID(_ context.Context, missionRef, sysID string) (*TicketBinding, error) {
	for _, binding := range s.bindings {
		if binding.MissionRef == missionRef && binding.ServiceNowSysID == sysID {
			clone := *binding
			return &clone, nil
		}
	}
	return nil, nil
}

func (s *memoryStore) ListTicketBindingsByMissionRef(_ context.Context, missionRef string) ([]*TicketBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := []*TicketBinding{}
	for _, binding := range s.bindings {
		if missionRef == "" || binding.MissionRef == missionRef {
			clone := *binding
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (s *memoryStore) UpdateTicketBinding(_ context.Context, binding *TicketBinding) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	clone := *binding
	s.bindings[binding.BindingID] = &clone
	return nil
}

func (s *memoryStore) DeleteTicketBinding(_ context.Context, bindingID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.bindings, bindingID)
	return nil
}

type evaluator struct {
	gotReq   *ResolveAuthorityContextRequest
	response *AuthorityContextResponse
	err      error
}

func (e *evaluator) EvaluateAuthorityContext(_ context.Context, req *ResolveAuthorityContextRequest) (*AuthorityContextResponse, error) {
	e.gotReq = req
	if e.err != nil {
		return nil, e.err
	}
	return e.response, nil
}

type eventSink struct {
	events []*Event
	err    error
}

func (s *eventSink) PublishEvent(_ context.Context, event *Event) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, event)
	return nil
}

func newTestService(store *memoryStore, evaluator Evaluator, events *eventSink) *Service {
	return NewService(&Config{
		Store:     store,
		Evaluator: evaluator,
		EventSink: events,
		Clock:     fixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
	})
}

func TestServiceNowTicketBindingLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	evaluator := &evaluator{response: &AuthorityContextResponse{
		Accepted:    true,
		Status:      ResolutionStatusAccepted,
		BindingID:   "sn-binding",
		MissionRef:  "mref_123",
		Subject:     "agent@example.com",
		Groups:      []string{"Change Approvers"},
		ReasonCodes: []string{"servicenow_binding_satisfied"},
	}}
	events := &eventSink{}
	service := newTestService(store, evaluator, events)

	binding, err := service.CreateTicketBinding(ctx, &CreateTicketBindingRequest{
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
	})
	if err != nil {
		t.Fatalf("CreateTicketBinding: %v", err)
	}
	if binding.BindingID == "" || binding.Status != TicketBindingStatusActive {
		t.Fatalf("unexpected binding identity: %#v", binding)
	}
	if binding.CreatedAt.IsZero() || binding.CreatedAt != time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC) {
		t.Fatalf("created_at was not set from clock: %#v", binding.CreatedAt)
	}
	if len(events.events) != 1 || events.events[0].Type != "ticket_binding.created" {
		t.Fatalf("create event not published: %#v", events.events)
	}

	got, err := service.GetTicketBinding(ctx, binding.BindingID)
	if err != nil {
		t.Fatalf("GetTicketBinding: %v", err)
	}
	if got.ServiceNowSysID != "sys_123" || got.MissionRef != "mref_123" {
		t.Fatalf("unexpected stored binding: %#v", got)
	}

	byMission, err := service.ListTicketBindingsByMissionRef(ctx, "mref_123")
	if err != nil {
		t.Fatalf("ListTicketBindingsByMissionRef: %v", err)
	}
	if len(byMission) != 1 || byMission[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected mission binding list: %#v", byMission)
	}

	updated, err := service.UpdateTicketStatus(ctx, binding.BindingID, "in_progress")
	if err != nil {
		t.Fatalf("UpdateTicketStatus: %v", err)
	}
	if updated.State != "in_progress" || updated.LastResolvedAt.IsZero() {
		t.Fatalf("ticket status was not updated: %#v", updated)
	}
	if len(events.events) != 2 || events.events[1].Type != "ticket_binding.status_updated" {
		t.Fatalf("status update event not published: %#v", events.events)
	}

	resolved, err := service.ResolveAuthorityContext(ctx, &ResolveAuthorityContextRequest{
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
	if !resolved.Accepted || evaluator.gotReq.Subject != "agent@example.com" {
		t.Fatalf("unexpected authority response: resp=%#v req=%#v", resolved, evaluator.gotReq)
	}
	if len(events.events) != 3 || events.events[2].Type != "authority_context.resolved" {
		t.Fatalf("resolve event not published: %#v", events.events)
	}

	if err := service.DeleteTicketBinding(ctx, binding.BindingID); err != nil {
		t.Fatalf("DeleteTicketBinding: %v", err)
	}
	if _, ok := store.bindings[binding.BindingID]; ok {
		t.Fatalf("binding still present after delete: %#v", store.bindings[binding.BindingID])
	}
	if len(events.events) != 4 || events.events[3].Type != "ticket_binding.deleted" {
		t.Fatalf("delete event not published: %#v", events.events)
	}
}

func TestServiceNowValidationAndErrorPaths(t *testing.T) {
	ctx := context.Background()
	service := newTestService(newMemoryStore(), &evaluator{}, &eventSink{})

	if _, err := service.CreateTicketBinding(ctx, &CreateTicketBindingRequest{InstanceURL: "https://acme.service-now.com"}); !errors.Is(err, ErrMissionRefRequired) {
		t.Fatalf("missing mission err = %v, want ErrMissionRefRequired", err)
	}
	if _, err := service.CreateTicketBinding(ctx, &CreateTicketBindingRequest{MissionRef: "mref_123"}); !errors.Is(err, ErrInstanceURLRequired) {
		t.Fatalf("missing instance URL err = %v, want ErrInstanceURLRequired", err)
	}
	if _, err := service.GetTicketBinding(ctx, "missing"); !errors.Is(err, ErrTicketBindingNotFound) {
		t.Fatalf("missing binding err = %v, want ErrTicketBindingNotFound", err)
	}

	storeErr := errors.New("store failed")
	store := newMemoryStore()
	store.createErr = storeErr
	service = newTestService(store, &evaluator{}, &eventSink{})
	if _, err := service.CreateTicketBinding(ctx, &CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com"}); !errors.Is(err, storeErr) {
		t.Fatalf("create store err = %v, want wrapped store error", err)
	}

	eventErr := errors.New("event failed")
	service = newTestService(newMemoryStore(), &evaluator{}, &eventSink{err: eventErr})
	if _, err := service.CreateTicketBinding(ctx, &CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com"}); !errors.Is(err, eventErr) {
		t.Fatalf("create event err = %v, want wrapped event error", err)
	}

	store = newMemoryStore()
	events := &eventSink{}
	service = newTestService(store, &evaluator{}, events)
	binding, err := service.CreateTicketBinding(ctx, &CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com", State: "new"})
	if err != nil {
		t.Fatalf("CreateTicketBinding setup: %v", err)
	}
	if _, err := service.UpdateTicketStatus(ctx, binding.BindingID, "not_a_state"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("invalid state err = %v, want ErrInvalidState", err)
	}

	evalErr := errors.New("evaluation failed")
	service = newTestService(newMemoryStore(), &evaluator{err: evalErr}, &eventSink{})
	if _, err := service.ResolveAuthorityContext(ctx, &ResolveAuthorityContextRequest{MissionRef: "mref_123"}); !errors.Is(err, evalErr) {
		t.Fatalf("resolve evaluator err = %v, want wrapped evaluator error", err)
	}

	store = newMemoryStore()
	store.deleteErr = storeErr
	service = newTestService(store, &evaluator{}, &eventSink{})
	binding, err = service.CreateTicketBinding(ctx, &CreateTicketBindingRequest{MissionRef: "mref_123", InstanceURL: "https://acme.service-now.com", State: "new"})
	if err != nil {
		t.Fatalf("CreateTicketBinding delete setup: %v", err)
	}
	if err := service.DeleteTicketBinding(ctx, binding.BindingID); !errors.Is(err, storeErr) {
		t.Fatalf("delete store err = %v, want wrapped store error", err)
	}
}

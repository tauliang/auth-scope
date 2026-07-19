package slack

import (
	"errors"
	"slices"
	"testing"
	"time"
)

type slackFixedClock struct {
	now time.Time
}

func (c slackFixedClock) Now() time.Time {
	return c.now
}

type slackMemoryStore struct {
	bindings  map[string]WorkspaceBinding
	listErr   error
	saveErr   error
	updateErr error
}

func newSlackMemoryStore(bindings ...WorkspaceBinding) *slackMemoryStore {
	store := &slackMemoryStore{bindings: map[string]WorkspaceBinding{}}
	for _, binding := range bindings {
		store.bindings[binding.BindingID] = binding
	}
	return store
}

func (s *slackMemoryStore) SaveWorkspaceBinding(binding WorkspaceBinding) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *slackMemoryStore) GetWorkspaceBinding(id string) (WorkspaceBinding, error) {
	binding, ok := s.bindings[id]
	if !ok {
		return WorkspaceBinding{}, errors.New("not found")
	}
	return binding, nil
}

func (s *slackMemoryStore) UpdateWorkspaceBinding(binding WorkspaceBinding) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *slackMemoryStore) ListWorkspaceBindings() ([]WorkspaceBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	bindings := make([]WorkspaceBinding, 0, len(s.bindings))
	for _, binding := range s.bindings {
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

type slackEvaluator struct {
	gotMissionRef string
	gotContext    map[string]any
	response      EvaluationResponse
	err           error
}

func (e *slackEvaluator) Evaluate(missionRef string, _ EvaluationRequest, context map[string]any) (EvaluationResponse, error) {
	e.gotMissionRef = missionRef
	e.gotContext = context
	if e.err != nil {
		return EvaluationResponse{}, e.err
	}
	return e.response, nil
}

type slackEventSink struct {
	events []Event
}

func (s *slackEventSink) AppendEvent(event Event) error {
	s.events = append(s.events, event)
	return nil
}

func newSlackService(store *slackMemoryStore, evaluator Evaluator, events *slackEventSink) *Service {
	return NewService(Config{
		Store:     store,
		Evaluator: evaluator,
		Events:    events,
		Clock:     slackFixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
		NewID: func(prefix string) string {
			return prefix + "_test"
		},
	})
}

func TestServiceCreateWorkspaceBindingDefaultsAndLists(t *testing.T) {
	store := newSlackMemoryStore()
	events := &slackEventSink{}
	service := newSlackService(store, nil, events)

	binding, err := service.CreateWorkspaceBinding(CreateWorkspaceBindingRequest{
		WorkspaceID:   " T123 ",
		WorkspaceName: " Acme ",
		WorkspaceURL:  " https://acme.slack.com ",
		MissionRef:    " mref_123 ",
		RequiredRoles: []string{" Admin ", ""},
		AdminRoles:    []string{"Owner"},
		Metadata:      map[string]string{"env": "demo"},
	}, Principal{UserID: "UADMIN", Email: "admin@example.com"})
	if err != nil {
		t.Fatalf("CreateWorkspaceBinding: %v", err)
	}
	if binding.BindingID != "slb_test" || binding.TenantID != "default" {
		t.Fatalf("unexpected identity defaults: %#v", binding)
	}
	if binding.WorkspaceID != "T123" || binding.MissionRef != "mref_123" {
		t.Fatalf("unexpected normalized binding: %#v", binding)
	}
	if binding.RoleClaim != "roles" || binding.RoleMatchMode != RoleMatchAny {
		t.Fatalf("unexpected role defaults: %#v", binding)
	}
	if len(binding.RequiredRoles) != 1 || binding.RequiredRoles[0] != "Admin" {
		t.Fatalf("required roles were not cleaned: %#v", binding.RequiredRoles)
	}
	if len(events.events) != 1 || events.events[0].Type != "slack.workspace_bound" {
		t.Fatalf("workspace bind event not emitted: %#v", events.events)
	}

	list, err := service.ListWorkspaceBindings()
	if err != nil {
		t.Fatalf("ListWorkspaceBindings: %v", err)
	}
	if len(list) != 1 || list[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected binding list: %#v", list)
	}
}

func TestServiceAuthorizeMessageActionAcceptsAndEvaluates(t *testing.T) {
	binding := WorkspaceBinding{
		BindingID:       "slb_1",
		TenantID:        "demo",
		WorkspaceID:     "T123",
		MissionRef:      "mref_123",
		RequiredRoles:   []string{"Workspace Admin"},
		AdminRoles:      []string{"Owner"},
		AllowedChannels: []string{"C111"},
		AllowedUsers:    []string{"U123"},
		AllowedActions:  []string{ActionTypePostMessage},
		RoleClaim:       "slack_roles",
		RoleMatchMode:   RoleMatchAll,
		Status:          WorkspaceBindingStatusActive,
	}
	store := newSlackMemoryStore(binding)
	events := &slackEventSink{}
	evaluator := &slackEvaluator{response: EvaluationResponse{
		Decision:       "allow",
		MissionRef:     "mref_123",
		MissionVersion: 7,
		ReasonCodes:    []string{"in_scope"},
	}}
	service := newSlackService(store, evaluator, events)

	resp, err := service.AuthorizeMessageAction(AuthorizeMessageActionRequest{
		WorkspaceID: "T123",
		ChannelID:   " C111 ",
		Action:      ActionTypePostMessage,
		Claims: map[string]any{
			"user_id":     "U123",
			"email":       "agent@example.com",
			"slack_roles": []any{"Workspace Admin", "Owner"},
		},
		Context: map[string]any{"risk": "low"},
		Evaluation: &EvaluationRequest{
			MissionVersionSeen: 6,
			Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "agent"},
			Action: MessageAction{
				Type:      "message_event",
				Resource:  ActionResource{Type: "message", ID: "msg_1", ChannelID: "C111"},
				Operation: "post",
			},
		},
	})
	if err != nil {
		t.Fatalf("AuthorizeMessageAction: %v", err)
	}
	if !resp.Accepted || resp.Status != ResolutionStatusAccepted || !resp.Admin {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Evaluation == nil || resp.Evaluation.MissionVersion != 7 {
		t.Fatalf("expected evaluation response, got %#v", resp.Evaluation)
	}
	if evaluator.gotMissionRef != "mref_123" {
		t.Fatalf("evaluation mission ref = %q", evaluator.gotMissionRef)
	}
	if evaluator.gotContext["slack.binding_id"] != "slb_1" || evaluator.gotContext["risk"] != "low" {
		t.Fatalf("evaluation context missing Slack facts: %#v", evaluator.gotContext)
	}
	updated := store.bindings["slb_1"]
	if updated.LastResolutionStatus != ResolutionStatusAccepted || updated.LastUserID != "U123" {
		t.Fatalf("binding was not updated after acceptance: %#v", updated)
	}
	if len(events.events) != 1 || events.events[0].Type != "slack.message_action_evaluated" {
		t.Fatalf("resolution event not emitted: %#v", events.events)
	}
}

func TestServiceAuthorizeMessageActionDeniesFailClosedRequirements(t *testing.T) {
	store := newSlackMemoryStore(WorkspaceBinding{
		BindingID:       "slb_1",
		TenantID:        "demo",
		WorkspaceID:     "T123",
		MissionRef:      "mref_123",
		RequiredRoles:   []string{"Workspace Admin"},
		AllowedChannels: []string{"C111"},
		BlockedChannels: []string{"C999"},
		AllowedUsers:    []string{"U999"},
		AllowedActions:  []string{ActionTypePostMessage},
		Status:          WorkspaceBindingStatusActive,
	})
	service := newSlackService(store, nil, &slackEventSink{})

	resp, err := service.AuthorizeMessageAction(AuthorizeMessageActionRequest{
		WorkspaceID: "T123",
		UserID:      "U123",
		ChannelID:   "C999",
		Action:      ActionTypeDeleteMessage,
		Roles:       []string{"Member"},
	})
	if err != nil {
		t.Fatalf("AuthorizeMessageAction: %v", err)
	}
	if resp.Accepted || resp.Status != ResolutionStatusDenied {
		t.Fatalf("expected denial, got %#v", resp)
	}
	for _, code := range []string{"slack_user_not_allowed", "slack_channel_blocked", "slack_channel_not_allowed", "slack_required_role_missing", "slack_action_not_allowed"} {
		if !slices.Contains(resp.ReasonCodes, code) {
			t.Fatalf("expected reason %q in %#v", code, resp.ReasonCodes)
		}
	}
	if store.bindings["slb_1"].LastResolutionStatus != ResolutionStatusDenied {
		t.Fatalf("binding was not updated after denial: %#v", store.bindings["slb_1"])
	}
}

func TestServiceAuthorizeMessageActionValidationAndLookupErrors(t *testing.T) {
	service := newSlackService(newSlackMemoryStore(), nil, nil)
	if _, err := service.AuthorizeMessageAction(AuthorizeMessageActionRequest{Action: "post"}); err == nil {
		t.Fatal("expected workspace_id validation error")
	}
	if _, err := service.AuthorizeMessageAction(AuthorizeMessageActionRequest{WorkspaceID: "T123"}); err == nil {
		t.Fatal("expected action validation error")
	}

	listErr := errors.New("list failed")
	store := newSlackMemoryStore()
	store.listErr = listErr
	service = newSlackService(store, nil, nil)
	if _, err := service.AuthorizeMessageAction(AuthorizeMessageActionRequest{WorkspaceID: "T123", Action: "post"}); !errors.Is(err, listErr) {
		t.Fatalf("expected list error, got %v", err)
	}

	service = newSlackService(newSlackMemoryStore(WorkspaceBinding{
		BindingID:   "slb_1",
		WorkspaceID: "T123",
		MissionRef:  "mref_123",
		Status:      WorkspaceBindingStatusActive,
	}), nil, nil)
	if _, err := service.AuthorizeMessageAction(AuthorizeMessageActionRequest{
		WorkspaceID: "T123",
		UserID:      "U123",
		Action:      "post",
		Evaluation:  &EvaluationRequest{},
	}); err == nil {
		t.Fatal("expected missing evaluator error")
	}

	resp, err := service.AuthorizeMessageAction(AuthorizeMessageActionRequest{WorkspaceID: "T404", UserID: "U123", Action: "post"})
	if err != nil {
		t.Fatalf("no-binding authorization should not error: %v", err)
	}
	if resp.Accepted || !slices.Contains(resp.ReasonCodes, "slack_no_matching_binding") {
		t.Fatalf("expected no matching binding denial, got %#v", resp)
	}
}

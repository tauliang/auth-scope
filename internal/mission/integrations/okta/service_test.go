package okta

import (
	"errors"
	"slices"
	"testing"
	"time"
)

type oktaFixedClock struct {
	now time.Time
}

func (c oktaFixedClock) Now() time.Time {
	return c.now
}

type oktaMemoryStore struct {
	bindings  map[string]AppBinding
	listErr   error
	saveErr   error
	updateErr error
}

func newOktaMemoryStore(bindings ...AppBinding) *oktaMemoryStore {
	store := &oktaMemoryStore{bindings: map[string]AppBinding{}}
	for _, binding := range bindings {
		store.bindings[binding.BindingID] = binding
	}
	return store
}

func (s *oktaMemoryStore) SaveAppBinding(binding AppBinding) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *oktaMemoryStore) GetAppBinding(id string) (AppBinding, error) {
	binding, ok := s.bindings[id]
	if !ok {
		return AppBinding{}, errors.New("not found")
	}
	return binding, nil
}

func (s *oktaMemoryStore) UpdateAppBinding(binding AppBinding) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *oktaMemoryStore) ListAppBindings() ([]AppBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	bindings := make([]AppBinding, 0, len(s.bindings))
	for _, binding := range s.bindings {
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

type oktaEvaluator struct {
	gotMissionRef string
	gotContext    map[string]any
	gotRequest    EvaluationRequest
	response      EvaluationResponse
	err           error
}

func (e *oktaEvaluator) Evaluate(req EvaluationRequest) (EvaluationResponse, error) {
	e.gotMissionRef = req.MissionRef
	e.gotContext = req.Context
	e.gotRequest = req
	if e.err != nil {
		return EvaluationResponse{}, e.err
	}
	return e.response, nil
}

type oktaEventSink struct {
	events []Event
}

func (s *oktaEventSink) AppendEvent(event Event) error {
	s.events = append(s.events, event)
	return nil
}

func newOktaService(store *oktaMemoryStore, evaluator Evaluator, events *oktaEventSink) *Service {
	return NewService(Config{
		Store:     store,
		Evaluator: evaluator,
		Events:    events,
		Clock:     oktaFixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
		NewID: func(prefix string) string {
			return prefix + "_test"
		},
	})
}

func TestOktaServiceCreateAppBindingDefaultsAndLists(t *testing.T) {
	store := newOktaMemoryStore()
	events := &oktaEventSink{}
	service := newOktaService(store, nil, events)

	binding, err := service.CreateAppBinding(CreateAppBindingRequest{
		Issuer:         " https://acme.okta.com/oauth2/default/ ",
		ClientID:       " 0oa_client ",
		AppID:          " app_1 ",
		AppLabel:       " Auth Scope ",
		MissionRef:     " mref_123 ",
		RequiredGroups: []string{" Mission Operators ", ""},
		AdminGroups:    []string{"Mission Admins"},
		Metadata:       map[string]string{"env": "demo"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateAppBinding: %v", err)
	}
	if binding.BindingID != "okb_test" || binding.TenantID != "default" {
		t.Fatalf("unexpected identity defaults: %#v", binding)
	}
	if binding.Issuer != "https://acme.okta.com/oauth2/default" || binding.AuthorizationServerID != "default" {
		t.Fatalf("issuer metadata was not normalized: %#v", binding)
	}
	if binding.GroupClaim != "groups" || binding.SubjectClaim != "sub" || binding.ScopeClaim != "scp" || binding.GroupMatchMode != GroupMatchAny {
		t.Fatalf("claim defaults not populated: %#v", binding)
	}
	if len(binding.RequiredGroups) != 1 || binding.RequiredGroups[0] != "Mission Operators" {
		t.Fatalf("required groups were not cleaned: %#v", binding.RequiredGroups)
	}
	if len(events.events) != 1 || events.events[0].Type != "okta.app_bound" {
		t.Fatalf("app bind event not emitted: %#v", events.events)
	}

	list, err := service.ListAppBindings()
	if err != nil {
		t.Fatalf("ListAppBindings: %v", err)
	}
	if len(list) != 1 || list[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected binding list: %#v", list)
	}
}

func TestOktaServiceResolveAuthorityContextAcceptsAndEvaluates(t *testing.T) {
	store := newOktaMemoryStore(AppBinding{
		BindingID:       "okb_1",
		TenantID:        "demo",
		Issuer:          "https://acme.okta.com/oauth2/default",
		ClientID:        "0oa_client",
		AppID:           "app_1",
		AppLabel:        "Auth Scope",
		MissionRef:      "mref_123",
		RequiredGroups:  []string{"Mission Operators"},
		AdminGroups:     []string{"Mission Admins"},
		AllowedSubjects: []string{"00u_agent"},
		GroupClaim:      "groups",
		SubjectClaim:    "sub",
		ScopeClaim:      "scp",
		GroupMatchMode:  GroupMatchAll,
		Status:          AppBindingStatusActive,
	})
	events := &oktaEventSink{}
	evaluator := &oktaEvaluator{response: EvaluationResponse{Decision: "allow", MissionRef: "mref_123", MissionVersion: 3}}
	service := newOktaService(store, evaluator, events)

	resp, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		TenantID:   "demo",
		MissionRef: "mref_123",
		Claims: map[string]any{
			"iss":    "https://acme.okta.com/oauth2/default",
			"cid":    "0oa_client",
			"sub":    "00u_agent",
			"groups": []any{"Mission Operators", "Mission Admins"},
			"scp":    "openid groups",
		},
		Context: map[string]any{"risk": "low"},
		Evaluation: &EvaluationRequest{
			MissionVersionSeen: 2,
			Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "agent"},
			Action: EvaluationAction{
				Type:      "tool_call",
				Resource:  EvaluationActionResource{Type: "drive_folder", ID: "board"},
				Operation: "read",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveAuthorityContext: %v", err)
	}
	if !resp.Accepted || resp.Status != ResolutionStatusAccepted || !resp.Admin {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Evaluation == nil || resp.Evaluation.MissionVersion != 3 {
		t.Fatalf("expected evaluation response, got %#v", resp.Evaluation)
	}
	if evaluator.gotMissionRef != "mref_123" || evaluator.gotContext["okta.binding_id"] != "okb_1" {
		t.Fatalf("evaluation was not called with authority context: %q %#v", evaluator.gotMissionRef, evaluator.gotContext)
	}
	if store.bindings["okb_1"].LastResolutionStatus != ResolutionStatusAccepted || store.bindings["okb_1"].LastSubject != "00u_agent" {
		t.Fatalf("binding was not updated after acceptance: %#v", store.bindings["okb_1"])
	}
	if len(events.events) != 1 || events.events[0].Type != "okta.authority_context_resolved" {
		t.Fatalf("resolution event not emitted: %#v", events.events)
	}
}

func TestOktaServiceResolveAuthorityContextDeniesRequirements(t *testing.T) {
	store := newOktaMemoryStore(AppBinding{
		BindingID:       "okb_1",
		TenantID:        "demo",
		Issuer:          "https://acme.okta.com/oauth2/default",
		ClientID:        "0oa_client",
		MissionRef:      "mref_123",
		RequiredGroups:  []string{"Mission Operators"},
		AllowedSubjects: []string{"00u_allowed"},
		GroupMatchMode:  GroupMatchAll,
		Status:          AppBindingStatusActive,
	})
	service := newOktaService(store, nil, &oktaEventSink{})

	resp, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://acme.okta.com/oauth2/default",
			"cid":    "0oa_client",
			"sub":    "00u_other",
			"groups": []any{"Everyone"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveAuthorityContext: %v", err)
	}
	if resp.Accepted || resp.Status != ResolutionStatusDenied {
		t.Fatalf("expected denial, got %#v", resp)
	}
	for _, code := range []string{"okta_subject_not_allowed", "okta_required_group_missing"} {
		if !slices.Contains(resp.ReasonCodes, code) {
			t.Fatalf("expected reason %q in %#v", code, resp.ReasonCodes)
		}
	}
	if store.bindings["okb_1"].LastResolutionStatus != ResolutionStatusDenied {
		t.Fatalf("binding was not updated after denial: %#v", store.bindings["okb_1"])
	}
}

func TestOktaServiceValidationLookupAndEvaluatorErrors(t *testing.T) {
	service := newOktaService(newOktaMemoryStore(), nil, nil)
	for name, req := range map[string]CreateAppBindingRequest{
		"issuer":           {ClientID: "0oa", MissionRef: "mref"},
		"client":           {Issuer: "https://acme.okta.com/oauth2/default", MissionRef: "mref"},
		"mission":          {Issuer: "https://acme.okta.com/oauth2/default", ClientID: "0oa"},
		"group match mode": {Issuer: "https://acme.okta.com/oauth2/default", ClientID: "0oa", MissionRef: "mref", GroupMatchMode: "sometimes"},
	} {
		if _, err := service.CreateAppBinding(req, Principal{}); err == nil {
			t.Fatalf("expected create validation error for %s", name)
		}
	}

	resp, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://acme.okta.com/oauth2/default", "cid": "0oa", "sub": "00u"},
	})
	if err != nil {
		t.Fatalf("no-match resolution should not error: %v", err)
	}
	if resp.Accepted || !slices.Contains(resp.ReasonCodes, "okta_no_matching_binding") {
		t.Fatalf("expected no matching binding denial, got %#v", resp)
	}

	listErr := errors.New("list failed")
	store := newOktaMemoryStore()
	store.listErr = listErr
	service = newOktaService(store, nil, nil)
	if _, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://acme.okta.com/oauth2/default", "cid": "0oa", "sub": "00u"},
	}); !errors.Is(err, listErr) {
		t.Fatalf("expected list error, got %v", err)
	}

	service = newOktaService(newOktaMemoryStore(AppBinding{
		BindingID:  "okb_1",
		Issuer:     "https://acme.okta.com/oauth2/default",
		ClientID:   "0oa",
		MissionRef: "mref",
		Status:     AppBindingStatusActive,
	}), nil, nil)
	if _, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims:     map[string]any{"iss": "https://acme.okta.com/oauth2/default", "cid": "0oa", "sub": "00u"},
		Evaluation: &EvaluationRequest{},
	}); err == nil {
		t.Fatal("expected missing evaluator error")
	}
}

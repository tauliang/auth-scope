package entra

import (
	"errors"
	"slices"
	"testing"
	"time"
)

type entraFixedClock struct {
	now time.Time
}

func (c entraFixedClock) Now() time.Time {
	return c.now
}

type entraMemoryStore struct {
	registrations map[string]AppRegistration
	listErr       error
	saveErr       error
	updateErr     error
}

func newEntraMemoryStore(registrations ...AppRegistration) *entraMemoryStore {
	store := &entraMemoryStore{registrations: map[string]AppRegistration{}}
	for _, registration := range registrations {
		store.registrations[registration.RegistrationID] = registration
	}
	return store
}

func (s *entraMemoryStore) SaveAppRegistration(registration AppRegistration) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.registrations[registration.RegistrationID] = registration
	return nil
}

func (s *entraMemoryStore) GetAppRegistration(id string) (AppRegistration, error) {
	registration, ok := s.registrations[id]
	if !ok {
		return AppRegistration{}, errors.New("not found")
	}
	return registration, nil
}

func (s *entraMemoryStore) UpdateAppRegistration(registration AppRegistration) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.registrations[registration.RegistrationID] = registration
	return nil
}

func (s *entraMemoryStore) ListAppRegistrations() ([]AppRegistration, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	registrations := make([]AppRegistration, 0, len(s.registrations))
	for _, registration := range s.registrations {
		registrations = append(registrations, registration)
	}
	return registrations, nil
}

type entraEvaluator struct {
	gotMissionRef string
	gotContext    map[string]any
	gotRequest    EvaluationRequest
	response      EvaluationResponse
	err           error
}

func (e *entraEvaluator) Evaluate(req EvaluationRequest) (EvaluationResponse, error) {
	e.gotMissionRef = req.MissionRef
	e.gotContext = req.Context
	e.gotRequest = req
	if e.err != nil {
		return EvaluationResponse{}, e.err
	}
	return e.response, nil
}

type entraEventSink struct {
	events []Event
}

func (s *entraEventSink) AppendEvent(event Event) error {
	s.events = append(s.events, event)
	return nil
}

func newEntraService(store *entraMemoryStore, evaluator Evaluator, events *entraEventSink) *Service {
	return NewService(Config{
		Store:     store,
		Evaluator: evaluator,
		Events:    events,
		Clock:     entraFixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
		NewID: func(prefix string) string {
			return prefix + "_test"
		},
	})
}

func TestEntraServiceCreateAppRegistrationDefaultsAndLists(t *testing.T) {
	store := newEntraMemoryStore()
	events := &entraEventSink{}
	service := newEntraService(store, nil, events)

	registration, err := service.CreateAppRegistration(CreateAppRegistrationRequest{
		TenantName:     " Contoso ",
		Issuer:         " https://login.microsoftonline.com/tenant/v2.0/ ",
		ClientID:       " client_1 ",
		AppID:          " app_1 ",
		AppName:        " Auth Scope ",
		MissionRef:     " mref_123 ",
		RequiredGroups: []string{" Mission Operators ", ""},
		AdminGroups:    []string{"Mission Admins"},
		Metadata:       map[string]string{"env": "demo"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateAppRegistration: %v", err)
	}
	if registration.RegistrationID != "enr_test" || registration.TenantID != "default" {
		t.Fatalf("unexpected identity defaults: %#v", registration)
	}
	if registration.Issuer != "https://login.microsoftonline.com/tenant/v2.0" {
		t.Fatalf("issuer was not normalized: %#v", registration)
	}
	if registration.GroupClaim != "groups" || registration.SubjectClaim != "sub" || registration.RolesClaim != "roles" || registration.GroupMatchMode != GroupMatchAny {
		t.Fatalf("claim defaults not populated: %#v", registration)
	}
	if len(registration.RequiredGroups) != 1 || registration.RequiredGroups[0] != "Mission Operators" {
		t.Fatalf("required groups were not cleaned: %#v", registration.RequiredGroups)
	}
	if len(events.events) != 1 || events.events[0].Type != "entra.app_registered" {
		t.Fatalf("registration event not emitted: %#v", events.events)
	}

	list, err := service.ListAppRegistrations()
	if err != nil {
		t.Fatalf("ListAppRegistrations: %v", err)
	}
	if len(list) != 1 || list[0].RegistrationID != registration.RegistrationID {
		t.Fatalf("unexpected registration list: %#v", list)
	}
}

func TestEntraServiceResolveAuthorityContextAcceptsAndEvaluates(t *testing.T) {
	store := newEntraMemoryStore(AppRegistration{
		RegistrationID:  "enr_1",
		TenantID:        "demo",
		Issuer:          "https://login.microsoftonline.com/tenant/v2.0",
		ClientID:        "client_1",
		AppID:           "app_1",
		AppName:         "Auth Scope",
		MissionRef:      "mref_123",
		RequiredGroups:  []string{"Mission Operators"},
		AdminGroups:     []string{"Mission Admins"},
		AllowedSubjects: []string{"user@example.com"},
		GroupClaim:      "groups",
		SubjectClaim:    "sub",
		RolesClaim:      "roles",
		GroupMatchMode:  GroupMatchAll,
		Status:          AppRegistrationStatusActive,
	})
	events := &entraEventSink{}
	evaluator := &entraEvaluator{response: EvaluationResponse{Decision: "allow", MissionRef: "mref_123", MissionVersion: 4}}
	service := newEntraService(store, evaluator, events)

	resp, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		TenantID:   "demo",
		MissionRef: "mref_123",
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/tenant/v2.0",
			"azp":    "client_1",
			"sub":    "user@example.com",
			"groups": []any{"Mission Operators", "Mission Admins"},
			"roles":  []any{"Reader"},
		},
		Context: map[string]any{"risk": "low"},
		Evaluation: &EvaluationRequest{
			MissionVersionSeen: 3,
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
	if resp.Evaluation == nil || resp.Evaluation.MissionVersion != 4 {
		t.Fatalf("expected evaluation response, got %#v", resp.Evaluation)
	}
	if evaluator.gotMissionRef != "mref_123" || evaluator.gotContext["entra.registration_id"] != "enr_1" {
		t.Fatalf("evaluation was not called with authority context: %q %#v", evaluator.gotMissionRef, evaluator.gotContext)
	}
	if store.registrations["enr_1"].LastResolutionStatus != ResolutionStatusAccepted || store.registrations["enr_1"].LastSubject != "user@example.com" {
		t.Fatalf("registration was not updated after acceptance: %#v", store.registrations["enr_1"])
	}
	if len(events.events) != 1 || events.events[0].Type != "entra.authority_context_resolved" {
		t.Fatalf("resolution event not emitted: %#v", events.events)
	}
}

func TestEntraServiceResolveAuthorityContextDeniesRequirements(t *testing.T) {
	store := newEntraMemoryStore(AppRegistration{
		RegistrationID:  "enr_1",
		TenantID:        "demo",
		Issuer:          "https://login.microsoftonline.com/tenant/v2.0",
		ClientID:        "client_1",
		MissionRef:      "mref_123",
		RequiredGroups:  []string{"Mission Operators"},
		AllowedSubjects: []string{"allowed@example.com"},
		GroupMatchMode:  GroupMatchAll,
		Status:          AppRegistrationStatusActive,
	})
	service := newEntraService(store, nil, &entraEventSink{})

	resp, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/tenant/v2.0",
			"appid":  "client_1",
			"sub":    "other@example.com",
			"groups": []any{"Everyone"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveAuthorityContext: %v", err)
	}
	if resp.Accepted || resp.Status != ResolutionStatusDenied {
		t.Fatalf("expected denial, got %#v", resp)
	}
	for _, code := range []string{"entra_subject_not_allowed", "entra_required_group_missing"} {
		if !slices.Contains(resp.ReasonCodes, code) {
			t.Fatalf("expected reason %q in %#v", code, resp.ReasonCodes)
		}
	}
	if store.registrations["enr_1"].LastResolutionStatus != ResolutionStatusDenied {
		t.Fatalf("registration was not updated after denial: %#v", store.registrations["enr_1"])
	}
}

func TestEntraServiceValidationLookupAndEvaluatorErrors(t *testing.T) {
	service := newEntraService(newEntraMemoryStore(), nil, nil)
	for name, req := range map[string]CreateAppRegistrationRequest{
		"issuer":           {ClientID: "client", MissionRef: "mref"},
		"client":           {Issuer: "https://login.microsoftonline.com/tenant/v2.0", MissionRef: "mref"},
		"mission":          {Issuer: "https://login.microsoftonline.com/tenant/v2.0", ClientID: "client"},
		"group match mode": {Issuer: "https://login.microsoftonline.com/tenant/v2.0", ClientID: "client", MissionRef: "mref", GroupMatchMode: "sometimes"},
	} {
		if _, err := service.CreateAppRegistration(req, Principal{}); err == nil {
			t.Fatalf("expected create validation error for %s", name)
		}
	}

	resp, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://login.microsoftonline.com/tenant/v2.0", "azp": "client", "sub": "user"},
	})
	if err != nil {
		t.Fatalf("no-match resolution should not error: %v", err)
	}
	if resp.Accepted || !slices.Contains(resp.ReasonCodes, "entra_no_matching_registration") {
		t.Fatalf("expected no matching registration denial, got %#v", resp)
	}

	listErr := errors.New("list failed")
	store := newEntraMemoryStore()
	store.listErr = listErr
	service = newEntraService(store, nil, nil)
	if _, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://login.microsoftonline.com/tenant/v2.0", "azp": "client", "sub": "user"},
	}); !errors.Is(err, listErr) {
		t.Fatalf("expected list error, got %v", err)
	}

	service = newEntraService(newEntraMemoryStore(AppRegistration{
		RegistrationID: "enr_1",
		Issuer:         "https://login.microsoftonline.com/tenant/v2.0",
		ClientID:       "client",
		MissionRef:     "mref",
		Status:         AppRegistrationStatusActive,
	}), nil, nil)
	if _, err := service.ResolveAuthorityContext(ResolveAuthorityContextRequest{
		Claims:     map[string]any{"iss": "https://login.microsoftonline.com/tenant/v2.0", "azp": "client", "sub": "user"},
		Evaluation: &EvaluationRequest{},
	}); err == nil {
		t.Fatal("expected missing evaluator error")
	}
}

package salesforce

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type salesforceFixedClock struct {
	now time.Time
}

func (c salesforceFixedClock) Now() time.Time {
	return c.now
}

type salesforceMemoryStore struct {
	bindings  map[string]OrgBinding
	listErr   error
	saveErr   error
	updateErr error
}

func newSalesforceMemoryStore(bindings ...OrgBinding) *salesforceMemoryStore {
	store := &salesforceMemoryStore{bindings: map[string]OrgBinding{}}
	for _, binding := range bindings {
		store.bindings[binding.BindingID] = binding
	}
	return store
}

func (s *salesforceMemoryStore) SaveOrgBinding(binding OrgBinding) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *salesforceMemoryStore) GetOrgBinding(id string) (OrgBinding, error) {
	binding, ok := s.bindings[id]
	if !ok {
		return OrgBinding{}, errors.New("not found")
	}
	return binding, nil
}

func (s *salesforceMemoryStore) UpdateOrgBinding(binding OrgBinding) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *salesforceMemoryStore) ListOrgBindings() ([]OrgBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	bindings := make([]OrgBinding, 0, len(s.bindings))
	for _, binding := range s.bindings {
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

type salesforceEvaluator struct {
	gotRequest EvaluationRequest
	response   EvaluationResponse
	err        error
}

func (e *salesforceEvaluator) Evaluate(req EvaluationRequest) (EvaluationResponse, error) {
	e.gotRequest = req
	if e.err != nil {
		return EvaluationResponse{}, e.err
	}
	return e.response, nil
}

type salesforceEventSink struct {
	events []Event
}

func (s *salesforceEventSink) AppendEvent(event Event) error {
	s.events = append(s.events, event)
	return nil
}

func newSalesforceService(store *salesforceMemoryStore, evaluator Evaluator, events *salesforceEventSink) *Service {
	return NewService(Config{
		Store:     store,
		Evaluator: evaluator,
		Events:    events,
		Clock:     salesforceFixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
		NewID: func(prefix string) string {
			return prefix + "_test"
		},
	})
}

func TestSalesforceServiceCreateOrgBindingDefaultsAndLists(t *testing.T) {
	store := newSalesforceMemoryStore()
	events := &salesforceEventSink{}
	service := newSalesforceService(store, nil, events)

	binding, err := service.CreateOrgBinding(CreateOrgBindingRequest{
		InstanceURL:            " https://acme.my.salesforce.com/lightning/setup ",
		OrgID:                  " 00Dxx0000001ABC ",
		OrgName:                " Acme ",
		MissionRef:             " mref_123 ",
		AllowedObjectAPINames:  []string{" Account ", ""},
		AllowedRecordTypeNames: []string{" Customer "},
		AllowedActions:         []string{ActionUpdateRecord},
		RequiredProfiles:       []string{" Standard User "},
		RequiredPermissionSets: []string{" CRM Agent "},
		AdminProfiles:          []string{"System Administrator"},
		Metadata:               map[string]string{"env": "demo"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateOrgBinding: %v", err)
	}
	if binding.BindingID != "sfb_test" || binding.TenantID != "default" {
		t.Fatalf("unexpected identity defaults: %#v", binding)
	}
	if binding.InstanceURL != "https://acme.my.salesforce.com" || binding.OrgID != "00Dxx0000001ABC" {
		t.Fatalf("org metadata was not normalized: %#v", binding)
	}
	if binding.ProfileClaim != "profile" || binding.PermissionSetsClaim != "permission_sets" || binding.PermissionSetMatchMode != PermissionMatchAny {
		t.Fatalf("claim defaults not populated: %#v", binding)
	}
	if len(binding.AllowedObjectAPINames) != 1 || binding.AllowedObjectAPINames[0] != "Account" {
		t.Fatalf("objects were not cleaned: %#v", binding.AllowedObjectAPINames)
	}
	if len(events.events) != 1 || events.events[0].Type != "salesforce.org_bound" {
		t.Fatalf("org bind event not emitted: %#v", events.events)
	}

	list, err := service.ListOrgBindings()
	if err != nil {
		t.Fatalf("ListOrgBindings: %v", err)
	}
	if len(list) != 1 || list[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected binding list: %#v", list)
	}
}

func TestSalesforceServiceAuthorizeRecordActionAcceptsAndEvaluates(t *testing.T) {
	store := newSalesforceMemoryStore(OrgBinding{
		BindingID:              "sfb_1",
		TenantID:               "demo",
		InstanceURL:            "https://acme.my.salesforce.com",
		OrgID:                  "00Dxx0000001ABC",
		MissionRef:             "mref_123",
		AllowedObjectAPINames:  []string{"Account"},
		AllowedRecordTypeNames: []string{"Customer"},
		AllowedActions:         []string{ActionUpdateRecord},
		RequiredProfiles:       []string{"Standard User"},
		RequiredPermissionSets: []string{"CRM Agent"},
		AdminProfiles:          []string{"System Administrator"},
		AdminPermissionSets:    []string{"Mission Admin"},
		AllowedSubjects:        []string{"005xx000001"},
		ProfileClaim:           "profile",
		PermissionSetsClaim:    "permission_sets",
		SubjectClaim:           "sub",
		UsernameClaim:          "username",
		EmailClaim:             "email",
		PermissionSetMatchMode: PermissionMatchAny,
		Status:                 OrgBindingStatusActive,
	})
	events := &salesforceEventSink{}
	evaluator := &salesforceEvaluator{response: EvaluationResponse{
		Decision:       "allow",
		MissionRef:     "mref_123",
		MissionVersion: 3,
		ReasonCodes:    []string{"in_scope"},
	}}
	service := newSalesforceService(store, evaluator, events)

	resp, err := service.AuthorizeRecordAction(AuthorizeRecordActionRequest{
		TenantID:       "demo",
		MissionRef:     "mref_123",
		InstanceURL:    "https://acme.my.salesforce.com/",
		ObjectAPIName:  "Account",
		RecordID:       "001xx000003DGbY",
		RecordTypeName: "Customer",
		Action:         ActionUpdateRecord,
		Claims: map[string]any{
			"user_id":         "005xx000001",
			"sub":             "005xx000001",
			"username":        "agent@example.com",
			"email":           "agent@example.com",
			"profile":         "Standard User",
			"permission_sets": []any{"CRM Agent", "Mission Admin"},
		},
		Context: map[string]any{"risk": "low"},
		Evaluation: &EvaluationRequest{
			MissionVersionSeen: 2,
			Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: EvaluationAction{
				Type:      "salesforce_record",
				Resource:  EvaluationActionResource{Type: "salesforce_record", ID: "Account:001xx000003DGbY"},
				Operation: "update",
			},
		},
	})
	if err != nil {
		t.Fatalf("AuthorizeRecordAction: %v", err)
	}
	if !resp.Accepted || !resp.Admin || resp.ObjectAPIName != "Account" {
		t.Fatalf("record authorization = %#v, want accepted admin Account context", resp)
	}
	if resp.Evaluation == nil || resp.Evaluation.Decision != "allow" {
		t.Fatalf("record evaluation = %#v, want allow", resp.Evaluation)
	}
	if evaluator.gotRequest.MissionRef != "mref_123" || evaluator.gotRequest.Context["salesforce.record_id"] != "001xx000003DGbY" {
		t.Fatalf("evaluator request = %#v, want enriched Salesforce context", evaluator.gotRequest)
	}
	if len(events.events) != 1 || events.events[0].Type != "salesforce.record_action_authorized" {
		t.Fatalf("record event not emitted: %#v", events.events)
	}
}

func TestSalesforceServiceAuthorizeRecordActionDeniesOutOfScopeObject(t *testing.T) {
	store := newSalesforceMemoryStore(OrgBinding{
		BindingID:              "sfb_1",
		TenantID:               "demo",
		InstanceURL:            "https://acme.my.salesforce.com",
		MissionRef:             "mref_123",
		AllowedObjectAPINames:  []string{"Account"},
		AllowedActions:         []string{ActionUpdateRecord},
		RequiredPermissionSets: []string{"CRM Agent"},
		SubjectClaim:           "sub",
		PermissionSetsClaim:    "permission_sets",
		PermissionSetMatchMode: PermissionMatchAny,
		Status:                 OrgBindingStatusActive,
	})
	events := &salesforceEventSink{}
	service := newSalesforceService(store, nil, events)

	resp, err := service.AuthorizeRecordAction(AuthorizeRecordActionRequest{
		TenantID:       "demo",
		MissionRef:     "mref_123",
		InstanceURL:    "https://acme.my.salesforce.com",
		ObjectAPIName:  "Opportunity",
		RecordID:       "006xx000004TmiE",
		Action:         ActionUpdateRecord,
		Subject:        "005xx000001",
		PermissionSets: []string{"CRM Agent"},
	})
	if err != nil {
		t.Fatalf("AuthorizeRecordAction: %v", err)
	}
	if resp.Accepted || resp.Status != ResolutionStatusDenied || !ContainsString(resp.ReasonCodes, "salesforce_object_not_allowed") {
		t.Fatalf("record response = %#v, want object denial", resp)
	}
	if len(events.events) != 1 || events.events[0].Type != "salesforce.record_action_authorized" {
		t.Fatalf("record event not emitted: %#v", events.events)
	}
}

func TestSalesforceServiceBindingLookupFailsClosed(t *testing.T) {
	store := newSalesforceMemoryStore(OrgBinding{
		BindingID:             "sfb_1",
		TenantID:              "demo",
		InstanceURL:           "https://acme.my.salesforce.com",
		MissionRef:            "mref_123",
		AllowedObjectAPINames: []string{"Account"},
		AllowedActions:        []string{ActionUpdateRecord},
		SubjectClaim:          "sub",
		Status:                OrgBindingStatusActive,
	})
	service := newSalesforceService(store, nil, &salesforceEventSink{})

	_, err := service.AuthorizeRecordAction(AuthorizeRecordActionRequest{
		TenantID:      "demo",
		ObjectAPIName: "Account",
		Action:        ActionUpdateRecord,
		Subject:       "005xx000001",
	})
	if err == nil || !strings.Contains(err.Error(), "instance_url or org_id") {
		t.Fatalf("AuthorizeRecordAction missing instance/org err = %v, want required-org error", err)
	}

	denied, err := service.AuthorizeRecordAction(AuthorizeRecordActionRequest{
		TenantID:      "demo",
		OrgID:         "00Dxx0000001ABC",
		ObjectAPIName: "Account",
		Action:        ActionUpdateRecord,
		Subject:       "005xx000001",
	})
	if err != nil {
		t.Fatalf("AuthorizeRecordAction org-only no match: %v", err)
	}
	if denied.Accepted || !ContainsString(denied.ReasonCodes, "salesforce_no_matching_binding") {
		t.Fatalf("org-only response = %#v, want no matching binding", denied)
	}

	binding := store.bindings["sfb_1"]
	binding.OrgID = "00Dxx0000001ABC"
	store.bindings["sfb_1"] = binding

	accepted, err := service.AuthorizeRecordAction(AuthorizeRecordActionRequest{
		TenantID:      "demo",
		OrgID:         "00Dxx0000001ABC",
		ObjectAPIName: "Account",
		Action:        ActionUpdateRecord,
		Subject:       "005xx000001",
	})
	if err != nil {
		t.Fatalf("AuthorizeRecordAction org-only match: %v", err)
	}
	if !accepted.Accepted || accepted.BindingID != "sfb_1" {
		t.Fatalf("org-only response = %#v, want exact org binding", accepted)
	}
}

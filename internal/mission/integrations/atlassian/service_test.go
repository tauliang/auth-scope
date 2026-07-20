package atlassian

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type atlassianFixedClock struct {
	now time.Time
}

func (c atlassianFixedClock) Now() time.Time {
	return c.now
}

type atlassianMemoryStore struct {
	bindings  map[string]SiteBinding
	listErr   error
	saveErr   error
	updateErr error
}

func newAtlassianMemoryStore(bindings ...SiteBinding) *atlassianMemoryStore {
	store := &atlassianMemoryStore{bindings: map[string]SiteBinding{}}
	for _, binding := range bindings {
		store.bindings[binding.BindingID] = binding
	}
	return store
}

func (s *atlassianMemoryStore) SaveSiteBinding(binding SiteBinding) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *atlassianMemoryStore) GetSiteBinding(id string) (SiteBinding, error) {
	binding, ok := s.bindings[id]
	if !ok {
		return SiteBinding{}, errors.New("not found")
	}
	return binding, nil
}

func (s *atlassianMemoryStore) UpdateSiteBinding(binding SiteBinding) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *atlassianMemoryStore) ListSiteBindings() ([]SiteBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	bindings := make([]SiteBinding, 0, len(s.bindings))
	for _, binding := range s.bindings {
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

type atlassianEvaluator struct {
	gotRequest EvaluationRequest
	response   EvaluationResponse
	err        error
}

func (e *atlassianEvaluator) Evaluate(req EvaluationRequest) (EvaluationResponse, error) {
	e.gotRequest = req
	if e.err != nil {
		return EvaluationResponse{}, e.err
	}
	return e.response, nil
}

type atlassianEventSink struct {
	events []Event
}

func (s *atlassianEventSink) AppendEvent(event Event) error {
	s.events = append(s.events, event)
	return nil
}

func newAtlassianService(store *atlassianMemoryStore, evaluator Evaluator, events *atlassianEventSink) *Service {
	return NewService(Config{
		Store:     store,
		Evaluator: evaluator,
		Events:    events,
		Clock:     atlassianFixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
		NewID: func(prefix string) string {
			return prefix + "_test"
		},
	})
}

func TestAtlassianServiceCreateSiteBindingDefaultsAndLists(t *testing.T) {
	store := newAtlassianMemoryStore()
	events := &atlassianEventSink{}
	service := newAtlassianService(store, nil, events)

	binding, err := service.CreateSiteBinding(CreateSiteBindingRequest{
		SiteURL:             " https://acme.atlassian.net/ ",
		CloudID:             " cloud_123 ",
		SiteName:            " Acme ",
		MissionRef:          " mref_123 ",
		JiraProjectKeys:     []string{" fin ", ""},
		ConfluenceSpaceKeys: []string{" eng "},
		AllowedJiraActions:  []string{JiraActionUpdateIssue},
		AllowedPageActions:  []string{ConfluenceActionUpdatePage},
		RequiredGroups:      []string{" Mission Operators ", ""},
		AdminGroups:         []string{"Mission Admins"},
		Metadata:            map[string]string{"env": "demo"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateSiteBinding: %v", err)
	}
	if binding.BindingID != "atb_test" || binding.TenantID != "default" {
		t.Fatalf("unexpected identity defaults: %#v", binding)
	}
	if binding.SiteURL != "https://acme.atlassian.net" || binding.CloudID != "cloud_123" {
		t.Fatalf("site metadata was not normalized: %#v", binding)
	}
	if binding.GroupClaim != "groups" || binding.SubjectClaim != "sub" || binding.EmailClaim != "email" || binding.GroupMatchMode != GroupMatchAny {
		t.Fatalf("claim defaults not populated: %#v", binding)
	}
	if len(binding.JiraProjectKeys) != 1 || binding.JiraProjectKeys[0] != "FIN" {
		t.Fatalf("project keys were not cleaned: %#v", binding.JiraProjectKeys)
	}
	if len(events.events) != 1 || events.events[0].Type != "atlassian.site_bound" {
		t.Fatalf("site bind event not emitted: %#v", events.events)
	}

	list, err := service.ListSiteBindings()
	if err != nil {
		t.Fatalf("ListSiteBindings: %v", err)
	}
	if len(list) != 1 || list[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected binding list: %#v", list)
	}
}

func TestAtlassianServiceAuthorizeJiraIssueActionAcceptsAndEvaluates(t *testing.T) {
	store := newAtlassianMemoryStore(SiteBinding{
		BindingID:          "atb_1",
		TenantID:           "demo",
		SiteURL:            "https://acme.atlassian.net",
		CloudID:            "cloud_123",
		MissionRef:         "mref_123",
		JiraProjectKeys:    []string{"FIN"},
		AllowedJiraActions: []string{JiraActionTransitionIssue},
		RequiredGroups:     []string{"Mission Operators"},
		AdminGroups:        []string{"Mission Admins"},
		AllowedSubjects:    []string{"acc_123"},
		GroupClaim:         "groups",
		SubjectClaim:       "sub",
		EmailClaim:         "email",
		GroupMatchMode:     GroupMatchAll,
		Status:             SiteBindingStatusActive,
	})
	events := &atlassianEventSink{}
	evaluator := &atlassianEvaluator{response: EvaluationResponse{
		Decision:       "allow",
		MissionRef:     "mref_123",
		MissionVersion: 3,
		ReasonCodes:    []string{"in_scope"},
	}}
	service := newAtlassianService(store, evaluator, events)

	resp, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		TenantID:   "demo",
		MissionRef: "mref_123",
		SiteURL:    "https://acme.atlassian.net/",
		IssueKey:   "fin-77",
		IssueType:  "Change",
		Action:     JiraActionTransitionIssue,
		Claims: map[string]any{
			"account_id": "acc_123",
			"sub":        "acc_123",
			"email":      "agent@example.com",
			"groups":     []any{"Mission Operators", "Mission Admins"},
		},
		Context: map[string]any{"risk": "low"},
		Evaluation: &EvaluationRequest{
			MissionVersionSeen: 2,
			Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: EvaluationAction{
				Type:      "jira_issue",
				Resource:  EvaluationActionResource{Type: "jira_issue", ID: "FIN-77"},
				Operation: "transition",
			},
		},
	})
	if err != nil {
		t.Fatalf("AuthorizeJiraIssueAction: %v", err)
	}
	if !resp.Accepted || !resp.Admin || resp.ProjectKey != "FIN" {
		t.Fatalf("jira authorization = %#v, want accepted admin FIN context", resp)
	}
	if resp.Evaluation == nil || resp.Evaluation.Decision != "allow" {
		t.Fatalf("jira evaluation = %#v, want allow", resp.Evaluation)
	}
	if evaluator.gotRequest.MissionRef != "mref_123" || evaluator.gotRequest.Context["jira.issue_key"] != "FIN-77" {
		t.Fatalf("evaluator request = %#v, want enriched Atlassian context", evaluator.gotRequest)
	}
	if len(events.events) != 1 || events.events[0].Type != "atlassian.jira_issue_action_authorized" {
		t.Fatalf("jira event not emitted: %#v", events.events)
	}
}

func TestAtlassianServiceAuthorizeConfluencePageActionDeniesOutOfScopeSpace(t *testing.T) {
	store := newAtlassianMemoryStore(SiteBinding{
		BindingID:           "atb_1",
		TenantID:            "demo",
		SiteURL:             "https://acme.atlassian.net",
		MissionRef:          "mref_123",
		ConfluenceSpaceKeys: []string{"ENG"},
		AllowedPageActions:  []string{ConfluenceActionUpdatePage},
		RequiredGroups:      []string{"Mission Operators"},
		GroupClaim:          "groups",
		SubjectClaim:        "sub",
		EmailClaim:          "email",
		GroupMatchMode:      GroupMatchAny,
		Status:              SiteBindingStatusActive,
	})
	events := &atlassianEventSink{}
	service := newAtlassianService(store, nil, events)

	resp, err := service.AuthorizeConfluencePageAction(AuthorizeConfluencePageActionRequest{
		TenantID:   "demo",
		MissionRef: "mref_123",
		SiteURL:    "https://acme.atlassian.net",
		SpaceKey:   "FIN",
		PageID:     "12345",
		Action:     ConfluenceActionUpdatePage,
		Subject:    "acc_123",
		Groups:     []string{"Mission Operators"},
	})
	if err != nil {
		t.Fatalf("AuthorizeConfluencePageAction: %v", err)
	}
	if resp.Accepted || resp.Status != ResolutionStatusDenied || !ContainsString(resp.ReasonCodes, "confluence_space_not_allowed") {
		t.Fatalf("confluence response = %#v, want space denial", resp)
	}
	if len(events.events) != 1 || events.events[0].Type != "atlassian.confluence_page_action_authorized" {
		t.Fatalf("confluence event not emitted: %#v", events.events)
	}
}

func TestAtlassianServiceBindingLookupFailsClosed(t *testing.T) {
	store := newAtlassianMemoryStore(SiteBinding{
		BindingID:          "atb_1",
		TenantID:           "demo",
		SiteURL:            "https://acme.atlassian.net",
		MissionRef:         "mref_123",
		JiraProjectKeys:    []string{"FIN"},
		AllowedJiraActions: []string{JiraActionTransitionIssue},
		SubjectClaim:       "sub",
		Status:             SiteBindingStatusActive,
	})
	service := newAtlassianService(store, nil, &atlassianEventSink{})

	_, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		TenantID: "demo",
		IssueKey: "FIN-77",
		Action:   JiraActionTransitionIssue,
		Subject:  "acc_123",
	})
	if err == nil || !strings.Contains(err.Error(), "site_url or cloud_id") {
		t.Fatalf("AuthorizeJiraIssueAction missing site/cloud err = %v, want required-site error", err)
	}

	denied, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		TenantID: "demo",
		CloudID:  "cloud_123",
		IssueKey: "FIN-77",
		Action:   JiraActionTransitionIssue,
		Subject:  "acc_123",
	})
	if err != nil {
		t.Fatalf("AuthorizeJiraIssueAction cloud-only no match: %v", err)
	}
	if denied.Accepted || !ContainsString(denied.ReasonCodes, "atlassian_no_matching_binding") {
		t.Fatalf("cloud-only response = %#v, want no matching binding", denied)
	}

	binding := store.bindings["atb_1"]
	binding.CloudID = "cloud_123"
	store.bindings["atb_1"] = binding

	accepted, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		TenantID: "demo",
		CloudID:  "cloud_123",
		IssueKey: "FIN-77",
		Action:   JiraActionTransitionIssue,
		Subject:  "acc_123",
	})
	if err != nil {
		t.Fatalf("AuthorizeJiraIssueAction cloud-only match: %v", err)
	}
	if !accepted.Accepted || accepted.BindingID != "atb_1" {
		t.Fatalf("cloud-only response = %#v, want exact cloud binding", accepted)
	}
}

func TestAtlassianServiceValidationAndErrorBranches(t *testing.T) {
	if NewService(Config{}).isConflict(errors.New("anything")) {
		t.Fatal("nil conflict classifier should fail closed")
	}

	for _, test := range []struct {
		name string
		req  CreateSiteBindingRequest
	}{
		{name: "bad site", req: CreateSiteBindingRequest{SiteURL: "acme.atlassian.net", MissionRef: "mref"}},
		{name: "missing mission", req: CreateSiteBindingRequest{SiteURL: "https://acme.atlassian.net"}},
		{name: "bad group mode", req: CreateSiteBindingRequest{SiteURL: "https://acme.atlassian.net", MissionRef: "mref", GroupMatchMode: "sometimes"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newAtlassianService(newAtlassianMemoryStore(), nil, nil).CreateSiteBinding(test.req, Principal{}); err == nil {
				t.Fatal("expected create validation error")
			}
		})
	}

	store := newAtlassianMemoryStore()
	store.saveErr = errors.New("save failed")
	if _, err := newAtlassianService(store, nil, nil).CreateSiteBinding(CreateSiteBindingRequest{
		SiteURL:    "https://acme.atlassian.net",
		MissionRef: "mref",
	}, Principal{}); err == nil {
		t.Fatal("expected save error")
	}
}

func TestAtlassianServiceAuthorizationEdgeBranches(t *testing.T) {
	active := SiteBinding{
		BindingID:            "atb_1",
		TenantID:             "demo",
		SiteURL:              "https://acme.atlassian.net",
		CloudID:              "cloud_123",
		MissionRef:           "mref_123",
		JiraProjectKeys:      []string{"FIN"},
		ConfluenceSpaceKeys:  []string{"ENG"},
		AllowedJiraActions:   []string{JiraActionTransitionIssue},
		AllowedPageActions:   []string{ConfluenceActionUpdatePage},
		RequiredGroups:       []string{"Mission Operators"},
		AllowedSubjects:      []string{"acc_123"},
		GroupClaim:           "groups",
		SubjectClaim:         "sub",
		EmailClaim:           "email",
		GroupMatchMode:       GroupMatchAll,
		Status:               SiteBindingStatusActive,
		LastResolutionStatus: ResolutionStatusAccepted,
	}
	service := newAtlassianService(newAtlassianMemoryStore(
		SiteBinding{BindingID: "disabled", SiteURL: "https://disabled.atlassian.net", CloudID: "disabled", Status: SiteBindingStatusDisabled},
		active,
	), nil, &atlassianEventSink{})

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "jira missing action", call: func() error {
			_, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{IssueKey: "FIN-1", SiteURL: "https://acme.atlassian.net"})
			return err
		}},
		{name: "jira missing project", call: func() error {
			_, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{Action: JiraActionTransitionIssue, SiteURL: "https://acme.atlassian.net"})
			return err
		}},
		{name: "jira missing user context", call: func() error {
			_, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{Action: JiraActionTransitionIssue, IssueKey: "FIN-1", SiteURL: "https://acme.atlassian.net"})
			return err
		}},
		{name: "confluence missing action", call: func() error {
			_, err := service.AuthorizeConfluencePageAction(AuthorizeConfluencePageActionRequest{SpaceKey: "ENG", SiteURL: "https://acme.atlassian.net"})
			return err
		}},
		{name: "confluence missing space", call: func() error {
			_, err := service.AuthorizeConfluencePageAction(AuthorizeConfluencePageActionRequest{Action: ConfluenceActionUpdatePage, SiteURL: "https://acme.atlassian.net"})
			return err
		}},
		{name: "confluence missing user context", call: func() error {
			_, err := service.AuthorizeConfluencePageAction(AuthorizeConfluencePageActionRequest{Action: ConfluenceActionUpdatePage, SpaceKey: "ENG", SiteURL: "https://acme.atlassian.net"})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	deniedJira, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		TenantID: "demo",
		SiteURL:  "https://acme.atlassian.net",
		IssueKey: "HR-1",
		Action:   JiraActionCommentIssue,
		Subject:  "wrong-subject",
		Groups:   []string{"Other"},
	})
	if err != nil {
		t.Fatalf("AuthorizeJiraIssueAction denied: %v", err)
	}
	for _, code := range []string{"atlassian_subject_not_allowed", "atlassian_required_group_missing", "jira_project_not_allowed", "jira_action_not_allowed"} {
		if !ContainsString(deniedJira.ReasonCodes, code) {
			t.Fatalf("jira reasons = %#v, missing %s", deniedJira.ReasonCodes, code)
		}
	}

	deniedConfluence, err := service.AuthorizeConfluencePageAction(AuthorizeConfluencePageActionRequest{
		TenantID: "demo",
		SiteURL:  "https://acme.atlassian.net",
		SpaceKey: "ENG",
		Action:   ConfluenceActionCommentPage,
		Subject:  "acc_123",
		Groups:   []string{"Mission Operators"},
	})
	if err != nil {
		t.Fatalf("AuthorizeConfluencePageAction denied: %v", err)
	}
	if !ContainsString(deniedConfluence.ReasonCodes, "confluence_action_not_allowed") {
		t.Fatalf("confluence reasons = %#v, want action denial", deniedConfluence.ReasonCodes)
	}

	_, err = service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		TenantID: "demo",
		SiteURL:  "://bad",
		IssueKey: "FIN-1",
		Action:   JiraActionTransitionIssue,
		Subject:  "acc_123",
		Groups:   []string{"Mission Operators"},
	})
	if err == nil {
		t.Fatal("expected invalid site URL error")
	}

	listErrStore := newAtlassianMemoryStore(active)
	listErrStore.listErr = errors.New("list failed")
	_, err = newAtlassianService(listErrStore, nil, nil).AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		SiteURL:  "https://acme.atlassian.net",
		IssueKey: "FIN-1",
		Action:   JiraActionTransitionIssue,
	})
	if err == nil {
		t.Fatal("expected list error")
	}

	noMatch, err := service.AuthorizeJiraIssueAction(AuthorizeJiraIssueActionRequest{
		TenantID: "other",
		SiteURL:  "https://disabled.atlassian.net",
		CloudID:  "wrong-cloud",
		IssueKey: "FIN-1",
		Action:   JiraActionTransitionIssue,
		Subject:  "acc_123",
		Groups:   []string{"Mission Operators"},
	})
	if err != nil {
		t.Fatalf("AuthorizeJiraIssueAction no match: %v", err)
	}
	if noMatch.Accepted || !ContainsString(noMatch.ReasonCodes, "atlassian_no_matching_binding") {
		t.Fatalf("no-match response = %#v", noMatch)
	}
}

func TestAtlassianServiceEvaluatorErrorBranches(t *testing.T) {
	binding := SiteBinding{
		BindingID:          "atb_1",
		TenantID:           "demo",
		SiteURL:            "https://acme.atlassian.net",
		MissionRef:         "mref_123",
		JiraProjectKeys:    []string{"FIN"},
		AllowedJiraActions: []string{JiraActionTransitionIssue},
		SubjectClaim:       "sub",
		GroupClaim:         "groups",
		GroupMatchMode:     GroupMatchAny,
		Status:             SiteBindingStatusActive,
	}
	req := AuthorizeJiraIssueActionRequest{
		TenantID: "demo",
		SiteURL:  "https://acme.atlassian.net",
		IssueKey: "FIN-1",
		Action:   JiraActionTransitionIssue,
		Subject:  "acc_123",
		Evaluation: &EvaluationRequest{
			Action: EvaluationAction{Resource: EvaluationActionResource{Type: "jira_issue", ID: "FIN-1"}, Operation: "transition"},
		},
	}
	if _, err := newAtlassianService(newAtlassianMemoryStore(binding), nil, nil).AuthorizeJiraIssueAction(req); err == nil || !strings.Contains(err.Error(), "evaluator") {
		t.Fatalf("expected missing evaluator error, got %v", err)
	}
	if _, err := newAtlassianService(newAtlassianMemoryStore(binding), &atlassianEvaluator{err: errors.New("eval failed")}, nil).AuthorizeJiraIssueAction(req); err == nil {
		t.Fatal("expected evaluator error")
	}
}

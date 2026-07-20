package github

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

type githubFixedClock struct {
	now time.Time
}

func (c githubFixedClock) Now() time.Time {
	return c.now
}

type githubMemoryStore struct {
	bindings   map[string]RepositoryBinding
	deliveries map[string]WebhookDelivery
	listErr    error
	saveErr    error
	updateErr  error
	conflict   error
}

func newGitHubMemoryStore(bindings ...RepositoryBinding) *githubMemoryStore {
	store := &githubMemoryStore{
		bindings:   map[string]RepositoryBinding{},
		deliveries: map[string]WebhookDelivery{},
		conflict:   errors.New("duplicate delivery"),
	}
	for _, binding := range bindings {
		store.bindings[binding.BindingID] = binding
	}
	return store
}

func (s *githubMemoryStore) SaveRepositoryBinding(binding RepositoryBinding) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *githubMemoryStore) GetRepositoryBinding(id string) (RepositoryBinding, error) {
	binding, ok := s.bindings[id]
	if !ok {
		return RepositoryBinding{}, errors.New("not found")
	}
	return binding, nil
}

func (s *githubMemoryStore) UpdateRepositoryBinding(binding RepositoryBinding) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *githubMemoryStore) ListRepositoryBindings() ([]RepositoryBinding, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	bindings := make([]RepositoryBinding, 0, len(s.bindings))
	for _, binding := range s.bindings {
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func (s *githubMemoryStore) SaveWebhookDelivery(delivery WebhookDelivery) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	if _, exists := s.deliveries[delivery.DeliveryID]; exists {
		return s.conflict
	}
	s.deliveries[delivery.DeliveryID] = delivery
	return nil
}

func (s *githubMemoryStore) GetWebhookDelivery(id string) (WebhookDelivery, error) {
	delivery, ok := s.deliveries[id]
	if !ok {
		return WebhookDelivery{}, errors.New("not found")
	}
	return delivery, nil
}

type githubEvaluator struct {
	requests  []EvaluationRequest
	responses []EvaluationResponse
	err       error
}

func (e *githubEvaluator) Evaluate(req EvaluationRequest) (EvaluationResponse, error) {
	e.requests = append(e.requests, req)
	if e.err != nil {
		return EvaluationResponse{}, e.err
	}
	if len(e.responses) == 0 {
		return EvaluationResponse{Decision: "allow", MissionRef: req.MissionRef, MissionVersion: 1}, nil
	}
	index := len(e.requests) - 1
	if index >= len(e.responses) {
		index = len(e.responses) - 1
	}
	return e.responses[index], nil
}

type githubEventSink struct {
	events []Event
}

func (s *githubEventSink) AppendEvent(event Event) error {
	s.events = append(s.events, event)
	return nil
}

func newGitHubService(store *githubMemoryStore, evaluator Evaluator, events *githubEventSink) *Service {
	return NewService(Config{
		Store:         store,
		Evaluator:     evaluator,
		Events:        events,
		Clock:         githubFixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)},
		WebhookSecret: []byte("secret"),
		NewID: func(prefix string) string {
			return prefix + "_test"
		},
		IsConflict: IsConflict(store.conflict),
	})
}

func TestGitHubServiceCreateRepositoryBindingDefaultsAndLists(t *testing.T) {
	store := newGitHubMemoryStore()
	events := &githubEventSink{}
	service := newGitHubService(store, nil, events)

	binding, err := service.CreateRepositoryBinding(CreateRepositoryBindingRequest{
		Repository: "https://github.com/Acme/Auth-Scope.git",
		MissionRef: "mref_123",
		RequiredChecks: []string{
			" unit ",
			"",
		},
		Metadata: map[string]string{"env": "demo"},
	}, Principal{Subject: "reviewer@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateRepositoryBinding: %v", err)
	}
	if binding.BindingID != "ghr_test" || binding.TenantID != "default" {
		t.Fatalf("unexpected identity defaults: %#v", binding)
	}
	if binding.Owner != "acme" || binding.Repo != "auth-scope" || binding.Repository != "acme/auth-scope" {
		t.Fatalf("repository was not normalized: %#v", binding)
	}
	if binding.DefaultBranch != "main" || !slices.Contains(binding.BranchPatterns, "refs/heads/main") {
		t.Fatalf("default branch patterns not populated: %#v", binding.BranchPatterns)
	}
	if len(binding.RequiredChecks) != 1 || binding.RequiredChecks[0] != "unit" {
		t.Fatalf("required checks were not cleaned: %#v", binding.RequiredChecks)
	}
	if len(events.events) != 1 || events.events[0].Type != "github.repository_bound" {
		t.Fatalf("repository bind event not emitted: %#v", events.events)
	}

	list, err := service.ListRepositoryBindings()
	if err != nil {
		t.Fatalf("ListRepositoryBindings: %v", err)
	}
	if len(list) != 1 || list[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected binding list: %#v", list)
	}
}

func TestGitHubServiceIngestWebhookAcceptsAndDetectsDuplicateDelivery(t *testing.T) {
	store := newGitHubMemoryStore(RepositoryBinding{
		BindingID:  "ghr_1",
		TenantID:   "demo",
		Repository: "acme/auth-scope",
		MissionRef: "mref_123",
		Status:     RepositoryBindingStatusActive,
	})
	events := &githubEventSink{}
	service := newGitHubService(store, nil, events)
	body := []byte(`{
		"action": "opened",
		"number": 42,
		"repository": {"full_name": "Acme/Auth-Scope"},
		"pull_request": {"number": 42, "head": {"sha": "abc123", "ref": "feature/slack"}}
	}`)

	resp, err := service.IngestWebhook(IngestWebhookRequest{
		Event:      "pull_request",
		DeliveryID: "delivery-1",
		Signature:  SignWebhookBody([]byte("secret"), body),
		Body:       body,
	})
	if err != nil {
		t.Fatalf("IngestWebhook: %v", err)
	}
	if !resp.Accepted || resp.Status != WebhookDeliveryStatusAccepted || resp.BindingID != "ghr_1" {
		t.Fatalf("unexpected webhook response: %#v", resp)
	}
	if store.bindings["ghr_1"].LastDeliveryID != "delivery-1" || store.bindings["ghr_1"].LastCheckSHA != "abc123" {
		t.Fatalf("binding was not updated from webhook: %#v", store.bindings["ghr_1"])
	}

	duplicate, err := service.IngestWebhook(IngestWebhookRequest{
		Event:      "pull_request",
		DeliveryID: "delivery-1",
		Signature:  SignWebhookBody([]byte("secret"), body),
		Body:       body,
	})
	if err != nil {
		t.Fatalf("duplicate IngestWebhook: %v", err)
	}
	if duplicate.Accepted || duplicate.Status != WebhookDeliveryStatusDuplicate || !strings.Contains(duplicate.Message, "already processed") {
		t.Fatalf("unexpected duplicate response: %#v", duplicate)
	}
	if len(events.events) != 1 || events.events[0].Type != "github.webhook_received" {
		t.Fatalf("expected one event for accepted delivery, got %#v", events.events)
	}
}

func TestGitHubServicePlanCheckRunCombinesEvaluationConclusions(t *testing.T) {
	store := newGitHubMemoryStore(RepositoryBinding{
		BindingID:  "ghr_1",
		TenantID:   "demo",
		Repository: "acme/auth-scope",
		MissionRef: "mref_123",
		Status:     RepositoryBindingStatusActive,
	})
	evaluator := &githubEvaluator{responses: []EvaluationResponse{
		{Decision: "allow", MissionRef: "mref_123", MissionVersion: 5},
		{Decision: "require_approval", MissionRef: "mref_123", MissionVersion: 5},
		{Decision: "deny", MissionRef: "mref_123", MissionVersion: 5, ReasonCodes: []string{"out_of_scope"}},
	}}
	events := &githubEventSink{}
	service := newGitHubService(store, evaluator, events)

	resp, err := service.PlanCheckRun(CheckRunPlanRequest{
		Repository:         "ACME/Auth-Scope",
		MissionRef:         "mref_123",
		MissionVersionSeen: 4,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "agent"},
		PullRequest:        42,
		HeadSHA:            "abc123",
		Branch:             "feature/slack",
		ChangedFiles: []ChangedFile{
			{Path: "/internal/mission/slack.go", Status: "modified", Additions: 5},
			{Path: "README.md", Operation: "document"},
			{Path: "secret.txt", Status: "deleted", Deletions: 1},
		},
		Context: map[string]any{"risk": "medium"},
	})
	if err != nil {
		t.Fatalf("PlanCheckRun: %v", err)
	}
	if resp.Conclusion != CheckConclusionFailure || resp.Title != "Mission authority blocked this change" {
		t.Fatalf("unexpected check conclusion: %#v", resp)
	}
	if resp.ExternalID != "mref_123:abc123" || resp.MissionVersion != 5 || len(resp.Evaluations) != 3 {
		t.Fatalf("unexpected check response shape: %#v", resp)
	}
	if evaluator.requests[0].Action.Resource.ID != "acme/auth-scope:internal/mission/slack.go" {
		t.Fatalf("unexpected resource id: %#v", evaluator.requests[0].Action)
	}
	if evaluator.requests[1].Action.Operation != "document" || evaluator.requests[2].Action.Operation != "delete" {
		t.Fatalf("unexpected operations: %#v", evaluator.requests)
	}
	if evaluator.requests[0].Context["github.pull_request"] != 42 || evaluator.requests[0].Context["risk"] != "medium" {
		t.Fatalf("context missing GitHub facts: %#v", evaluator.requests[0].Context)
	}
	if store.bindings["ghr_1"].LastCheckStatus != CheckConclusionFailure || store.bindings["ghr_1"].LastCheckSHA != "abc123" {
		t.Fatalf("binding was not updated from check plan: %#v", store.bindings["ghr_1"])
	}
	if len(events.events) != 1 || events.events[0].Type != "github.check_run_planned" {
		t.Fatalf("check-run event not emitted: %#v", events.events)
	}
}

func TestGitHubHelpersNormalizeSummarizeAndValidateSignatures(t *testing.T) {
	body := []byte(`{"ok": true}`)
	signature := SignWebhookBody([]byte("secret"), body)
	if !ValidateWebhookSignature([]byte("secret"), signature, body) {
		t.Fatal("expected valid signature")
	}
	if ValidateWebhookSignature([]byte("secret"), "sha256=bad", body) {
		t.Fatal("expected invalid signature to fail")
	}
	if ValidateWebhookSignature(nil, signature, body) {
		t.Fatal("expected empty secret to fail")
	}

	owner, repo, repository, err := NormalizeRepository("", "", "https://github.com/Acme/Auth-Scope.git")
	if err != nil {
		t.Fatalf("NormalizeRepository: %v", err)
	}
	if owner != "acme" || repo != "auth-scope" || repository != "acme/auth-scope" {
		t.Fatalf("unexpected normalized repository: %q %q %q", owner, repo, repository)
	}
	if _, _, _, err := NormalizeRepository("", "", "missing-owner"); err == nil {
		t.Fatal("expected invalid repository error")
	}

	summary := SummarizeWebhook("pull_request", map[string]any{
		"action": "synchronize",
		"number": float64(42),
		"repository": map[string]any{
			"owner": map[string]any{"login": "Acme"},
			"name":  "Auth-Scope",
		},
		"pull_request": map[string]any{
			"head": map[string]any{"sha": "abc123", "ref": "feature/slack"},
		},
	})
	if summary.Repository != "acme/auth-scope" || summary.PullRequest != 42 || summary.SHA != "abc123" || summary.Branch != "feature/slack" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if BranchFromRef("refs/heads/main") != "main" {
		t.Fatal("expected branch name without refs prefix")
	}
	if CheckText([]CheckEvaluation{{Decision: "allow"}, {Decision: "deny"}, {Decision: "deny"}}) != "1 allow, 2 deny" {
		t.Fatal("unexpected check text")
	}
}

func TestGitHubServiceValidationAndWebhookErrorBranches(t *testing.T) {
	if NewService(Config{}).isConflict(errors.New("anything")) {
		t.Fatal("nil conflict classifier should fail closed")
	}
	if _, err := newGitHubService(newGitHubMemoryStore(), nil, nil).CreateRepositoryBinding(CreateRepositoryBindingRequest{Repository: "missing-owner"}, Principal{}); err == nil {
		t.Fatal("expected invalid repository error")
	}
	saveErrStore := newGitHubMemoryStore()
	saveErrStore.saveErr = errors.New("save failed")
	if _, err := newGitHubService(saveErrStore, nil, nil).CreateRepositoryBinding(CreateRepositoryBindingRequest{Repository: "acme/auth-scope"}, Principal{}); err == nil {
		t.Fatal("expected repository save error")
	}

	body := []byte(`{"repository":{"full_name":"acme/auth-scope"}}`)
	signature := SignWebhookBody([]byte("secret"), body)
	service := newGitHubService(newGitHubMemoryStore(), nil, &githubEventSink{})
	for _, test := range []struct {
		name string
		req  IngestWebhookRequest
	}{
		{name: "missing event", req: IngestWebhookRequest{DeliveryID: "d1", Signature: signature, Body: body}},
		{name: "missing delivery", req: IngestWebhookRequest{Event: "push", Signature: signature, Body: body}},
		{name: "invalid signature", req: IngestWebhookRequest{Event: "push", DeliveryID: "d1", Signature: "sha256=bad", Body: body}},
		{name: "bad json", req: IngestWebhookRequest{Event: "push", DeliveryID: "d1", Signature: SignWebhookBody([]byte("secret"), []byte("{")), Body: []byte("{")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.IngestWebhook(test.req); err == nil {
				t.Fatal("expected webhook error")
			}
		})
	}

	noSecret := NewService(Config{Store: newGitHubMemoryStore()})
	if _, err := noSecret.IngestWebhook(IngestWebhookRequest{Event: "push", DeliveryID: "d1", Signature: signature, Body: body}); err == nil {
		t.Fatal("expected missing webhook secret error")
	}

	listErrStore := newGitHubMemoryStore()
	listErrStore.listErr = errors.New("list failed")
	if _, err := newGitHubService(listErrStore, nil, nil).IngestWebhook(IngestWebhookRequest{Event: "push", DeliveryID: "d1", Signature: signature, Body: body}); err == nil {
		t.Fatal("expected binding lookup list error")
	}

	ignored, err := service.IngestWebhook(IngestWebhookRequest{Event: "push", DeliveryID: "ignored-1", Signature: signature, Body: body})
	if err != nil {
		t.Fatalf("IngestWebhook ignored: %v", err)
	}
	if ignored.Accepted || ignored.Status != WebhookDeliveryStatusIgnored {
		t.Fatalf("ignored webhook = %#v, want ignored status", ignored)
	}

	deliveryErrStore := newGitHubMemoryStore()
	deliveryErrStore.saveErr = errors.New("delivery save failed")
	if _, err := newGitHubService(deliveryErrStore, nil, nil).IngestWebhook(IngestWebhookRequest{Event: "push", DeliveryID: "d2", Signature: signature, Body: body}); err == nil {
		t.Fatal("expected delivery save error")
	}
}

func TestGitHubServiceCheckRunErrorAndConclusionBranches(t *testing.T) {
	active := RepositoryBinding{BindingID: "ghr_1", TenantID: "demo", Repository: "acme/auth-scope", MissionRef: "mref_123", Status: RepositoryBindingStatusActive}
	for _, test := range []struct {
		name    string
		store   *githubMemoryStore
		eval    Evaluator
		request CheckRunPlanRequest
	}{
		{name: "bad repository", store: newGitHubMemoryStore(active), request: CheckRunPlanRequest{Repository: "missing-owner", HeadSHA: "abc", ChangedFiles: []ChangedFile{{Path: "README.md"}}}},
		{name: "missing head", store: newGitHubMemoryStore(active), request: CheckRunPlanRequest{Repository: "acme/auth-scope", ChangedFiles: []ChangedFile{{Path: "README.md"}}}},
		{name: "missing files", store: newGitHubMemoryStore(active), request: CheckRunPlanRequest{Repository: "acme/auth-scope", HeadSHA: "abc"}},
		{name: "list error", store: func() *githubMemoryStore {
			store := newGitHubMemoryStore(active)
			store.listErr = errors.New("list failed")
			return store
		}(), request: CheckRunPlanRequest{Repository: "acme/auth-scope", HeadSHA: "abc", ChangedFiles: []ChangedFile{{Path: "README.md"}}}},
		{name: "not bound", store: newGitHubMemoryStore(), request: CheckRunPlanRequest{Repository: "acme/auth-scope", HeadSHA: "abc", ChangedFiles: []ChangedFile{{Path: "README.md"}}}},
		{name: "missing mission ref", store: newGitHubMemoryStore(RepositoryBinding{BindingID: "ghr_1", Repository: "acme/auth-scope", Status: RepositoryBindingStatusActive}), request: CheckRunPlanRequest{Repository: "acme/auth-scope", HeadSHA: "abc", ChangedFiles: []ChangedFile{{Path: "README.md"}}}},
		{name: "mission mismatch", store: newGitHubMemoryStore(active), request: CheckRunPlanRequest{Repository: "acme/auth-scope", MissionRef: "other", HeadSHA: "abc", ChangedFiles: []ChangedFile{{Path: "README.md"}}}},
		{name: "empty path", store: newGitHubMemoryStore(active), eval: &githubEvaluator{}, request: CheckRunPlanRequest{Repository: "acme/auth-scope", HeadSHA: "abc", ChangedFiles: []ChangedFile{{Path: " / "}}}},
		{name: "evaluator error", store: newGitHubMemoryStore(active), eval: &githubEvaluator{err: errors.New("eval failed")}, request: CheckRunPlanRequest{Repository: "acme/auth-scope", HeadSHA: "abc", ChangedFiles: []ChangedFile{{Path: "README.md"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			eval := test.eval
			if eval == nil {
				eval = &githubEvaluator{}
			}
			if _, err := newGitHubService(test.store, eval, nil).PlanCheckRun(test.request); err == nil {
				t.Fatal("expected check-run planning error")
			}
		})
	}

	for _, test := range []struct {
		name       string
		decision   string
		conclusion string
	}{
		{name: "approval", decision: "require_approval", conclusion: CheckConclusionActionRequired},
		{name: "refresh", decision: "require_refresh", conclusion: CheckConclusionNeutral},
	} {
		t.Run(test.name, func(t *testing.T) {
			resp, err := newGitHubService(newGitHubMemoryStore(active), &githubEvaluator{responses: []EvaluationResponse{{Decision: test.decision, MissionVersion: 7}}}, &githubEventSink{}).PlanCheckRun(CheckRunPlanRequest{
				Repository:   "acme/auth-scope",
				HeadSHA:      "abc",
				ChangedFiles: []ChangedFile{{Path: "README.md"}},
			})
			if err != nil {
				t.Fatalf("PlanCheckRun: %v", err)
			}
			if resp.Conclusion != test.conclusion {
				t.Fatalf("conclusion = %s, want %s", resp.Conclusion, test.conclusion)
			}
		})
	}
}

package mission

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIGitHubIntegrationLifecycle(t *testing.T) {
	service := testService()
	service.githubWebhookSecret = []byte("github-webhook-secret")
	mission := approveGitHubMission(t, service)
	router := withTestAdminAuthorization(NewHandler(service).Routes())

	createBinding := httptest.NewRecorder()
	router.ServeHTTP(createBinding, jsonRequest(http.MethodPost, "/v1/integrations/github/repositories", CreateGitHubRepositoryBindingRequest{
		Repository:    "tauliang/auth-scope",
		MissionRef:    mission.MissionRef,
		DefaultBranch: "main",
		RequiredChecks: []string{
			"Auth Scope Mission Authority",
		},
	}))
	if createBinding.Code != http.StatusCreated {
		t.Fatalf("create binding status = %d body=%s", createBinding.Code, createBinding.Body.String())
	}
	var binding GitHubRepositoryBinding
	decodeTestJSON(t, createBinding.Body.Bytes(), &binding)
	if binding.Repository != "tauliang/auth-scope" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected binding: %#v", binding)
	}

	listBindings := httptest.NewRecorder()
	router.ServeHTTP(listBindings, httptest.NewRequest(http.MethodGet, "/v1/integrations/github/repositories", nil))
	if listBindings.Code != http.StatusOK {
		t.Fatalf("list bindings status = %d body=%s", listBindings.Code, listBindings.Body.String())
	}

	planCheck := httptest.NewRecorder()
	router.ServeHTTP(planCheck, jsonRequest(http.MethodPost, "/v1/integrations/github/check-runs/plan", GitHubCheckRunPlanRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              GitHubActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Repository:         "tauliang/auth-scope",
		HeadSHA:            "abc123",
		Branch:             "agent/fix-filter",
		ChangedFiles:       []GitHubChangedFile{{Path: "frontend/src/features/missions/MissionDetailPage.tsx", Status: "modified"}},
		Context:            map[string]any{"risk": "low", "reversible": true},
	}))
	if planCheck.Code != http.StatusOK {
		t.Fatalf("plan check status = %d body=%s", planCheck.Code, planCheck.Body.String())
	}
	var plan GitHubCheckRunPlanResponse
	decodeTestJSON(t, planCheck.Body.Bytes(), &plan)
	if plan.Conclusion != GitHubCheckConclusionSuccess || len(plan.Evaluations) != 1 {
		t.Fatalf("unexpected check plan: %#v", plan)
	}

	body := mustMarshalGitHub(t, map[string]any{
		"action": "synchronize",
		"repository": map[string]any{
			"full_name": "tauliang/auth-scope",
		},
		"pull_request": map[string]any{
			"number": float64(42),
			"head": map[string]any{
				"sha": "abc123",
				"ref": "agent/fix-filter",
			},
		},
	})
	invalidWebhook := httptest.NewRecorder()
	invalidReq := httptest.NewRequest(http.MethodPost, "/v1/integrations/github/webhooks", bytes.NewReader(body))
	invalidReq.Header.Set("X-GitHub-Event", "pull_request")
	invalidReq.Header.Set("X-GitHub-Delivery", "delivery-invalid")
	invalidReq.Header.Set("X-Hub-Signature-256", "sha256=bad")
	router.ServeHTTP(invalidWebhook, invalidReq)
	if invalidWebhook.Code != http.StatusUnauthorized {
		t.Fatalf("invalid webhook status = %d body=%s", invalidWebhook.Code, invalidWebhook.Body.String())
	}

	webhook := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/integrations/github/webhooks", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-1")
	req.Header.Set("X-Hub-Signature-256", SignGitHubWebhookBody(service.githubWebhookSecret, body))
	router.ServeHTTP(webhook, req)
	if webhook.Code != http.StatusAccepted {
		t.Fatalf("webhook status = %d body=%s", webhook.Code, webhook.Body.String())
	}
	var webhookResp GitHubWebhookResponse
	decodeTestJSON(t, webhook.Body.Bytes(), &webhookResp)
	if !webhookResp.Accepted || webhookResp.BindingID != binding.BindingID || webhookResp.SHA != "abc123" {
		t.Fatalf("unexpected webhook response: %#v", webhookResp)
	}
}

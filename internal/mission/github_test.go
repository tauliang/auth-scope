package mission

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestGitHubWebhookSignatureValidation(t *testing.T) {
	secret := []byte("github-webhook-secret")
	body := []byte(`{"repository":{"full_name":"tauliang/auth-scope"}}`)
	signature := SignGitHubWebhookBody(secret, body)

	if !ValidateGitHubWebhookSignature(secret, signature, body) {
		t.Fatal("expected valid GitHub webhook signature")
	}
	if ValidateGitHubWebhookSignature(secret, signature, []byte(`{"tampered":true}`)) {
		t.Fatal("expected tampered body to fail signature validation")
	}
	if ValidateGitHubWebhookSignature(secret, "sha1=bad", body) {
		t.Fatal("expected unsupported signature algorithm to fail")
	}
}

func TestGitHubRepositoryBindingWebhookAndDuplicate(t *testing.T) {
	service := testService()
	service.githubWebhookSecret = []byte("github-webhook-secret")
	mission := approveGitHubMission(t, service)

	binding, err := service.CreateGitHubRepositoryBinding(CreateGitHubRepositoryBindingRequest{
		Repository: "Tauliang/Auth-Scope",
		MissionRef: mission.MissionRef,
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateGitHubRepositoryBinding: %v", err)
	}
	if binding.Repository != "tauliang/auth-scope" || binding.DefaultBranch != "main" {
		t.Fatalf("unexpected binding: %#v", binding)
	}
	if _, err := service.CreateGitHubRepositoryBinding(CreateGitHubRepositoryBindingRequest{
		Repository: "tauliang/auth-scope",
	}, Principal{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate binding err = %v, want ErrConflict", err)
	}

	body := mustMarshalGitHub(t, map[string]any{
		"action": "opened",
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
	resp, err := service.IngestGitHubWebhook(IngestGitHubWebhookRequest{
		Event:      "pull_request",
		DeliveryID: "delivery-1",
		Signature:  SignGitHubWebhookBody(service.githubWebhookSecret, body),
		Body:       body,
	})
	if err != nil {
		t.Fatalf("IngestGitHubWebhook: %v", err)
	}
	if !resp.Accepted || resp.BindingID != binding.BindingID || resp.MissionRef != mission.MissionRef || resp.SHA != "abc123" {
		t.Fatalf("unexpected webhook response: %#v", resp)
	}
	duplicate, err := service.IngestGitHubWebhook(IngestGitHubWebhookRequest{
		Event:      "pull_request",
		DeliveryID: "delivery-1",
		Signature:  SignGitHubWebhookBody(service.githubWebhookSecret, body),
		Body:       body,
	})
	if err != nil {
		t.Fatalf("duplicate IngestGitHubWebhook: %v", err)
	}
	if duplicate.Status != GitHubWebhookDeliveryStatusDuplicate || duplicate.Accepted {
		t.Fatalf("unexpected duplicate response: %#v", duplicate)
	}
	if _, err := service.IngestGitHubWebhook(IngestGitHubWebhookRequest{
		Event:      "pull_request",
		DeliveryID: "delivery-2",
		Signature:  "sha256=bad",
		Body:       body,
	}); err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestGitHubCheckRunPlanEvaluatesChangedFiles(t *testing.T) {
	service := testService()
	mission := approveGitHubMission(t, service)
	binding, err := service.CreateGitHubRepositoryBinding(CreateGitHubRepositoryBindingRequest{
		Repository: "tauliang/auth-scope",
		MissionRef: mission.MissionRef,
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateGitHubRepositoryBinding: %v", err)
	}

	plan, err := service.PlanGitHubCheckRun(GitHubCheckRunPlanRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              GitHubActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Repository:         "https://github.com/tauliang/auth-scope.git",
		PullRequest:        42,
		HeadSHA:            "abc123",
		Branch:             "agent/fix-filter",
		ChangedFiles: []GitHubChangedFile{
			{Path: "frontend/src/features/missions/MissionDetailPage.tsx", Status: "modified"},
			{Path: "infra/prod/secrets.tf", Status: "modified"},
		},
		Context: map[string]any{"risk": "low", "reversible": true},
	})
	if err != nil {
		t.Fatalf("PlanGitHubCheckRun: %v", err)
	}
	if plan.BindingID != binding.BindingID || plan.MissionRef != mission.MissionRef || plan.HeadSHA != "abc123" {
		t.Fatalf("unexpected plan identity: %#v", plan)
	}
	if plan.Conclusion != GitHubCheckConclusionActionRequired {
		t.Fatalf("plan conclusion = %s, want %s: %#v", plan.Conclusion, GitHubCheckConclusionActionRequired, plan)
	}
	if len(plan.Evaluations) != 2 {
		t.Fatalf("evaluations len = %d, want 2", len(plan.Evaluations))
	}
	if plan.Evaluations[0].Decision != string(DecisionAllow) || plan.Evaluations[0].DecisionArtifact == "" {
		t.Fatalf("first evaluation = %#v, want allow with artifact", plan.Evaluations[0])
	}
	if plan.Evaluations[1].Decision != string(DecisionRequireExpansion) {
		t.Fatalf("second evaluation = %#v, want require_expansion", plan.Evaluations[1])
	}
	updated, err := service.github.GetGitHubRepositoryBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetGitHubRepositoryBinding: %v", err)
	}
	if updated.LastCheckSHA != "abc123" || updated.LastCheckStatus != GitHubCheckConclusionActionRequired {
		t.Fatalf("binding check fields = %#v", updated)
	}
}

func approveGitHubMission(t *testing.T, service *Service) ApproveProposalResponse {
	t.Helper()
	req := validProposalRequest()
	req.Intent = Purpose{Objective: "Govern coding agent pull request"}
	req.Conditions = nil
	req.AuthorityRegion = AuthorityRegion{
		Resources: []ResourceGrant{
			{Type: "repo_path", ID: "tauliang/auth-scope:frontend/**", Actions: []string{"edit"}},
			{Type: "repo_path", ID: "tauliang/auth-scope:README.md", Actions: []string{"edit"}},
		},
		ForbiddenActions: []string{"delete", "deploy_production"},
	}
	proposal, err := service.CreateProposal(req)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	mission, err := service.ApproveProposal(proposal.ProposalID, ApproveProposalRequest{
		Approver: Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}
	return mission
}

func mustMarshalGitHub(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return data
}

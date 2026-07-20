package mission

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAtlassianMissionAdapterJiraAndConfluenceAuthorization(t *testing.T) {
	service := testService()
	mission := approveAtlassianMission(t, service)

	binding, err := service.CreateAtlassianSiteBinding(CreateAtlassianSiteBindingRequest{
		TenantID:            "demo",
		SiteURL:             "https://acme.atlassian.net/",
		CloudID:             "cloud_123",
		SiteName:            "Acme",
		MissionRef:          mission.MissionRef,
		JiraProjectKeys:     []string{"FIN"},
		ConfluenceSpaceKeys: []string{"ENG"},
		AllowedJiraActions:  []string{AtlassianJiraActionTransitionIssue},
		AllowedPageActions:  []string{AtlassianConfluenceActionUpdatePage},
		RequiredGroups:      []string{"Mission Operators"},
		AdminGroups:         []string{"Mission Admins"},
		AllowedSubjects:     []string{"acc_123"},
	}, Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateAtlassianSiteBinding: %v", err)
	}
	if binding.SiteURL != "https://acme.atlassian.net" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected Atlassian binding: %#v", binding)
	}

	jira, err := service.AuthorizeAtlassianJiraIssueAction(AuthorizeAtlassianJiraIssueActionRequest{
		TenantID:   "demo",
		MissionRef: mission.MissionRef,
		SiteURL:    "https://acme.atlassian.net",
		IssueKey:   "FIN-77",
		Action:     AtlassianJiraActionTransitionIssue,
		AccountID:  "acc_123",
		Subject:    "acc_123",
		Groups:     []string{"Mission Operators", "Mission Admins"},
		Context:    map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &AtlassianEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              AtlassianActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: AtlassianEvaluationAction{
				Type:      "jira_issue",
				Resource:  AtlassianEvaluationActionResource{Type: "jira_issue", ID: "FIN-77"},
				Operation: "transition",
			},
		},
	})
	if err != nil {
		t.Fatalf("AuthorizeAtlassianJiraIssueAction: %v", err)
	}
	if !jira.Accepted || jira.Evaluation == nil || jira.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("jira authorization = %#v, want accepted allow", jira)
	}
	if jira.Context["atlassian.binding_id"] != binding.BindingID || jira.Context["jira.project_key"] != "FIN" {
		t.Fatalf("jira context = %#v, want binding and project context", jira.Context)
	}

	confluence, err := service.AuthorizeAtlassianConfluencePageAction(AuthorizeAtlassianConfluencePageActionRequest{
		TenantID:   "demo",
		MissionRef: mission.MissionRef,
		SiteURL:    "https://acme.atlassian.net",
		SpaceKey:   "ENG",
		PageID:     "12345",
		PageTitle:  "Runbook",
		Action:     AtlassianConfluenceActionUpdatePage,
		AccountID:  "acc_123",
		Subject:    "acc_123",
		Groups:     []string{"Mission Operators"},
		Context:    map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &AtlassianEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              AtlassianActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: AtlassianEvaluationAction{
				Type:      "confluence_page",
				Resource:  AtlassianEvaluationActionResource{Type: "confluence_page", ID: "ENG:12345"},
				Operation: "update",
			},
		},
	})
	if err != nil {
		t.Fatalf("AuthorizeAtlassianConfluencePageAction: %v", err)
	}
	if !confluence.Accepted || confluence.Evaluation == nil || confluence.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("confluence authorization = %#v, want accepted allow", confluence)
	}

	denied, err := service.AuthorizeAtlassianConfluencePageAction(AuthorizeAtlassianConfluencePageActionRequest{
		TenantID:   "demo",
		MissionRef: mission.MissionRef,
		SiteURL:    "https://acme.atlassian.net",
		SpaceKey:   "HR",
		PageID:     "999",
		Action:     AtlassianConfluenceActionUpdatePage,
		AccountID:  "acc_123",
		Subject:    "acc_123",
		Groups:     []string{"Mission Operators"},
	})
	if err != nil {
		t.Fatalf("AuthorizeAtlassianConfluencePageAction denied: %v", err)
	}
	if denied.Accepted || !contains(denied.ReasonCodes, "confluence_space_not_allowed") {
		t.Fatalf("denied confluence authorization = %#v, want space denial", denied)
	}
}

func TestAtlassianMissionAdapterConversions(t *testing.T) {
	principal := atlassianPrincipal(Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"})
	if principal.Subject != "alice@example.com" || principal.Issuer != "https://idp.example.com" {
		t.Fatalf("unexpected principal conversion: %#v", principal)
	}

	actor := missionActorFromAtlassian(AtlassianActor{AgentInstanceID: "inst_123", ClientID: "agent", KeyThumbprint: "thumb"})
	if actor.AgentInstanceID != "inst_123" || actor.ClientID != "agent" || actor.KeyThumbprint != "thumb" {
		t.Fatalf("unexpected actor conversion: %#v", actor)
	}
}

func TestSignedAtlassianJiraAPIBindsAgentIdentity(t *testing.T) {
	service := testService()
	publicKey, privateKey := testAgentKeypair(t)
	registered, err := service.RegisterAgent(RegisterAgentRequest{
		TenantID:  "demo",
		Agent:     Agent{Provider: "https://agents.example.com", ClientID: "research-agent", InstanceID: "inst_123"},
		PublicKey: publicKey,
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	mission := approveAtlassianMission(t, service)
	if _, err := service.CreateAtlassianSiteBinding(CreateAtlassianSiteBindingRequest{
		TenantID:           "demo",
		SiteURL:            "https://acme.atlassian.net",
		MissionRef:         mission.MissionRef,
		JiraProjectKeys:    []string{"FIN"},
		AllowedJiraActions: []string{AtlassianJiraActionTransitionIssue},
		RequiredGroups:     []string{"Mission Operators"},
		AllowedSubjects:    []string{"acc_123"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"}); err != nil {
		t.Fatalf("CreateAtlassianSiteBinding: %v", err)
	}

	body := AuthorizeAtlassianJiraIssueActionRequest{
		TenantID:   "demo",
		MissionRef: mission.MissionRef,
		SiteURL:    "https://acme.atlassian.net",
		IssueKey:   "FIN-77",
		Action:     AtlassianJiraActionTransitionIssue,
		AccountID:  "acc_123",
		Subject:    "acc_123",
		Groups:     []string{"Mission Operators"},
		Context:    map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &AtlassianEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Action: AtlassianEvaluationAction{
				Type:      "jira_issue",
				Resource:  AtlassianEvaluationActionResource{Type: "jira_issue", ID: "FIN-77"},
				Operation: "transition",
			},
		},
	}
	router := NewHandler(service).Routes()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedJSONRequest(t, http.MethodPost, "/v1/integrations/atlassian/jira/issues/authorize", body, registered.AgentID, privateKey, "nonce-atlassian-jira"))
	if rec.Code != http.StatusOK {
		t.Fatalf("signed Atlassian authorize status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp AtlassianActionAuthorizationResponse
	decodeTestJSON(t, rec.Body.Bytes(), &resp)
	if !resp.Accepted || resp.Evaluation == nil || resp.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("signed Atlassian response = %#v, want accepted allow with evaluation", resp)
	}
}

func approveAtlassianMission(t *testing.T, service *Service) ApproveProposalResponse {
	t.Helper()
	req := validProposalRequest()
	req.Intent = Purpose{Objective: "Govern Atlassian Jira and Confluence work"}
	req.AuthorityRegion = AuthorityRegion{
		Resources: []ResourceGrant{
			{Type: "jira_issue", ID: "FIN-77", Actions: []string{"transition"}},
			{Type: "confluence_page", ID: "ENG:12345", Actions: []string{"update"}},
		},
		ForbiddenActions: []string{"delete"},
	}
	req.Conditions = nil
	proposal, err := service.CreateProposal(req)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	mission, err := service.ApproveProposal(proposal.ProposalID, ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}
	return mission
}

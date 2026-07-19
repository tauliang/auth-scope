package mission

import "testing"

func TestAtlassianHTTPIntegrationFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()
	mission := approveAtlassianMission(t, service)

	binding := postJSON[AtlassianSiteBinding](t, router, "/v1/integrations/atlassian/site-bindings", CreateAtlassianSiteBindingRequest{
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
	})
	if binding.SiteURL != "https://acme.atlassian.net" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected Atlassian binding: %#v", binding)
	}

	listed := getJSON[struct {
		SiteBindings []AtlassianSiteBinding `json:"site_bindings"`
	}](t, router, "/v1/integrations/atlassian/site-bindings")
	if len(listed.SiteBindings) != 1 || listed.SiteBindings[0].BindingID != binding.BindingID {
		t.Fatalf("listed Atlassian bindings = %#v, want binding %s", listed.SiteBindings, binding.BindingID)
	}

	jira := postJSON[AtlassianActionAuthorizationResponse](t, router, "/v1/integrations/atlassian/jira/issues/authorize", AuthorizeAtlassianJiraIssueActionRequest{
		SiteURL:   "https://acme.atlassian.net",
		IssueKey:  "FIN-77",
		Action:    AtlassianJiraActionTransitionIssue,
		AccountID: "acc_123",
		Groups:    []string{"Mission Operators", "Mission Admins"},
		Context:   map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
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
	if !jira.Accepted || jira.Product != "jira" || jira.Evaluation == nil || jira.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("jira authorization = %#v, want accepted allow", jira)
	}

	confluenceDenied := postJSON[AtlassianActionAuthorizationResponse](t, router, "/v1/integrations/atlassian/confluence/pages/authorize", AuthorizeAtlassianConfluencePageActionRequest{
		SiteURL:   "https://acme.atlassian.net",
		SpaceKey:  "HR",
		PageID:    "999",
		Action:    AtlassianConfluenceActionUpdatePage,
		AccountID: "acc_123",
		Groups:    []string{"Mission Operators"},
	})
	if confluenceDenied.Accepted || !contains(confluenceDenied.ReasonCodes, "confluence_space_not_allowed") {
		t.Fatalf("confluence denied = %#v, want space denial", confluenceDenied)
	}
}

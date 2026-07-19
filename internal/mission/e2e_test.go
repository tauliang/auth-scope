package mission

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestE2EMissionAuthorityFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()

	proposal := postJSON[CreateProposalResponse](t, router, "/v1/mission-proposals", validProposalRequest())
	mission := postJSON[ApproveProposalResponse](t, router, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{
			DisplayHash: "sha256:e2e",
			Method:      "e2e-test",
		},
	})

	allowed := postJSON[EvaluateResponse](t, router, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "write_draft"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
	})
	if allowed.Decision != DecisionAllow {
		t.Fatalf("allowed decision = %s, want %s", allowed.Decision, DecisionAllow)
	}

	child := postJSON[DelegationResponse](t, router, "/v1/missions/"+mission.MissionRef+"/delegate", DelegationRequest{
		DelegatingActor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TargetAgent:     Agent{Provider: "https://agents.example.com", ClientID: "chart-agent", InstanceID: "inst_child"},
		RequestedAuthority: AuthorityRegion{
			Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
			ForbiddenActions: []string{"send_external"},
		},
		Delegation: DelegationPolicy{Permitted: false, CascadeRevocation: true},
	})
	if child.ParentMissionRef != mission.MissionRef {
		t.Fatalf("child parent = %q, want %q", child.ParentMissionRef, mission.MissionRef)
	}

	childAllowed := postJSON[EvaluateResponse](t, router, "/v1/missions/"+child.ChildMissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: 1,
		Actor:              Actor{AgentInstanceID: "inst_child", ClientID: "chart-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if childAllowed.Decision != DecisionAllow {
		t.Fatalf("child decision = %s, want %s", childAllowed.Decision, DecisionAllow)
	}

	_ = postJSON[Mission](t, router, "/v1/missions/"+mission.MissionRef+"/revoke", StateChangeRequest{Reason: "principal revoked"})

	denied := postJSON[EvaluateResponse](t, router, "/v1/missions/"+child.ChildMissionRef+"/evaluate", EvaluateRequest{
		Actor:   Actor{AgentInstanceID: "inst_child", ClientID: "chart-agent"},
		Action:  Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context: map[string]any{"finance.close.status": "open"},
	})
	if denied.Decision != DecisionDeny {
		t.Fatalf("child after parent revoke decision = %s, want %s", denied.Decision, DecisionDeny)
	}
}

func TestE2EGovernanceExpansionAndToolGatewayFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()

	proposal := postJSON[CreateProposalResponse](t, router, "/v1/mission-proposals", validProposalRequest())
	mission := postJSON[ApproveProposalResponse](t, router, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "e2e-test"},
	})

	outOfScope := postJSON[EvaluateResponse](t, router, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "high", "reversible": false},
	})
	if outOfScope.Decision != DecisionRequireApproval {
		t.Fatalf("out of scope decision = %s, want %s", outOfScope.Decision, DecisionRequireApproval)
	}
	expansionID, _ := outOfScope.Constraints["expansion_request_id"].(string)
	if expansionID == "" {
		t.Fatalf("expected expansion id in decision: %#v", outOfScope)
	}
	verified := postJSON[VerifyDecisionArtifactResponse](t, router, "/v1/decision-artifacts/verify", VerifyDecisionArtifactRequest{
		DecisionArtifact: outOfScope.DecisionArtifact,
	})
	if !verified.Valid || verified.Evidence == nil {
		t.Fatalf("verify response = %#v, want valid with evidence", verified)
	}

	approved := postJSON[ExpansionDecisionResponse](t, router, "/v1/expansion-requests/"+expansionID+"/approve", ExpansionDecisionRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "e2e-test"},
	})
	if approved.Status != ExpansionStatusApproved {
		t.Fatalf("expansion status = %s, want approved", approved.Status)
	}

	_ = postJSON[ToolContract](t, router, "/v1/tool-contracts", ToolContract{
		ToolName:        "slack.post",
		ResourceType:    "slack_channel",
		ResourceIDParam: "channel_id",
		Operation:       "post_update",
		RequiredContext: []string{"finance.close.status"},
	})
	toolDecision := postJSON[AuthorizeToolCallResponse](t, router, "/v1/tool-calls/authorize", AuthorizeToolCallRequest{
		MissionRef:         mission.MissionRef,
		MissionVersionSeen: approved.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		ToolName:           "slack.post",
		Arguments:          map[string]any{"channel_id": "board"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if toolDecision.Decision != DecisionAllow || toolDecision.DecisionArtifact == "" {
		t.Fatalf("tool decision = %#v, want allow with artifact", toolDecision)
	}
}

func TestE2EAdvancedGovernanceFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()

	proposal := postJSON[CreateProposalResponse](t, router, "/v1/mission-proposals", validProposalRequest())
	mission := postJSON[ApproveProposalResponse](t, router, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "e2e-test"},
	})

	projection := postJSON[ProjectionResponse](t, router, "/v1/missions/"+mission.MissionRef+"/projections", CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Type:               ProjectionTypeMCPContext,
		TTLSeconds:         60,
	})
	if projection.Token == "" {
		t.Fatal("expected signed projection token")
	}
	verified := postJSON[VerifyProjectionResponse](t, router, "/v1/projections/verify", VerifyProjectionRequest{Token: projection.Token})
	if !verified.Valid {
		t.Fatalf("expected valid projection, got %#v", verified)
	}

	lease := postJSON[LeaseResponse](t, router, "/v1/missions/"+mission.MissionRef+"/leases", CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TTLSeconds:         30,
	})
	if lease.Decision != DecisionAllow {
		t.Fatalf("lease decision = %s, want allow", lease.Decision)
	}

	_ = postJSON[ApprovalRule](t, router, "/v1/approval-rules", ApprovalRule{
		TenantID:          "demo",
		ResourceType:      "slack_channel",
		ResourceID:        "board",
		Operation:         "post_update",
		RiskLevel:         "high",
		RequiredApprovals: 2,
		AllowedIssuers:    []string{"https://idp.example.com"},
	})
	expansion := postJSON[EvaluateResponse](t, router, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "high", "reversible": false},
	})
	expansionID, _ := expansion.Constraints["expansion_request_id"].(string)
	if expansionID == "" {
		t.Fatalf("expected expansion id: %#v", expansion)
	}
	first := postJSON[SubmitExpansionApprovalResponse](t, router, "/v1/expansion-requests/"+expansionID+"/approvals", SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	})
	if first.Status != ExpansionStatusPending {
		t.Fatalf("first approval status = %s, want pending", first.Status)
	}
	second := postJSONAsAdmin[SubmitExpansionApprovalResponse](t, router, "/v1/expansion-requests/"+expansionID+"/approvals", SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "spoofed@example.com", Issuer: "https://evil.example.com"},
	}, defaultDevelopmentSecondAdminToken)
	if second.Status != ExpansionStatusApproved {
		t.Fatalf("second approval status = %s, want approved", second.Status)
	}

	staleLease := postJSON[LeaseResponse](t, router, "/v1/leases/"+lease.LeaseID+"/refresh", RefreshLeaseRequest{
		Actor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	})
	if staleLease.Decision != DecisionDeny {
		t.Fatalf("lease after mission expansion = %s, want deny", staleLease.Decision)
	}
}

func TestE2ESlackIntegrationFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()
	mission := approveSlackMission(t, service)

	binding := postJSON[SlackWorkspaceBinding](t, router, "/v1/integrations/slack/workspace-bindings", CreateSlackWorkspaceBindingRequest{
		WorkspaceID:     "T12345678",
		WorkspaceName:   "Acme Corp",
		WorkspaceURL:    "https://acme-corp.slack.com",
		MissionRef:      mission.MissionRef,
		RequiredRoles:   []string{"Workspace Admin"},
		AllowedChannels: []string{"C11111111"},
		AllowedActions:  []string{SlackActionTypePostMessage},
	})
	if binding.WorkspaceID != "T12345678" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected Slack binding: %#v", binding)
	}

	type slackBindingList struct {
		WorkspaceBindings []SlackWorkspaceBinding `json:"workspace_bindings"`
	}
	list := getJSON[slackBindingList](t, router, "/v1/integrations/slack/workspace-bindings")
	if len(list.WorkspaceBindings) != 1 || list.WorkspaceBindings[0].BindingID != binding.BindingID {
		t.Fatalf("Slack binding list = %#v", list)
	}

	authorized := postJSON[SlackMessageAuthorizationResponse](t, router, "/v1/integrations/slack/message-actions/authorize", AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
		Evaluation: &SlackEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              SlackActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: SlackMessageAction{
				Type:      "message_event",
				Resource:  SlackActionResource{Type: "message", ID: "msg_123", ChannelID: "C11111111"},
				Operation: "post",
			},
		},
	})
	if !authorized.Accepted || authorized.Evaluation == nil || authorized.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("Slack authorization = %#v, want accepted allow", authorized)
	}

	denied := postJSON[SlackMessageAuthorizationResponse](t, router, "/v1/integrations/slack/message-actions/authorize", AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		Action:      SlackActionTypePostMessage,
	})
	if denied.Accepted || !contains(denied.ReasonCodes, "slack_channel_not_allowed") {
		t.Fatalf("Slack missing channel response = %#v, want channel denial", denied)
	}
}

func TestE2EAtlassianIntegrationFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()
	mission := approveAtlassianMission(t, service)

	binding := postJSON[AtlassianSiteBinding](t, router, "/v1/integrations/atlassian/site-bindings", CreateAtlassianSiteBindingRequest{
		TenantID:            "demo",
		SiteURL:             "https://acme.atlassian.net/",
		CloudID:             "cloud_123",
		SiteName:            "Acme Atlassian",
		MissionRef:          mission.MissionRef,
		JiraProjectKeys:     []string{"FIN"},
		ConfluenceSpaceKeys: []string{"ENG"},
		AllowedJiraActions:  []string{AtlassianJiraActionTransitionIssue},
		AllowedPageActions:  []string{AtlassianConfluenceActionUpdatePage},
		RequiredGroups:      []string{"Mission Operators"},
		GroupMatchMode:      AtlassianGroupMatchAny,
	})
	if binding.SiteURL != "https://acme.atlassian.net" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected Atlassian binding: %#v", binding)
	}

	listed := getJSON[struct {
		SiteBindings []AtlassianSiteBinding `json:"site_bindings"`
	}](t, router, "/v1/integrations/atlassian/site-bindings")
	if len(listed.SiteBindings) != 1 || listed.SiteBindings[0].BindingID != binding.BindingID {
		t.Fatalf("Atlassian binding list = %#v, want binding %s", listed.SiteBindings, binding.BindingID)
	}

	jira := postJSON[AtlassianActionAuthorizationResponse](t, router, "/v1/integrations/atlassian/jira/issues/authorize", AuthorizeAtlassianJiraIssueActionRequest{
		TenantID:  "demo",
		SiteURL:   "https://acme.atlassian.net",
		CloudID:   "cloud_123",
		AccountID: "acc_123",
		Email:     "agent@example.com",
		Groups:    []string{"Mission Operators"},
		IssueKey:  "FIN-77",
		Action:    AtlassianJiraActionTransitionIssue,
		Context:   map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &AtlassianEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              AtlassianActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: AtlassianEvaluationAction{
				Type:      "jira_issue_transition",
				Resource:  AtlassianEvaluationActionResource{Type: "jira_issue", ID: "FIN-77"},
				Operation: "transition",
			},
		},
	})
	if !jira.Accepted || jira.Evaluation == nil || jira.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("Jira authorization = %#v, want accepted allow", jira)
	}
	if jira.Context["atlassian.product"] != "jira" || jira.Context["jira.project_key"] != "FIN" {
		t.Fatalf("Jira context = %#v, want product and project context", jira.Context)
	}

	confluence := postJSON[AtlassianActionAuthorizationResponse](t, router, "/v1/integrations/atlassian/confluence/pages/authorize", AuthorizeAtlassianConfluencePageActionRequest{
		TenantID:  "demo",
		SiteURL:   "https://acme.atlassian.net",
		CloudID:   "cloud_123",
		AccountID: "acc_123",
		Email:     "agent@example.com",
		Groups:    []string{"Mission Operators"},
		SpaceKey:  "ENG",
		PageID:    "12345",
		PageTitle: "Runbook",
		Action:    AtlassianConfluenceActionUpdatePage,
		Context:   map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &AtlassianEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              AtlassianActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: AtlassianEvaluationAction{
				Type:      "confluence_page_update",
				Resource:  AtlassianEvaluationActionResource{Type: "confluence_page", ID: "ENG:12345"},
				Operation: "update",
			},
		},
	})
	if !confluence.Accepted || confluence.Evaluation == nil || confluence.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("Confluence authorization = %#v, want accepted allow", confluence)
	}

	denied := postJSON[AtlassianActionAuthorizationResponse](t, router, "/v1/integrations/atlassian/jira/issues/authorize", AuthorizeAtlassianJiraIssueActionRequest{
		TenantID:  "demo",
		SiteURL:   "https://acme.atlassian.net",
		CloudID:   "cloud_123",
		AccountID: "acc_123",
		Groups:    []string{"Mission Operators"},
		IssueKey:  "HR-9",
		Action:    AtlassianJiraActionTransitionIssue,
	})
	if denied.Accepted || !contains(denied.ReasonCodes, "jira_project_not_allowed") {
		t.Fatalf("Jira denied authorization = %#v, want project denial", denied)
	}
}

func TestE2EGrandGovernanceFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()

	proposal := postJSON[CreateProposalResponse](t, router, "/v1/mission-proposals", validProposalRequest())
	mission := postJSON[ApproveProposalResponse](t, router, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "e2e-test"},
	})

	negotiation := postJSON[AuthorityNegotiation](t, router, "/v1/missions/"+mission.MissionRef+"/authority/negotiations", CreateAuthorityNegotiationRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "delete"}}}},
	})
	if negotiation.Status != NegotiationStatusCounteroffered {
		t.Fatalf("negotiation status = %s, want %s", negotiation.Status, NegotiationStatusCounteroffered)
	}

	_ = postJSON[ProjectionResponse](t, router, "/v1/missions/"+mission.MissionRef+"/projections", CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Type:               ProjectionTypeMCPContext,
		TTLSeconds:         60,
	})

	rule := postJSON[ContainmentRule](t, router, "/v1/containment-rules", ContainmentRule{
		TenantID:   "demo",
		TargetType: ContainmentTargetAgent,
		TargetID:   "inst_123",
		Reason:     "e2e containment",
	})
	radius := getJSON[BlastRadius](t, router, "/v1/containment-rules/"+rule.RuleID+"/blast-radius")
	if len(radius.Missions) != 1 || len(radius.Projections) != 1 {
		t.Fatalf("blast radius = %#v, want mission and projection", radius)
	}

	contained := postJSON[EvaluateResponse](t, router, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if contained.Decision != DecisionDeny || !contains(contained.ReasonCodes, "CONTAINMENT_ACTIVE") {
		t.Fatalf("contained decision = %#v, want containment deny", contained)
	}

	lineage := getJSON[LineageGraph](t, router, "/v1/missions/"+mission.MissionRef+"/lineage")
	if !lineageHasNode(lineage, "mission:"+mission.MissionRef) {
		t.Fatalf("lineage = %#v, want mission node", lineage)
	}

	_ = postJSON[ContainmentRule](t, router, "/v1/containment-rules/"+rule.RuleID+"/lift", StateChangeRequest{Reason: "resolved"})
	allowed := postJSON[EvaluateResponse](t, router, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if allowed.Decision != DecisionAllow {
		t.Fatalf("allowed decision = %s, want %s", allowed.Decision, DecisionAllow)
	}
}

func TestE2EEntraIntegrationFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()

	proposalReq := validProposalRequest()
	proposalReq.Intent = Purpose{Objective: "Govern Entra-authenticated agent work"}
	proposalReq.Conditions = nil
	proposal := postJSON[CreateProposalResponse](t, router, "/v1/mission-proposals", proposalReq)
	mission := postJSON[ApproveProposalResponse](t, router, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "e2e-test"},
	})

	registration := postJSON[EntraAppRegistration](t, router, "/v1/integrations/entra/app-registrations", CreateEntraAppRegistrationRequest{
		Issuer:          "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0/",
		ClientID:        "00000000-0000-0000-0000-000000000000",
		AppID:           "app_entra_001",
		AppName:         "Auth Scope Console",
		MissionRef:      mission.MissionRef,
		RequiredGroups:  []string{"Mission Operators"},
		AdminGroups:     []string{"Mission Admins"},
		AllowedSubjects: []string{"user@example.onmicrosoft.com"},
		GroupMatchMode:  EntraGroupMatchAny,
	})
	if registration.Issuer != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0" ||
		registration.JWKSURI != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/discovery/v2.0/keys" {
		t.Fatalf("entra registration = %#v, want normalized issuer and jwks uri", registration)
	}

	listed := getJSON[struct {
		AppRegistrations []EntraAppRegistration `json:"app_registrations"`
	}](t, router, "/v1/integrations/entra/app-registrations")
	if len(listed.AppRegistrations) != 1 || listed.AppRegistrations[0].RegistrationID != registration.RegistrationID {
		t.Fatalf("listed entra registrations = %#v, want registration %s", listed.AppRegistrations, registration.RegistrationID)
	}

	resolved := postJSON[EntraAuthorityContextResponse](t, router, "/v1/integrations/entra/authority-context/resolve", ResolveEntraAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"azp":    "00000000-0000-0000-0000-000000000000",
			"sub":    "user@example.onmicrosoft.com",
			"groups": []any{"Mission Operators", "Mission Admins"},
			"roles":  []any{"Reader"},
		},
		Context: map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &EntraEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              EntraActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: EntraEvaluationAction{
				Type:      "tool_call",
				Resource:  EntraEvaluationActionResource{Type: "drive_folder", ID: "board"},
				Operation: "read",
			},
		},
	})
	if !resolved.Accepted || !resolved.Admin || resolved.RegistrationID != registration.RegistrationID {
		t.Fatalf("entra resolved context = %#v, want accepted admin context", resolved)
	}
	if resolved.Evaluation == nil || resolved.Evaluation.Decision != string(DecisionAllow) || resolved.Evaluation.DecisionArtifact == "" {
		t.Fatalf("entra evaluation = %#v, want allow with decision artifact", resolved.Evaluation)
	}
	if resolved.Context["entra.registration_id"] != registration.RegistrationID || resolved.Context["finance.close.status"] != "open" {
		t.Fatalf("entra context = %#v, want registration and caller context", resolved.Context)
	}

	denied := postJSON[EntraAuthorityContextResponse](t, router, "/v1/integrations/entra/authority-context/resolve", ResolveEntraAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"aud":    "00000000-0000-0000-0000-000000000000",
			"sub":    "user@example.onmicrosoft.com",
			"groups": []any{"Everyone"},
		},
	})
	if denied.Accepted || denied.Status != EntraResolutionStatusDenied || !contains(denied.ReasonCodes, "entra_required_group_missing") {
		t.Fatalf("entra denied context = %#v, want required-group denial", denied)
	}
}

func postJSON[T any](t *testing.T, router http.Handler, path string, value any) T {
	return postJSONAsAdmin[T](t, router, path, value, defaultDevelopmentAdminToken)
}

func postJSONAsAdmin[T any](t *testing.T, router http.Handler, path string, value any, token string) T {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code < 200 || resp.Code >= 300 {
		t.Fatalf("POST %s status = %d body=%s", path, resp.Code, resp.Body.String())
	}
	var decoded T
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return decoded
}

func getJSON[T any](t *testing.T, router http.Handler, path string) T {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+defaultDevelopmentAdminToken)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code < 200 || resp.Code >= 300 {
		t.Fatalf("GET %s status = %d body=%s", path, resp.Code, resp.Body.String())
	}
	var decoded T
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return decoded
}

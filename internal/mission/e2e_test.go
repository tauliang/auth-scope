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

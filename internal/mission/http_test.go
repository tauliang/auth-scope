package mission

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAndDiscoveryAPI(t *testing.T) {
	router := testRouter()

	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", health.Code, http.StatusOK)
	}

	discovery := httptest.NewRecorder()
	router.ServeHTTP(discovery, httptest.NewRequest(http.MethodGet, "/.well-known/mission-authority", nil))
	if discovery.Code != http.StatusOK {
		t.Fatalf("discovery status = %d, want %d", discovery.Code, http.StatusOK)
	}
	var body map[string]any
	decodeTestJSON(t, discovery.Body.Bytes(), &body)
	if body["api_base"] == "" {
		t.Fatalf("expected api_base in discovery: %#v", body)
	}
	supports, ok := body["supports"].(map[string]any)
	if !ok || supports["expansion_requests"] != true || supports["tool_gateway_enforcement"] != true {
		t.Fatalf("expected governance support in discovery: %#v", body)
	}
}

func TestCreateProposalAPIValidation(t *testing.T) {
	router := testRouter()

	badJSON := httptest.NewRecorder()
	router.ServeHTTP(badJSON, httptest.NewRequest(http.MethodPost, "/v1/mission-proposals", bytes.NewBufferString("{")))
	if badJSON.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON status = %d, want %d", badJSON.Code, http.StatusBadRequest)
	}

	unknownField := httptest.NewRecorder()
	router.ServeHTTP(unknownField, jsonRequest(http.MethodPost, "/v1/mission-proposals", map[string]any{"unknown": true}))
	if unknownField.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want %d", unknownField.Code, http.StatusBadRequest)
	}

	missingRequired := httptest.NewRecorder()
	router.ServeHTTP(missingRequired, jsonRequest(http.MethodPost, "/v1/mission-proposals", CreateProposalRequest{}))
	if missingRequired.Code != http.StatusBadRequest {
		t.Fatalf("missing required status = %d, want %d", missingRequired.Code, http.StatusBadRequest)
	}
}

func TestAPIProposalApprovalEvaluationAndEvents(t *testing.T) {
	router := testRouter()

	create := httptest.NewRecorder()
	router.ServeHTTP(create, jsonRequest(http.MethodPost, "/v1/mission-proposals", validProposalRequest()))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var proposal CreateProposalResponse
	decodeTestJSON(t, create.Body.Bytes(), &proposal)

	approve := httptest.NewRecorder()
	router.ServeHTTP(approve, jsonRequest(http.MethodPost, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "api-test"},
	}))
	if approve.Code != http.StatusCreated {
		t.Fatalf("approve status = %d body=%s", approve.Code, approve.Body.String())
	}
	var missionResp ApproveProposalResponse
	decodeTestJSON(t, approve.Body.Bytes(), &missionResp)

	evaluate := httptest.NewRecorder()
	router.ServeHTTP(evaluate, jsonRequest(http.MethodPost, "/v1/missions/"+missionResp.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: missionResp.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	}))
	if evaluate.Code != http.StatusOK {
		t.Fatalf("evaluate status = %d body=%s", evaluate.Code, evaluate.Body.String())
	}
	var decision EvaluateResponse
	decodeTestJSON(t, evaluate.Body.Bytes(), &decision)
	if decision.Decision != DecisionAllow {
		t.Fatalf("decision = %s, want %s", decision.Decision, DecisionAllow)
	}

	resume := httptest.NewRecorder()
	router.ServeHTTP(resume, jsonRequest(http.MethodPost, "/v1/missions/"+missionResp.MissionRef+"/resume", ResumeRequest{
		MissionVersionSeen: missionResp.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	}))
	if resume.Code != http.StatusOK {
		t.Fatalf("resume status = %d body=%s", resume.Code, resume.Body.String())
	}

	introspect := httptest.NewRecorder()
	router.ServeHTTP(introspect, httptest.NewRequest(http.MethodGet, "/v1/missions/"+missionResp.MissionRef+"/introspect", nil))
	if introspect.Code != http.StatusOK {
		t.Fatalf("introspect status = %d body=%s", introspect.Code, introspect.Body.String())
	}

	events := httptest.NewRecorder()
	router.ServeHTTP(events, httptest.NewRequest(http.MethodGet, "/v1/events", nil))
	if events.Code != http.StatusOK {
		t.Fatalf("events status = %d body=%s", events.Code, events.Body.String())
	}
}

func TestAPIStateChangesAndErrors(t *testing.T) {
	router := testRouter()

	notFound := httptest.NewRecorder()
	router.ServeHTTP(notFound, jsonRequest(http.MethodPost, "/v1/mission-proposals/missing/approve", ApproveProposalRequest{}))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d, want %d", notFound.Code, http.StatusNotFound)
	}

	mission := createAPIMission(t, router)

	delegateBad := httptest.NewRecorder()
	router.ServeHTTP(delegateBad, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/delegate", DelegationRequest{
		DelegatingActor: Actor{AgentInstanceID: "wrong", ClientID: "research-agent"},
	}))
	if delegateBad.Code != http.StatusBadRequest {
		t.Fatalf("delegate bad status = %d, want %d", delegateBad.Code, http.StatusBadRequest)
	}

	complete := httptest.NewRecorder()
	router.ServeHTTP(complete, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/complete", StateChangeRequest{Reason: "done"}))
	if complete.Code != http.StatusOK {
		t.Fatalf("complete status = %d body=%s", complete.Code, complete.Body.String())
	}

	revokeMissing := httptest.NewRecorder()
	router.ServeHTTP(revokeMissing, jsonRequest(http.MethodPost, "/v1/missions/missing/revoke", StateChangeRequest{Reason: "missing"}))
	if revokeMissing.Code != http.StatusNotFound {
		t.Fatalf("revoke missing status = %d, want %d", revokeMissing.Code, http.StatusNotFound)
	}
}

func TestAPIGovernanceExpansionArtifactAndToolGateway(t *testing.T) {
	router := testRouter()
	mission := createAPIMission(t, router)

	evaluate := httptest.NewRecorder()
	router.ServeHTTP(evaluate, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "high", "reversible": false},
	}))
	if evaluate.Code != http.StatusOK {
		t.Fatalf("evaluate status = %d body=%s", evaluate.Code, evaluate.Body.String())
	}
	var decision EvaluateResponse
	decodeTestJSON(t, evaluate.Body.Bytes(), &decision)
	if decision.Decision != DecisionRequireApproval {
		t.Fatalf("decision = %s, want %s", decision.Decision, DecisionRequireApproval)
	}
	expansionID, ok := decision.Constraints["expansion_request_id"].(string)
	if !ok || expansionID == "" {
		t.Fatalf("missing expansion id in decision: %#v", decision)
	}

	getExpansion := httptest.NewRecorder()
	router.ServeHTTP(getExpansion, httptest.NewRequest(http.MethodGet, "/v1/expansion-requests/"+expansionID, nil))
	if getExpansion.Code != http.StatusOK {
		t.Fatalf("get expansion status = %d body=%s", getExpansion.Code, getExpansion.Body.String())
	}
	var expansion ExpansionRequest
	decodeTestJSON(t, getExpansion.Body.Bytes(), &expansion)
	if expansion.Status != ExpansionStatusPending {
		t.Fatalf("expansion status = %s, want pending", expansion.Status)
	}

	approve := httptest.NewRecorder()
	router.ServeHTTP(approve, jsonRequest(http.MethodPost, "/v1/expansion-requests/"+expansionID+"/approve", ExpansionDecisionRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "api-test"},
	}))
	if approve.Code != http.StatusOK {
		t.Fatalf("approve expansion status = %d body=%s", approve.Code, approve.Body.String())
	}
	var approval ExpansionDecisionResponse
	decodeTestJSON(t, approve.Body.Bytes(), &approval)
	if approval.Status != ExpansionStatusApproved || approval.MissionVersion != mission.MissionVersion+1 {
		t.Fatalf("unexpected approval: %#v", approval)
	}

	verify := httptest.NewRecorder()
	router.ServeHTTP(verify, jsonRequest(http.MethodPost, "/v1/decision-artifacts/verify", VerifyDecisionArtifactRequest{
		DecisionArtifact: decision.DecisionArtifact,
	}))
	if verify.Code != http.StatusOK {
		t.Fatalf("verify status = %d body=%s", verify.Code, verify.Body.String())
	}
	var verified VerifyDecisionArtifactResponse
	decodeTestJSON(t, verify.Body.Bytes(), &verified)
	if !verified.Valid || verified.Evidence == nil || verified.Payload.PolicyVersion != DefaultPolicyVersionID {
		t.Fatalf("unexpected verify response: %#v", verified)
	}

	registerTool := httptest.NewRecorder()
	router.ServeHTTP(registerTool, jsonRequest(http.MethodPost, "/v1/tool-contracts", ToolContract{
		ToolName:        "drive.read",
		ResourceType:    "drive_folder",
		ResourceIDParam: "folder_id",
		Operation:       "read",
		RequiredContext: []string{"finance.close.status"},
	}))
	if registerTool.Code != http.StatusCreated {
		t.Fatalf("register tool status = %d body=%s", registerTool.Code, registerTool.Body.String())
	}

	getTool := httptest.NewRecorder()
	router.ServeHTTP(getTool, httptest.NewRequest(http.MethodGet, "/v1/tool-contracts/drive.read", nil))
	if getTool.Code != http.StatusOK {
		t.Fatalf("get tool status = %d body=%s", getTool.Code, getTool.Body.String())
	}

	authorize := httptest.NewRecorder()
	router.ServeHTTP(authorize, jsonRequest(http.MethodPost, "/v1/tool-calls/authorize", AuthorizeToolCallRequest{
		MissionRef:         mission.MissionRef,
		MissionVersionSeen: approval.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		ToolName:           "drive.read",
		Arguments:          map[string]any{"folder_id": "board"},
		Context:            map[string]any{"finance.close.status": "open"},
	}))
	if authorize.Code != http.StatusOK {
		t.Fatalf("authorize status = %d body=%s", authorize.Code, authorize.Body.String())
	}
	var toolDecision AuthorizeToolCallResponse
	decodeTestJSON(t, authorize.Body.Bytes(), &toolDecision)
	if toolDecision.Decision != DecisionAllow || toolDecision.DecisionArtifact == "" {
		t.Fatalf("expected allowed tool call, got %#v", toolDecision)
	}
}

func TestAPIManualExpansionRequestAndDenial(t *testing.T) {
	router := testRouter()
	mission := createAPIMission(t, router)

	create := httptest.NewRecorder()
	router.ServeHTTP(create, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/expansion-requests", CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "crm_account", ID: "acme"}, Operation: "update"},
		Justification:      "need customer status",
	}))
	if create.Code != http.StatusCreated {
		t.Fatalf("create expansion status = %d body=%s", create.Code, create.Body.String())
	}
	var expansion ExpansionRequest
	decodeTestJSON(t, create.Body.Bytes(), &expansion)
	if expansion.Status != ExpansionStatusPending {
		t.Fatalf("expansion status = %s, want pending", expansion.Status)
	}

	deny := httptest.NewRecorder()
	router.ServeHTTP(deny, jsonRequest(http.MethodPost, "/v1/expansion-requests/"+expansion.ExpansionID+"/deny", ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		Reason:   "not needed",
	}))
	if deny.Code != http.StatusOK {
		t.Fatalf("deny expansion status = %d body=%s", deny.Code, deny.Body.String())
	}
	var denied ExpansionDecisionResponse
	decodeTestJSON(t, deny.Body.Bytes(), &denied)
	if denied.Status != ExpansionStatusDenied {
		t.Fatalf("denied status = %s, want denied", denied.Status)
	}

	duplicateTool := httptest.NewRecorder()
	router.ServeHTTP(duplicateTool, jsonRequest(http.MethodPost, "/v1/tool-contracts", ToolContract{
		ToolName:        "drive.read",
		ResourceType:    "drive_folder",
		ResourceIDParam: "folder_id",
		Operation:       "read",
	}))
	if duplicateTool.Code != http.StatusCreated {
		t.Fatalf("first tool contract status = %d body=%s", duplicateTool.Code, duplicateTool.Body.String())
	}
	duplicateTool = httptest.NewRecorder()
	router.ServeHTTP(duplicateTool, jsonRequest(http.MethodPost, "/v1/tool-contracts", ToolContract{
		ToolName:        "drive.read",
		ResourceType:    "drive_folder",
		ResourceIDParam: "folder_id",
		Operation:       "read",
	}))
	if duplicateTool.Code != http.StatusConflict {
		t.Fatalf("duplicate tool contract status = %d, want %d body=%s", duplicateTool.Code, http.StatusConflict, duplicateTool.Body.String())
	}
}

func TestAPIProjectionLeaseApprovalRuleAndEventStream(t *testing.T) {
	router := testRouter()
	mission := createAPIMission(t, router)

	createProjection := httptest.NewRecorder()
	router.ServeHTTP(createProjection, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/projections", CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Type:               ProjectionTypeToolGatewayToken,
		TTLSeconds:         60,
	}))
	if createProjection.Code != http.StatusCreated {
		t.Fatalf("create projection status = %d body=%s", createProjection.Code, createProjection.Body.String())
	}
	var projection ProjectionResponse
	decodeTestJSON(t, createProjection.Body.Bytes(), &projection)
	if projection.Token == "" || projection.Status != ProjectionStatusActive {
		t.Fatalf("unexpected projection: %#v", projection)
	}

	verifyProjection := httptest.NewRecorder()
	router.ServeHTTP(verifyProjection, jsonRequest(http.MethodPost, "/v1/projections/verify", VerifyProjectionRequest{Token: projection.Token}))
	if verifyProjection.Code != http.StatusOK {
		t.Fatalf("verify projection status = %d body=%s", verifyProjection.Code, verifyProjection.Body.String())
	}
	var verified VerifyProjectionResponse
	decodeTestJSON(t, verifyProjection.Body.Bytes(), &verified)
	if !verified.Valid || verified.Payload.ProjectionID != projection.ProjectionID {
		t.Fatalf("unexpected verify projection response: %#v", verified)
	}

	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/v1/projections/"+projection.ProjectionID+"/status", nil))
	if status.Code != http.StatusOK {
		t.Fatalf("projection status code = %d body=%s", status.Code, status.Body.String())
	}

	createLease := httptest.NewRecorder()
	router.ServeHTTP(createLease, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/leases", CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TTLSeconds:         30,
	}))
	if createLease.Code != http.StatusCreated {
		t.Fatalf("create lease status = %d body=%s", createLease.Code, createLease.Body.String())
	}
	var lease LeaseResponse
	decodeTestJSON(t, createLease.Body.Bytes(), &lease)
	if lease.Decision != DecisionAllow || lease.LeaseID == "" {
		t.Fatalf("unexpected lease: %#v", lease)
	}

	refreshLease := httptest.NewRecorder()
	router.ServeHTTP(refreshLease, jsonRequest(http.MethodPost, "/v1/leases/"+lease.LeaseID+"/refresh", RefreshLeaseRequest{
		Actor:      Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TTLSeconds: 30,
	}))
	if refreshLease.Code != http.StatusOK {
		t.Fatalf("refresh lease status = %d body=%s", refreshLease.Code, refreshLease.Body.String())
	}

	createRule := httptest.NewRecorder()
	router.ServeHTTP(createRule, jsonRequest(http.MethodPost, "/v1/approval-rules", ApprovalRule{
		TenantID:          "demo",
		ResourceType:      "slack_channel",
		ResourceID:        "board",
		Operation:         "post_update",
		RiskLevel:         "high",
		RequiredApprovals: 2,
		AllowedIssuers:    []string{"https://idp.example.com"},
	}))
	if createRule.Code != http.StatusCreated {
		t.Fatalf("create rule status = %d body=%s", createRule.Code, createRule.Body.String())
	}
	listRules := httptest.NewRecorder()
	router.ServeHTTP(listRules, httptest.NewRequest(http.MethodGet, "/v1/approval-rules", nil))
	if listRules.Code != http.StatusOK {
		t.Fatalf("list rules status = %d body=%s", listRules.Code, listRules.Body.String())
	}

	evaluate := httptest.NewRecorder()
	router.ServeHTTP(evaluate, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "high", "reversible": false},
	}))
	if evaluate.Code != http.StatusOK {
		t.Fatalf("evaluate status = %d body=%s", evaluate.Code, evaluate.Body.String())
	}
	var decision EvaluateResponse
	decodeTestJSON(t, evaluate.Body.Bytes(), &decision)
	expansionID, _ := decision.Constraints["expansion_request_id"].(string)
	if expansionID == "" {
		t.Fatalf("expected expansion id: %#v", decision)
	}

	directApprove := httptest.NewRecorder()
	router.ServeHTTP(directApprove, jsonRequest(http.MethodPost, "/v1/expansion-requests/"+expansionID+"/approve", ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}))
	if directApprove.Code != http.StatusBadRequest {
		t.Fatalf("direct approval status = %d, want %d body=%s", directApprove.Code, http.StatusBadRequest, directApprove.Body.String())
	}

	firstApproval := httptest.NewRecorder()
	router.ServeHTTP(firstApproval, jsonRequest(http.MethodPost, "/v1/expansion-requests/"+expansionID+"/approvals", SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}))
	if firstApproval.Code != http.StatusOK {
		t.Fatalf("first approval status = %d body=%s", firstApproval.Code, firstApproval.Body.String())
	}
	var partial SubmitExpansionApprovalResponse
	decodeTestJSON(t, firstApproval.Body.Bytes(), &partial)
	if partial.Status != ExpansionStatusPending || partial.ApprovalsReceived != 1 {
		t.Fatalf("unexpected partial approval: %#v", partial)
	}

	secondApproval := httptest.NewRecorder()
	router.ServeHTTP(secondApproval, jsonRequestAsAdmin(http.MethodPost, "/v1/expansion-requests/"+expansionID+"/approvals", SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "spoofed@example.com", Issuer: "https://evil.example.com"},
	}, defaultDevelopmentSecondAdminToken))
	if secondApproval.Code != http.StatusOK {
		t.Fatalf("second approval status = %d body=%s", secondApproval.Code, secondApproval.Body.String())
	}
	var approved SubmitExpansionApprovalResponse
	decodeTestJSON(t, secondApproval.Body.Bytes(), &approved)
	if approved.Status != ExpansionStatusApproved {
		t.Fatalf("unexpected approval response: %#v", approved)
	}

	revokeProjection := httptest.NewRecorder()
	router.ServeHTTP(revokeProjection, jsonRequest(http.MethodPost, "/v1/projections/"+projection.ProjectionID+"/revoke", StateChangeRequest{Reason: "test"}))
	if revokeProjection.Code != http.StatusOK {
		t.Fatalf("revoke projection status = %d body=%s", revokeProjection.Code, revokeProjection.Body.String())
	}

	stream := httptest.NewRecorder()
	router.ServeHTTP(stream, httptest.NewRequest(http.MethodGet, "/v1/events/stream", nil))
	if stream.Code != http.StatusOK || stream.Header().Get("content-type") != "text/event-stream" {
		t.Fatalf("stream status/header = %d/%q body=%s", stream.Code, stream.Header().Get("content-type"), stream.Body.String())
	}
	if !bytes.Contains(stream.Body.Bytes(), []byte("mission.projection_revoked")) {
		t.Fatalf("expected projection revoked event in stream, body=%s", stream.Body.String())
	}
}

func TestAPIAuthorityNegotiationContainmentAndLineage(t *testing.T) {
	router := testRouter()
	mission := createAPIMission(t, router)

	createNegotiation := httptest.NewRecorder()
	router.ServeHTTP(createNegotiation, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/authority/negotiations", CreateAuthorityNegotiationRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "delete"}}}},
	}))
	if createNegotiation.Code != http.StatusCreated {
		t.Fatalf("create negotiation status = %d body=%s", createNegotiation.Code, createNegotiation.Body.String())
	}
	var negotiation AuthorityNegotiation
	decodeTestJSON(t, createNegotiation.Body.Bytes(), &negotiation)
	if negotiation.Status != NegotiationStatusCounteroffered {
		t.Fatalf("negotiation status = %s, want %s", negotiation.Status, NegotiationStatusCounteroffered)
	}

	getNegotiation := httptest.NewRecorder()
	router.ServeHTTP(getNegotiation, httptest.NewRequest(http.MethodGet, "/v1/authority/negotiations/"+negotiation.NegotiationID, nil))
	if getNegotiation.Code != http.StatusOK {
		t.Fatalf("get negotiation status = %d body=%s", getNegotiation.Code, getNegotiation.Body.String())
	}

	createRule := httptest.NewRecorder()
	router.ServeHTTP(createRule, jsonRequest(http.MethodPost, "/v1/containment-rules", ContainmentRule{
		TenantID:   "demo",
		TargetType: ContainmentTargetAgent,
		TargetID:   "inst_123",
		Reason:     "api test",
		CreatedBy:  Principal{Subject: "security@example.com", Issuer: "https://idp.example.com"},
	}))
	if createRule.Code != http.StatusCreated {
		t.Fatalf("create containment rule status = %d body=%s", createRule.Code, createRule.Body.String())
	}
	var rule ContainmentRule
	decodeTestJSON(t, createRule.Body.Bytes(), &rule)

	listRules := httptest.NewRecorder()
	router.ServeHTTP(listRules, httptest.NewRequest(http.MethodGet, "/v1/containment-rules", nil))
	if listRules.Code != http.StatusOK {
		t.Fatalf("list containment rules status = %d body=%s", listRules.Code, listRules.Body.String())
	}

	blastRadius := httptest.NewRecorder()
	router.ServeHTTP(blastRadius, httptest.NewRequest(http.MethodGet, "/v1/containment-rules/"+rule.RuleID+"/blast-radius", nil))
	if blastRadius.Code != http.StatusOK {
		t.Fatalf("blast radius status = %d body=%s", blastRadius.Code, blastRadius.Body.String())
	}
	var radius BlastRadius
	decodeTestJSON(t, blastRadius.Body.Bytes(), &radius)
	if len(radius.Missions) != 1 || radius.Missions[0].MissionRef != mission.MissionRef {
		t.Fatalf("unexpected blast radius: %#v", radius)
	}

	evaluate := httptest.NewRecorder()
	router.ServeHTTP(evaluate, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	}))
	if evaluate.Code != http.StatusOK {
		t.Fatalf("evaluate contained status = %d body=%s", evaluate.Code, evaluate.Body.String())
	}
	var decision EvaluateResponse
	decodeTestJSON(t, evaluate.Body.Bytes(), &decision)
	if decision.Decision != DecisionDeny || !contains(decision.ReasonCodes, "CONTAINMENT_ACTIVE") {
		t.Fatalf("expected containment denial, got %#v", decision)
	}

	missionLineage := httptest.NewRecorder()
	router.ServeHTTP(missionLineage, httptest.NewRequest(http.MethodGet, "/v1/missions/"+mission.MissionRef+"/lineage", nil))
	if missionLineage.Code != http.StatusOK {
		t.Fatalf("mission lineage status = %d body=%s", missionLineage.Code, missionLineage.Body.String())
	}
	var lineage LineageGraph
	decodeTestJSON(t, missionLineage.Body.Bytes(), &lineage)
	if !lineageHasNode(lineage, "mission:"+mission.MissionRef) {
		t.Fatalf("lineage missing mission: %#v", lineage)
	}

	agentLineage := httptest.NewRecorder()
	router.ServeHTTP(agentLineage, httptest.NewRequest(http.MethodGet, "/v1/agents/inst_123/lineage", nil))
	if agentLineage.Code != http.StatusOK {
		t.Fatalf("agent lineage status = %d body=%s", agentLineage.Code, agentLineage.Body.String())
	}

	lift := httptest.NewRecorder()
	router.ServeHTTP(lift, jsonRequest(http.MethodPost, "/v1/containment-rules/"+rule.RuleID+"/lift", StateChangeRequest{Reason: "resolved"}))
	if lift.Code != http.StatusOK {
		t.Fatalf("lift containment status = %d body=%s", lift.Code, lift.Body.String())
	}

	evaluate = httptest.NewRecorder()
	router.ServeHTTP(evaluate, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	}))
	if evaluate.Code != http.StatusOK {
		t.Fatalf("evaluate after lift status = %d body=%s", evaluate.Code, evaluate.Body.String())
	}
	decodeTestJSON(t, evaluate.Body.Bytes(), &decision)
	if decision.Decision != DecisionAllow {
		t.Fatalf("decision after lift = %s, want %s", decision.Decision, DecisionAllow)
	}
}

func testRouter() http.Handler {
	service := testService()
	return withTestAdminAuthorization(NewHandler(service).Routes())
}

func withTestAdminAuthorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", "Bearer "+defaultDevelopmentAdminToken)
		}
		next.ServeHTTP(w, r)
	})
}

func createAPIMission(t *testing.T, router http.Handler) ApproveProposalResponse {
	t.Helper()
	create := httptest.NewRecorder()
	router.ServeHTTP(create, jsonRequest(http.MethodPost, "/v1/mission-proposals", validProposalRequest()))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var proposal CreateProposalResponse
	decodeTestJSON(t, create.Body.Bytes(), &proposal)

	approve := httptest.NewRecorder()
	router.ServeHTTP(approve, jsonRequest(http.MethodPost, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}))
	if approve.Code != http.StatusCreated {
		t.Fatalf("approve status = %d body=%s", approve.Code, approve.Body.String())
	}
	var mission ApproveProposalResponse
	decodeTestJSON(t, approve.Body.Bytes(), &mission)
	return mission
}

func jsonRequest(method string, path string, value any) *http.Request {
	return jsonRequestAsAdmin(method, path, value, defaultDevelopmentAdminToken)
}

func jsonRequestAsAdmin(method string, path string, value any, token string) *http.Request {
	var body bytes.Buffer
	_ = json.NewEncoder(&body).Encode(value)
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func decodeTestJSON(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("decode JSON %s: %v", string(data), err)
	}
}

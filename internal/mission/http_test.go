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

func testRouter() http.Handler {
	service := testService()
	return NewHandler(service).Routes()
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
	var body bytes.Buffer
	_ = json.NewEncoder(&body).Encode(value)
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("content-type", "application/json")
	return req
}

func decodeTestJSON(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("decode JSON %s: %v", string(data), err)
	}
}

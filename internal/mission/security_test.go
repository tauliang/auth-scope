package mission

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentRegistrationSignatureReplayAndRevocation(t *testing.T) {
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
	if registered.Status != AgentStatusActive || registered.KeyThumbprint == "" {
		t.Fatalf("registered identity = %#v", registered)
	}

	body := []byte(`{"actor":{"client_id":"research-agent","agent_instance_id":"inst_123"}}`)
	signature := testAgentSignature(privateKey, http.MethodPost, "/v1/missions/mref/evaluate", body, "nonce-1")
	identity, err := service.VerifyAgentRequestSignature(http.MethodPost, "/v1/missions/mref/evaluate", body, registered.AgentID, "nonce-1", signature)
	if err != nil {
		t.Fatalf("VerifyAgentRequestSignature: %v", err)
	}
	if identity.KeyThumbprint != registered.KeyThumbprint {
		t.Fatalf("identity thumbprint = %q, want %q", identity.KeyThumbprint, registered.KeyThumbprint)
	}

	if _, err := service.VerifyAgentRequestSignature(http.MethodPost, "/v1/missions/mref/evaluate", body, registered.AgentID, "nonce-1", signature); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("replayed signature err = %v, want ErrInvalidSignature", err)
	}

	if _, err := service.RevokeAgent(registered.AgentID, StateChangeRequest{Reason: "rotation"}); err != nil {
		t.Fatalf("RevokeAgent: %v", err)
	}
	signature = testAgentSignature(privateKey, http.MethodPost, "/v1/missions/mref/evaluate", body, "nonce-2")
	if _, err := service.VerifyAgentRequestSignature(http.MethodPost, "/v1/missions/mref/evaluate", body, registered.AgentID, "nonce-2", signature); !errors.Is(err, ErrAgentRevoked) {
		t.Fatalf("revoked signature err = %v, want ErrAgentRevoked", err)
	}
}

func TestGovernanceAdminAuthenticationAndIdentityBinding(t *testing.T) {
	service := testService()
	authenticated := Principal{Subject: "security@example.com", Issuer: "https://idp.example.com"}
	router := NewHandlerWithAdminAuthenticator(service, NewMultiBearerAdminAuthenticator(map[string]Principal{
		"verified-admin-token": authenticated,
	})).Routes()

	proposal, err := service.CreateProposal(validProposalRequest())
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	body := ApproveProposalRequest{Approver: Principal{Subject: "spoofed@example.com", Issuer: "https://evil.example.com"}}

	unauthorized := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", body)
	req.Header.Del("Authorization")
	router.ServeHTTP(unauthorized, req)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("missing admin token status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, jsonRequestAsAdmin(http.MethodPost, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", body, "wrong-token"))
	if invalid.Code != http.StatusUnauthorized {
		t.Fatalf("invalid admin token status = %d, want %d", invalid.Code, http.StatusUnauthorized)
	}

	approved := httptest.NewRecorder()
	router.ServeHTTP(approved, jsonRequestAsAdmin(http.MethodPost, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", body, "verified-admin-token"))
	if approved.Code != http.StatusCreated {
		t.Fatalf("approved status = %d body=%s", approved.Code, approved.Body.String())
	}
	var response ApproveProposalResponse
	decodeTestJSON(t, approved.Body.Bytes(), &response)
	stored, err := service.Introspect(response.MissionRef)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	if stored.Approval.Approver != authenticated {
		t.Fatalf("stored approver = %#v, want authenticated principal %#v", stored.Approval.Approver, authenticated)
	}

	contained := httptest.NewRecorder()
	router.ServeHTTP(contained, jsonRequestAsAdmin(http.MethodPost, "/v1/containment-rules", ContainmentRule{
		TenantID:   "demo",
		TargetType: ContainmentTargetMission,
		TargetID:   response.MissionRef,
		CreatedBy:  Principal{Subject: "spoofed@example.com"},
	}, "verified-admin-token"))
	if contained.Code != http.StatusCreated {
		t.Fatalf("containment status = %d body=%s", contained.Code, contained.Body.String())
	}
	var rule ContainmentRule
	decodeTestJSON(t, contained.Body.Bytes(), &rule)
	if rule.CreatedBy != authenticated {
		t.Fatalf("containment creator = %#v, want authenticated principal %#v", rule.CreatedBy, authenticated)
	}
}

func TestEvaluateRequiresBoundKeyThumbprintAndSignsDecisionArtifact(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	stored, err := service.Introspect(mission.MissionRef)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	stored.Agent.KeyThumbprint = "sha256:bound-key"
	if err := service.missions.UpdateMission(stored); err != nil {
		t.Fatalf("UpdateMission: %v", err)
	}

	denied, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		Actor:   Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:  Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context: map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate without key: %v", err)
	}
	if denied.Decision != DecisionDeny || denied.DecisionArtifact == "" {
		t.Fatalf("unbound-key evaluation = %#v, want deny with artifact", denied)
	}

	allowed, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		Actor:   Actor{AgentInstanceID: "inst_123", ClientID: "research-agent", KeyThumbprint: "sha256:bound-key"},
		Action:  Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context: map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate with key: %v", err)
	}
	if allowed.Decision != DecisionAllow {
		t.Fatalf("key-bound evaluation decision = %s, want %s", allowed.Decision, DecisionAllow)
	}
	payload, err := VerifyDecisionArtifact(allowed.DecisionArtifact, service.artifactKey)
	if err != nil {
		t.Fatalf("VerifyDecisionArtifact: %v", err)
	}
	if payload.MissionRef != mission.MissionRef || payload.Actor.KeyThumbprint != "sha256:bound-key" || payload.ContextHash == "" {
		t.Fatalf("artifact payload = %#v", payload)
	}
}

func TestSignedAuthZENEvaluationAPI(t *testing.T) {
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

	proposalReq := validProposalRequest()
	proposalReq.Agent.KeyThumbprint = registered.KeyThumbprint
	proposal, err := service.CreateProposal(proposalReq)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	mission, err := service.ApproveProposal(proposal.ProposalID, ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}

	router := NewHandler(service).Routes()
	reqBody := AuthZENEvaluationRequest{
		Subject:  AuthZENEntity{Type: "agent", ID: "inst_123", Properties: map[string]any{"client_id": "research-agent"}},
		Action:   AuthZENEntity{Type: "tool_call", ID: "read"},
		Resource: AuthZENEntity{Type: "drive_folder", ID: "board"},
		Context:  map[string]any{"mission_ref": mission.MissionRef, "mission_version_seen": mission.MissionVersion, "finance.close.status": "open"},
	}
	req := signedJSONRequest(t, http.MethodPost, "/access/v1/evaluation", reqBody, registered.AgentID, privateKey, "nonce-http-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("authzen evaluation status = %d body=%s", resp.Code, resp.Body.String())
	}
	var decoded AuthZENEvaluationResponse
	decodeTestJSON(t, resp.Body.Bytes(), &decoded)
	if !decoded.Decision || decoded.Context["decision_artifact"] == "" {
		t.Fatalf("authzen response = %#v", decoded)
	}

	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, signedJSONRequest(t, http.MethodPost, "/access/v1/evaluation", reqBody, registered.AgentID, privateKey, "nonce-http-1"))
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want %d", replay.Code, http.StatusUnauthorized)
	}
}

func TestStrictRuntimeHandlerRequiresSignedAgentRequest(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	router := NewHandlerWithOptions(service, AdminAuthenticatorFromEnv(), HandlerOptions{RequireAgentSignatures: true}).Routes()

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, jsonRequest(http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	}))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned runtime request status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestAgentRegistryAPIAndSignedMissionEvaluationAPI(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()
	publicKey, privateKey := testAgentKeypair(t)

	createAgent := httptest.NewRecorder()
	router.ServeHTTP(createAgent, jsonRequest(http.MethodPost, "/v1/agents", RegisterAgentRequest{
		TenantID:  "demo",
		Agent:     Agent{Provider: "https://agents.example.com", ClientID: "research-agent", InstanceID: "inst_123"},
		PublicKey: publicKey,
	}))
	if createAgent.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d body=%s", createAgent.Code, createAgent.Body.String())
	}
	var registered RegisterAgentResponse
	decodeTestJSON(t, createAgent.Body.Bytes(), &registered)

	getAgent := httptest.NewRecorder()
	getAgentReq := httptest.NewRequest(http.MethodGet, "/v1/agents/"+registered.AgentID, nil)
	getAgentReq.Header.Set("Authorization", "Bearer "+defaultDevelopmentAdminToken)
	router.ServeHTTP(getAgent, getAgentReq)
	if getAgent.Code != http.StatusOK {
		t.Fatalf("get agent status = %d body=%s", getAgent.Code, getAgent.Body.String())
	}

	proposalReq := validProposalRequest()
	proposalReq.Agent.KeyThumbprint = registered.KeyThumbprint
	proposal := postJSON[CreateProposalResponse](t, router, "/v1/mission-proposals", proposalReq)
	mission := postJSON[ApproveProposalResponse](t, router, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	})

	evaluateReq := EvaluateRequest{
		Action:  Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context: map[string]any{"finance.close.status": "open"},
	}
	evaluate := httptest.NewRecorder()
	router.ServeHTTP(evaluate, signedJSONRequest(t, http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", evaluateReq, registered.AgentID, privateKey, "nonce-eval-1"))
	if evaluate.Code != http.StatusOK {
		t.Fatalf("signed evaluate status = %d body=%s", evaluate.Code, evaluate.Body.String())
	}
	var decision EvaluateResponse
	decodeTestJSON(t, evaluate.Body.Bytes(), &decision)
	if decision.Decision != DecisionAllow || decision.DecisionArtifact == "" {
		t.Fatalf("signed evaluation = %#v", decision)
	}

	mismatch := httptest.NewRecorder()
	badActorReq := evaluateReq
	badActorReq.Actor = Actor{AgentInstanceID: "other", ClientID: "research-agent"}
	router.ServeHTTP(mismatch, signedJSONRequest(t, http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", badActorReq, registered.AgentID, privateKey, "nonce-eval-2"))
	if mismatch.Code != http.StatusUnauthorized {
		t.Fatalf("actor mismatch status = %d, want %d", mismatch.Code, http.StatusUnauthorized)
	}

	revokeAgent := httptest.NewRecorder()
	router.ServeHTTP(revokeAgent, jsonRequest(http.MethodPost, "/v1/agents/"+registered.AgentID+"/revoke", StateChangeRequest{Reason: "test"}))
	if revokeAgent.Code != http.StatusOK {
		t.Fatalf("revoke agent status = %d body=%s", revokeAgent.Code, revokeAgent.Body.String())
	}

	revoked := httptest.NewRecorder()
	router.ServeHTTP(revoked, signedJSONRequest(t, http.MethodPost, "/v1/missions/"+mission.MissionRef+"/evaluate", evaluateReq, registered.AgentID, privateKey, "nonce-eval-3"))
	if revoked.Code != http.StatusForbidden {
		t.Fatalf("revoked agent status = %d, want %d", revoked.Code, http.StatusForbidden)
	}
}

func TestAuthZENDiscoveryAndBatchAPI(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()
	mission := approveTestMission(t, service)

	discovery := httptest.NewRecorder()
	router.ServeHTTP(discovery, httptest.NewRequest(http.MethodGet, "/.well-known/authzen-configuration", nil))
	if discovery.Code != http.StatusOK {
		t.Fatalf("authzen discovery status = %d", discovery.Code)
	}

	batch := AuthZENEvaluationsRequest{Evaluations: []AuthZENEvaluationRequest{
		{
			Subject:  AuthZENEntity{Type: "agent", ID: "inst_123", Properties: map[string]any{"client_id": "research-agent"}},
			Action:   AuthZENEntity{Type: "tool_call", ID: "call", Properties: map[string]any{"operation": "read"}},
			Resource: AuthZENEntity{Type: "drive_folder", ID: "board", Properties: map[string]any{"mission_ref": mission.MissionRef}},
			Context:  map[string]any{"mission_version_seen": "1", "finance.close.status": "open"},
		},
		{
			Subject:  AuthZENEntity{Type: "agent", ID: "inst_123", Properties: map[string]any{"client_id": "research-agent", "mission_ref": mission.MissionRef}},
			Action:   AuthZENEntity{Type: "tool_call", ID: "delete"},
			Resource: AuthZENEntity{Type: "drive_folder", ID: "board"},
			Context:  map[string]any{"finance.close.status": "open"},
		},
	}}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, jsonRequest(http.MethodPost, "/access/v1/evaluations", batch))
	if resp.Code != http.StatusOK {
		t.Fatalf("authzen batch status = %d body=%s", resp.Code, resp.Body.String())
	}
	var decoded AuthZENEvaluationsResponse
	decodeTestJSON(t, resp.Body.Bytes(), &decoded)
	if len(decoded.Evaluations) != 2 || !decoded.Evaluations[0].Decision || decoded.Evaluations[1].Decision {
		t.Fatalf("authzen batch = %#v", decoded)
	}
}

func TestDecisionArtifactAndAgentValidationFailures(t *testing.T) {
	if _, err := DecodeAgentPublicKey("not-base64"); err == nil {
		t.Fatal("expected invalid public key error")
	}
	service := testService()
	publicKey, privateKey := testAgentKeypair(t)
	if _, err := service.RegisterAgent(RegisterAgentRequest{Agent: Agent{ClientID: "a", InstanceID: "i"}, PublicKey: "bad-key"}); err == nil {
		t.Fatal("expected bad public key registration error")
	}
	if _, err := service.RegisterAgent(RegisterAgentRequest{Agent: Agent{ClientID: "a", InstanceID: "i", KeyThumbprint: "sha256:wrong"}, PublicKey: publicKey}); err == nil {
		t.Fatal("expected thumbprint mismatch error")
	}

	registered, err := service.RegisterAgent(RegisterAgentRequest{Agent: Agent{ClientID: "a", InstanceID: "i"}, PublicKey: publicKey})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	body := []byte(`{}`)
	signature := testAgentSignature(privateKey, http.MethodPost, "/target", body, "nonce")
	if _, err := service.VerifyAgentRequestSignature(http.MethodPost, "/other", body, registered.AgentID, "nonce", signature); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("wrong target err = %v, want ErrInvalidSignature", err)
	}

	payload := DecisionArtifactPayload{ArtifactID: "mdar_test", MissionRef: "mref", Decision: DecisionAllow}
	artifact, err := SignDecisionArtifact(payload, service.artifactKey)
	if err != nil {
		t.Fatalf("SignDecisionArtifact: %v", err)
	}
	if _, err := VerifyDecisionArtifact(artifact+"tampered", service.artifactKey); err == nil {
		t.Fatal("expected tampered artifact verification error")
	}
	if _, err := VerifyDecisionArtifact("not.a.valid.token", service.artifactKey); err == nil {
		t.Fatal("expected malformed artifact verification error")
	}
}

func TestAgentServiceValidationAndErrorBranches(t *testing.T) {
	_ = SystemClock{}.Now()
	service := NewService(NewMemoryStore(), nil)
	if _, err := service.RegisterAgent(RegisterAgentRequest{Agent: Agent{ClientID: "agent"}}); err == nil {
		t.Fatal("expected missing agent instance validation error")
	}
	if _, err := service.VerifyAgentRequestSignature(http.MethodPost, "/target", nil, "", "", ""); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("missing signature err = %v, want ErrInvalidSignature", err)
	}
	if _, err := service.VerifyAgentRequestSignature(http.MethodPost, "/target", nil, "missing", "nonce", "signature"); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("missing agent err = %v, want ErrInvalidSignature", err)
	}
	if _, err := SignDecisionArtifact(DecisionArtifactPayload{}, nil); err == nil {
		t.Fatal("expected missing artifact signing key error")
	}
	if _, err := VerifyDecisionArtifact("", nil); err == nil {
		t.Fatal("expected missing artifact verification key error")
	}
	if _, err := service.EvaluateAuthZEN(AuthZENEvaluationRequest{}); err == nil {
		t.Fatal("expected missing mission_ref authzen error")
	}

	publicKey, _ := testAgentKeypair(t)
	registered, err := service.RegisterAgent(RegisterAgentRequest{
		Agent:     Agent{ClientID: "agent", InstanceID: "inst"},
		PublicKey: publicKey,
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	revoked, err := service.RevokeAgent(registered.AgentID, StateChangeRequest{Reason: "first"})
	if err != nil {
		t.Fatalf("RevokeAgent first: %v", err)
	}
	revokedAgain, err := service.RevokeAgent(registered.AgentID, StateChangeRequest{Reason: "second"})
	if err != nil {
		t.Fatalf("RevokeAgent second: %v", err)
	}
	if revokedAgain.RevokedAt != revoked.RevokedAt {
		t.Fatalf("second revoke should be idempotent: first=%s second=%s", revoked.RevokedAt, revokedAgain.RevokedAt)
	}
}

func testAgentKeypair(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(publicKey), privateKey
}

func testAgentSignature(privateKey ed25519.PrivateKey, method string, target string, body []byte, nonce string) string {
	message := []byte(CanonicalAgentSigningString(method, target, body, nonce))
	return EncodeAgentSignature(ed25519.Sign(privateKey, message))
}

func signedJSONRequest(t *testing.T, method string, path string, value any, agentID string, privateKey ed25519.PrivateKey, nonce string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	bodyBytes := body.Bytes()
	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-auth-scope-agent-id", agentID)
	req.Header.Set("x-auth-scope-nonce", nonce)
	req.Header.Set("x-auth-scope-signature", testAgentSignature(privateKey, method, path, bodyBytes, nonce))
	return req
}

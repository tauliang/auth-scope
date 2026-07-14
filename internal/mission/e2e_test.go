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

func postJSON[T any](t *testing.T, router http.Handler, path string, value any) T {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("content-type", "application/json")
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

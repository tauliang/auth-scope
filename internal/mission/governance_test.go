package mission

import (
	"strings"
	"testing"
	"time"
)

func TestExpansionApprovalExtendsMissionAuthority(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	action := Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"}

	resp, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             action,
		Context:            map[string]any{"finance.close.status": "open", "risk": "high", "reversible": false},
	})
	if err != nil {
		t.Fatalf("Evaluate out of scope: %v", err)
	}
	if resp.Decision != DecisionRequireApproval {
		t.Fatalf("decision = %s, want %s", resp.Decision, DecisionRequireApproval)
	}
	expansionID, ok := resp.Constraints["expansion_request_id"].(string)
	if !ok || !strings.HasPrefix(expansionID, "mex_") {
		t.Fatalf("expected expansion request id in constraints, got %#v", resp.Constraints)
	}
	expansion, err := service.GetExpansionRequest(expansionID)
	if err != nil {
		t.Fatalf("GetExpansionRequest: %v", err)
	}
	if expansion.Status != ExpansionStatusPending || expansion.Action.Operation != action.Operation {
		t.Fatalf("unexpected expansion request: %#v", expansion)
	}

	approved, err := service.ApproveExpansionRequest(expansionID, ExpansionDecisionRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "unit-test"},
		Reason:           "board update is now approved",
	})
	if err != nil {
		t.Fatalf("ApproveExpansionRequest: %v", err)
	}
	if approved.Status != ExpansionStatusApproved || approved.MissionVersion != mission.MissionVersion+1 {
		t.Fatalf("unexpected approval response: %#v", approved)
	}

	allowed, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: approved.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             action,
		Context:            map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
	})
	if err != nil {
		t.Fatalf("Evaluate after expansion: %v", err)
	}
	if allowed.Decision != DecisionAllow {
		t.Fatalf("decision after expansion = %s, want %s: %#v", allowed.Decision, DecisionAllow, allowed)
	}
}

func TestDecisionArtifactVerificationReturnsEvidence(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	allowed, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	verified := service.VerifyDecisionArtifactEvidence(VerifyDecisionArtifactRequest{DecisionArtifact: allowed.DecisionArtifact})
	if !verified.Valid {
		t.Fatalf("expected artifact valid, got error %q", verified.Error)
	}
	if verified.Payload.PolicyVersion != DefaultPolicyVersionID || verified.Payload.EvidenceID == "" {
		t.Fatalf("unexpected artifact payload: %#v", verified.Payload)
	}
	if verified.Evidence == nil {
		t.Fatal("expected stored evidence")
	}
	if verified.Evidence.Decision != DecisionAllow || len(verified.Evidence.ConditionResults) != 1 {
		t.Fatalf("unexpected evidence: %#v", verified.Evidence)
	}

	tampered := service.VerifyDecisionArtifactEvidence(VerifyDecisionArtifactRequest{DecisionArtifact: allowed.DecisionArtifact + "x"})
	if tampered.Valid || tampered.Error == "" {
		t.Fatalf("expected tampered artifact invalid, got %#v", tampered)
	}
}

func TestToolContractAuthorizeToolCall(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	contract, err := service.RegisterToolContract(ToolContract{
		ToolName:        "drive.read",
		ResourceType:    "drive_folder",
		ResourceIDParam: "folder_id",
		Operation:       "read",
		RequiredContext: []string{"finance.close.status"},
	})
	if err != nil {
		t.Fatalf("RegisterToolContract: %v", err)
	}
	if contract.ActionType != "tool_call" || contract.CreatedAt.IsZero() {
		t.Fatalf("expected defaults on contract: %#v", contract)
	}

	missingContext, err := service.AuthorizeToolCall(AuthorizeToolCallRequest{
		MissionRef: mission.MissionRef,
		Actor:      Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		ToolName:   "drive.read",
		Arguments:  map[string]any{"folder_id": "board"},
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall missing context: %v", err)
	}
	if missingContext.Decision != DecisionDeny || missingContext.DecisionArtifact != "" {
		t.Fatalf("expected fail-closed context denial without artifact, got %#v", missingContext)
	}

	allowed, err := service.AuthorizeToolCall(AuthorizeToolCallRequest{
		MissionRef:         mission.MissionRef,
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		ToolName:           "drive.read",
		Arguments:          map[string]any{"folder_id": "board"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall: %v", err)
	}
	if allowed.Decision != DecisionAllow || allowed.DecisionArtifact == "" {
		t.Fatalf("expected allowed tool call with artifact, got %#v", allowed)
	}

	missingArg, err := service.AuthorizeToolCall(AuthorizeToolCallRequest{
		MissionRef: mission.MissionRef,
		Actor:      Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		ToolName:   "drive.read",
		Context:    map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall missing arg: %v", err)
	}
	if missingArg.Decision != DecisionDeny || missingArg.Constraints["missing_argument"] != "folder_id" {
		t.Fatalf("expected argument denial, got %#v", missingArg)
	}
}

func TestExpansionRequestDenialIsTerminal(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	expansion, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "crm_account", ID: "acme"}, Operation: "update"},
		Justification:      "need customer status",
	})
	if err != nil {
		t.Fatalf("CreateExpansionRequest: %v", err)
	}
	denied, err := service.DenyExpansionRequest(expansion.ExpansionID, ExpansionDecisionRequest{
		Approver:         Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{Method: "unit-test", ApprovedAt: time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC)},
		Reason:           "not necessary",
	})
	if err != nil {
		t.Fatalf("DenyExpansionRequest: %v", err)
	}
	if denied.Status != ExpansionStatusDenied || denied.MissionVersion != mission.MissionVersion {
		t.Fatalf("unexpected denial response: %#v", denied)
	}
	if _, err := service.ApproveExpansionRequest(expansion.ExpansionID, ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com"},
	}); err == nil {
		t.Fatal("expected terminal expansion request to reject later approval")
	}
}

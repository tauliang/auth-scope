package mission

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExpansionApprovalHonorsContainmentCreatedAfterRequest(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	expansion, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
	})
	if err != nil {
		t.Fatalf("CreateExpansionRequest: %v", err)
	}
	rule, err := service.CreateContainmentRule(ContainmentRule{
		TenantID:   "demo",
		TargetType: ContainmentTargetMission,
		TargetID:   mission.MissionRef,
	})
	if err != nil {
		t.Fatalf("CreateContainmentRule: %v", err)
	}
	if _, err := service.SubmitExpansionApproval(expansion.ExpansionID, SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); err == nil || !strings.Contains(err.Error(), "containment rule") {
		t.Fatalf("SubmitExpansionApproval during containment err = %v", err)
	}
	records, err := service.approvals.ListApprovalRecords(ApprovalTargetExpansion, expansion.ExpansionID)
	if err != nil {
		t.Fatalf("ListApprovalRecords: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("contained approval persisted records: %#v", records)
	}
	if _, err := service.ApproveExpansionRequest(expansion.ExpansionID, ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); err == nil || !strings.Contains(err.Error(), "containment rule") {
		t.Fatalf("ApproveExpansionRequest during containment err = %v", err)
	}
	stored, err := service.GetExpansionRequest(expansion.ExpansionID)
	if err != nil {
		t.Fatalf("GetExpansionRequest: %v", err)
	}
	if stored.Status != ExpansionStatusPending {
		t.Fatalf("expansion status = %s, want pending", stored.Status)
	}
	current, err := service.Introspect(mission.MissionRef)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	if current.Version != mission.MissionVersion {
		t.Fatalf("mission version = %d, want %d", current.Version, mission.MissionVersion)
	}
	if _, err := service.LiftContainmentRule(rule.RuleID, StateChangeRequest{Reason: "resolved"}); err != nil {
		t.Fatalf("LiftContainmentRule: %v", err)
	}
	if _, err := service.ApproveExpansionRequest(expansion.ExpansionID, ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); err != nil {
		t.Fatalf("ApproveExpansionRequest after lift: %v", err)
	}
}

func TestConcurrentExpansionApprovalsCommitOnlyOneMissionVersion(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	create := func(resourceID string) ExpansionRequest {
		expansion, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
			MissionVersionSeen: mission.MissionVersion,
			Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "crm_account", ID: resourceID}, Operation: "update"},
		})
		if err != nil {
			t.Fatalf("CreateExpansionRequest: %v", err)
		}
		return expansion
	}
	first := create("acme")
	second := create("globex")

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{first.ExpansionID, second.ExpansionID} {
		wg.Add(1)
		go func(expansionID string) {
			defer wg.Done()
			<-start
			_, err := service.ApproveExpansionRequestContext(context.Background(), expansionID, ExpansionDecisionRequest{
				Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
			})
			results <- err
		}(id)
	}
	close(start)
	wg.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful approvals = %d, want 1", succeeded)
	}
	current, err := service.Introspect(mission.MissionRef)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	if current.Version != mission.MissionVersion+1 {
		t.Fatalf("mission version = %d, want %d", current.Version, mission.MissionVersion+1)
	}
}

func TestGovernanceValidationAndForbiddenExpansion(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	forbidden, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "email", ID: "board"}, Operation: "send_external"},
	})
	if err != nil {
		t.Fatalf("CreateExpansionRequest forbidden action: %v", err)
	}
	if _, err := service.ApproveExpansionRequest(forbidden.ExpansionID, ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); err == nil {
		t.Fatal("expected approval of forbidden action to fail")
	}

	if _, err := service.CreateExpansionRequest("missing", CreateExpansionRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateExpansionRequest missing err = %v, want ErrNotFound", err)
	}
	if _, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		Requester: Actor{AgentInstanceID: "wrong", ClientID: "research-agent"},
		Action:    Action{Type: "tool_call", Resource: ActionResource{Type: "crm_account", ID: "acme"}, Operation: "update"},
	}); err == nil {
		t.Fatal("expected unauthorized requester to fail")
	}
	if _, err := service.GetExpansionRequest(""); err == nil {
		t.Fatal("expected empty expansion id to fail")
	}
}

func TestExpansionApprovalDetectsStaleMissionVersion(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	first, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
	})
	if err != nil {
		t.Fatalf("CreateExpansionRequest first: %v", err)
	}
	second, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "crm_account", ID: "acme"}, Operation: "update"},
	})
	if err != nil {
		t.Fatalf("CreateExpansionRequest second: %v", err)
	}
	if _, err := service.ApproveExpansionRequest(first.ExpansionID, ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); err != nil {
		t.Fatalf("ApproveExpansionRequest first: %v", err)
	}
	if _, err := service.ApproveExpansionRequest(second.ExpansionID, ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); err == nil {
		t.Fatal("expected stale expansion approval to fail")
	}
}

func TestDecisionArtifactVerificationEdgeCases(t *testing.T) {
	service := testService()

	empty := service.VerifyDecisionArtifactEvidence(VerifyDecisionArtifactRequest{})
	if empty.Valid || empty.Error == "" {
		t.Fatalf("empty artifact verify = %#v, want invalid error", empty)
	}

	payload := DecisionArtifactPayload{
		ArtifactID:     "artifact-test",
		MissionRef:     "mission-missing",
		MissionVersion: 1,
		PolicyVersion:  DefaultPolicyVersionID,
		EvidenceID:     "missing-evidence",
		Decision:       DecisionAllow,
		Actor:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:         Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		ContextHash:    "sha256:test",
		IssuedAt:       time.Now().UTC(),
	}
	artifact, err := SignDecisionArtifact(payload, service.artifactKey)
	if err != nil {
		t.Fatalf("SignDecisionArtifact: %v", err)
	}
	verified := service.VerifyDecisionArtifactEvidence(VerifyDecisionArtifactRequest{DecisionArtifact: artifact})
	if !verified.Valid || verified.Evidence != nil {
		t.Fatalf("verify missing evidence = %#v, want valid without evidence", verified)
	}
}

func TestToolContractValidationAndOperationParam(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	invalidContracts := []ToolContract{
		{},
		{ToolName: "missing-resource"},
		{ToolName: "missing-id", ResourceType: "drive_folder"},
		{ToolName: "missing-operation", ResourceType: "drive_folder", ResourceID: "board"},
	}
	for _, contract := range invalidContracts {
		if _, err := service.RegisterToolContract(contract); err == nil {
			t.Fatalf("expected invalid contract to fail: %#v", contract)
		}
	}
	if _, err := service.GetToolContract(""); err == nil {
		t.Fatal("expected empty tool name to fail")
	}
	if _, err := service.AuthorizeToolCall(AuthorizeToolCallRequest{ToolName: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AuthorizeToolCall missing contract err = %v, want ErrNotFound", err)
	}

	if _, err := service.RegisterToolContract(ToolContract{
		ToolName:       "drive.dynamic",
		ResourceType:   "drive_folder",
		ResourceID:     "board",
		OperationParam: "operation",
	}); err != nil {
		t.Fatalf("RegisterToolContract dynamic: %v", err)
	}
	allowed, err := service.AuthorizeToolCall(AuthorizeToolCallRequest{
		MissionRef:         mission.MissionRef,
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		ToolName:           "drive.dynamic",
		Arguments:          map[string]any{"operation": "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall dynamic: %v", err)
	}
	if allowed.Decision != DecisionAllow {
		t.Fatalf("dynamic tool decision = %s, want allow", allowed.Decision)
	}
}

func TestGovernanceAuthorityHelpers(t *testing.T) {
	empty := authorityForAction(Action{})
	if len(empty.Resources) != 0 {
		t.Fatalf("empty authorityForAction = %#v", empty)
	}

	merged := mergeAuthority(AuthorityRegion{
		Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
	}, AuthorityRegion{
		Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"write_draft"}, Constraints: map[string]any{"max_bytes": 1024}}},
	})
	if len(merged.Resources) != 1 || len(merged.Resources[0].Actions) != 2 || merged.Resources[0].Constraints["max_bytes"] != 1024 {
		t.Fatalf("mergeAuthority = %#v", merged)
	}
}

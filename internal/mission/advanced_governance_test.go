package mission

import (
	"errors"
	"testing"
	"time"
)

func TestProjectionLifecycleSignsVerifiesAndRevokes(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	projection, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Type:               ProjectionTypeMCPContext,
		Claims:             map[string]any{"scope": "board.read"},
		TTLSeconds:         120,
	})
	if err != nil {
		t.Fatalf("CreateProjection: %v", err)
	}
	if projection.Token == "" || projection.Status != ProjectionStatusActive {
		t.Fatalf("unexpected projection: %#v", projection)
	}

	verified := service.VerifyProjection(VerifyProjectionRequest{Token: projection.Token})
	if !verified.Valid || verified.Projection == nil || verified.Payload.Type != ProjectionTypeMCPContext {
		t.Fatalf("VerifyProjection = %#v, want valid MCP projection", verified)
	}
	status, err := service.GetProjectionStatus(projection.ProjectionID)
	if err != nil {
		t.Fatalf("GetProjectionStatus: %v", err)
	}
	if status.Status != ProjectionStatusActive {
		t.Fatalf("status = %s, want active", status.Status)
	}

	revoked, err := service.RevokeProjection(projection.ProjectionID, StateChangeRequest{Reason: "gateway disconnected"})
	if err != nil {
		t.Fatalf("RevokeProjection: %v", err)
	}
	if revoked.Status != ProjectionStatusRevoked || revoked.RevokedAt.IsZero() {
		t.Fatalf("unexpected revoke status: %#v", revoked)
	}
	verified = service.VerifyProjection(VerifyProjectionRequest{Token: projection.Token})
	if verified.Valid || verified.Error == "" {
		t.Fatalf("revoked projection should be invalid, got %#v", verified)
	}
}

func TestProjectionValidationAndExpiry(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, fixedClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)})
	mission := approveTestMission(t, service)

	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Type:               "unknown",
	}); err == nil {
		t.Fatal("expected unsupported projection type")
	}
	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: 1,
		Actor:              Actor{AgentInstanceID: "wrong", ClientID: "research-agent"},
		Type:               ProjectionTypeOAuthClaims,
	}); err == nil {
		t.Fatal("expected unauthorized actor error")
	}
	projection, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Type:               ProjectionTypeOAuthClaims,
		TTLSeconds:         1,
	})
	if err != nil {
		t.Fatalf("CreateProjection short TTL: %v", err)
	}
	later := NewService(store, fixedClock{now: time.Date(2026, 7, 14, 12, 0, 2, 0, time.UTC)})
	verified := later.VerifyProjection(VerifyProjectionRequest{Token: projection.Token})
	if verified.Valid || verified.Error != "projection expired" {
		t.Fatalf("expected expired projection, got %#v", verified)
	}
}

func TestMissionLeaseRefreshInvalidatesOnStateChange(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	lease, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TTLSeconds:         30,
	})
	if err != nil {
		t.Fatalf("CreateMissionLease: %v", err)
	}
	if lease.Decision != DecisionAllow || lease.LeaseID == "" {
		t.Fatalf("unexpected lease: %#v", lease)
	}
	refreshed, err := service.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{
		Actor:      Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TTLSeconds: 30,
	})
	if err != nil {
		t.Fatalf("RefreshMissionLease: %v", err)
	}
	if refreshed.Decision != DecisionAllow {
		t.Fatalf("refresh decision = %s, want allow", refreshed.Decision)
	}
	if _, err := service.Revoke(mission.MissionRef, StateChangeRequest{Reason: "principal revoked"}); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	denied, err := service.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{
		Actor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("RefreshMissionLease after revoke: %v", err)
	}
	if denied.Decision != DecisionDeny {
		t.Fatalf("refresh after revoke decision = %s, want deny", denied.Decision)
	}
}

func TestMissionLeaseValidationBranches(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, fixedClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)})
	mission := approveTestMission(t, service)

	denied, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		Actor: Actor{AgentInstanceID: "wrong", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("CreateMissionLease unauthorized: %v", err)
	}
	if denied.Decision != DecisionDeny {
		t.Fatalf("unauthorized lease = %#v, want deny", denied)
	}
	lease, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TTLSeconds:         1,
	})
	if err != nil {
		t.Fatalf("CreateMissionLease: %v", err)
	}
	later := NewService(store, fixedClock{now: time.Date(2026, 7, 14, 12, 0, 2, 0, time.UTC)})
	expired, err := later.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{
		Actor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("RefreshMissionLease expired: %v", err)
	}
	if expired.Decision != DecisionDeny || expired.HumanReason != "lease expired" {
		t.Fatalf("expired lease response = %#v", expired)
	}
}

func TestApprovalRuleRequiresMultipleExpansionApprovals(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	rule, err := service.CreateApprovalRule(ApprovalRule{
		TenantID:          "demo",
		ResourceType:      "slack_channel",
		ResourceID:        "board",
		Operation:         "post_update",
		RiskLevel:         "high",
		RequiredApprovals: 2,
		AllowedIssuers:    []string{"https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("CreateApprovalRule: %v", err)
	}
	rules, err := service.ListApprovalRules()
	if err != nil {
		t.Fatalf("ListApprovalRules: %v", err)
	}
	if len(rules) != 1 || rules[0].RuleID != rule.RuleID {
		t.Fatalf("ListApprovalRules = %#v", rules)
	}

	outOfScope, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "high", "reversible": false},
	})
	if err != nil {
		t.Fatalf("Evaluate expansion: %v", err)
	}
	expansionID, _ := outOfScope.Constraints["expansion_request_id"].(string)
	if expansionID == "" {
		t.Fatalf("expected expansion id: %#v", outOfScope)
	}
	if _, err := service.ApproveExpansionRequest(expansionID, ExpansionDecisionRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); err == nil {
		t.Fatal("expected direct approval to require approval submissions")
	}

	first, err := service.SubmitExpansionApproval(expansionID, SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		Reason:   "looks good",
	})
	if err != nil {
		t.Fatalf("SubmitExpansionApproval first: %v", err)
	}
	if first.Status != ExpansionStatusPending || first.ApprovalsReceived != 1 || first.ApprovalsRequired != 2 {
		t.Fatalf("unexpected first approval response: %#v", first)
	}
	if _, err := service.SubmitExpansionApproval(expansionID, SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate approval err = %v, want ErrConflict", err)
	}
	if _, err := service.SubmitExpansionApproval(expansionID, SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "mallory@example.com", Issuer: "https://evil.example.com"},
	}); err == nil {
		t.Fatal("expected issuer rejected by rule")
	}

	second, err := service.SubmitExpansionApproval(expansionID, SubmitExpansionApprovalRequest{
		Approver: Principal{Subject: "bob@example.com", Issuer: "https://idp.example.com"},
		Reason:   "approved",
	})
	if err != nil {
		t.Fatalf("SubmitExpansionApproval second: %v", err)
	}
	if second.Status != ExpansionStatusApproved || second.ApprovalsReceived != 2 || second.MissionVersion != mission.MissionVersion+1 {
		t.Fatalf("unexpected final approval response: %#v", second)
	}
}

func TestProjectionTokenValidation(t *testing.T) {
	service := testService()
	if _, err := SignProjectionToken(ProjectionPayload{}, nil); err == nil {
		t.Fatal("expected missing projection key error")
	}
	if _, err := VerifyProjectionToken("not-a-token", service.artifactKey); err == nil {
		t.Fatal("expected malformed token error")
	}
	if got := service.VerifyProjection(VerifyProjectionRequest{}); got.Valid || got.Error == "" {
		t.Fatalf("VerifyProjection empty token = %#v", got)
	}
}

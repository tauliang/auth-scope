package mission

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
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

func TestProjectionAndLeaseFailureBranches(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	service.authorityGuard = fixedAuthorityGuard{err: errors.New("guard unavailable")}
	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
	}); err == nil {
		t.Fatal("expected projection containment guard error")
	}
	if _, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
	}); err == nil {
		t.Fatal("expected lease containment guard error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	service.authorityGuard = fixedAuthorityGuard{rule: ContainmentRule{RuleID: "ctr_block"}, ok: true}
	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
	}); err == nil {
		t.Fatal("expected projection blocked by containment")
	}

	service = testService()
	mission = approveTestMission(t, service)
	service.projections = failingProjectionStore{ProjectionStore: service.projections, saveProjectionErr: errors.New("save projection")}
	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
	}); err == nil {
		t.Fatal("expected projection save error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	service.projections = failingProjectionStore{ProjectionStore: service.projections, saveLeaseErr: errors.New("save lease")}
	if _, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
	}); err == nil {
		t.Fatal("expected lease save error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	service.artifactKey = nil
	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
	}); err == nil {
		t.Fatal("expected projection signing error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
		Claims:             map[string]any{"bad": func() {}},
	}); err == nil {
		t.Fatal("expected projection payload marshal error")
	}
}

func TestProjectionVerificationAndRevocationFailureBranches(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	if _, err := service.GetProjectionStatus("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjectionStatus missing err = %v, want ErrNotFound", err)
	}
	if _, err := service.RevokeProjection("missing", StateChangeRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RevokeProjection missing err = %v, want ErrNotFound", err)
	}

	projection, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
	})
	if err != nil {
		t.Fatalf("CreateProjection: %v", err)
	}

	token, err := SignProjectionToken(ProjectionPayload{ProjectionID: "missing"}, service.artifactKey)
	if err != nil {
		t.Fatalf("SignProjectionToken: %v", err)
	}
	if got := service.VerifyProjection(VerifyProjectionRequest{Token: token}); got.Valid || !strings.Contains(got.Error, ErrNotFound.Error()) {
		t.Fatalf("expected missing projection verification failure, got %#v", got)
	}

	service.authorityGuard = fixedAuthorityGuard{err: errors.New("guard unavailable")}
	if got := service.VerifyProjection(VerifyProjectionRequest{Token: projection.Token}); got.Valid || !strings.Contains(got.Error, "guard unavailable") {
		t.Fatalf("expected guard verification error, got %#v", got)
	}
	service.authorityGuard = fixedAuthorityGuard{rule: ContainmentRule{RuleID: "ctr_block"}, ok: true}
	if got := service.VerifyProjection(VerifyProjectionRequest{Token: projection.Token}); got.Valid || !strings.Contains(got.Error, "ctr_block") {
		t.Fatalf("expected containment verification error, got %#v", got)
	}

	service.authorityGuard = NewContainmentGuard(containmentGuardStores{MissionStore: service.missions, GovernanceReadStore: service.governanceReads})
	service.projections = failingProjectionStore{ProjectionStore: service.projections, updateProjectionErr: errors.New("update projection")}
	if _, err := service.RevokeProjection(projection.ProjectionID, StateChangeRequest{}); err == nil {
		t.Fatal("expected revoke update error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	projection, err = service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
	})
	if err != nil {
		t.Fatalf("CreateProjection: %v", err)
	}
	if _, err := service.RevokeProjection(projection.ProjectionID, StateChangeRequest{}); err != nil {
		t.Fatalf("RevokeProjection: %v", err)
	}
	again, err := service.RevokeProjection(projection.ProjectionID, StateChangeRequest{})
	if err != nil {
		t.Fatalf("RevokeProjection again: %v", err)
	}
	if again.Status != ProjectionStatusRevoked {
		t.Fatalf("second revoke status = %s, want revoked", again.Status)
	}
}

func TestLeaseRefreshFailureBranches(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	if _, err := service.RefreshMissionLease("missing", RefreshLeaseRequest{Actor: actor}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RefreshMissionLease missing err = %v, want ErrNotFound", err)
	}

	if err := service.projections.SaveMissionLease(MissionLease{
		LeaseID:        "lease-orphan",
		MissionRef:     "missing",
		MissionVersion: 1,
		Actor:          actor,
		ExpiresAt:      service.clock.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("SaveMissionLease orphan: %v", err)
	}
	if _, err := service.RefreshMissionLease("lease-orphan", RefreshLeaseRequest{Actor: actor}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RefreshMissionLease orphan err = %v, want ErrNotFound", err)
	}

	lease, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
	})
	if err != nil {
		t.Fatalf("CreateMissionLease: %v", err)
	}
	mismatched, err := service.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{Actor: Actor{AgentInstanceID: "inst_123", ClientID: "other"}})
	if err != nil {
		t.Fatalf("RefreshMissionLease mismatch: %v", err)
	}
	if mismatched.Decision != DecisionDeny || mismatched.HumanReason != "lease actor mismatch" {
		t.Fatalf("actor mismatch lease response = %#v", mismatched)
	}

	storedMission, err := service.Introspect(mission.MissionRef)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	storedMission.Version++
	if err := service.missions.UpdateMission(storedMission); err != nil {
		t.Fatalf("UpdateMission: %v", err)
	}
	stale, err := service.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{Actor: actor})
	if err != nil {
		t.Fatalf("RefreshMissionLease stale: %v", err)
	}
	if stale.Decision != DecisionDeny || stale.HumanReason != "mission version stale" {
		t.Fatalf("stale lease response = %#v", stale)
	}
	if err := service.projections.SaveMissionLease(MissionLease{
		LeaseID:        "lease-ahead",
		MissionRef:     mission.MissionRef,
		MissionVersion: storedMission.Version + 1,
		Actor:          actor,
		ExpiresAt:      service.clock.Now().Add(time.Minute),
	}); err != nil {
		t.Fatalf("SaveMissionLease ahead: %v", err)
	}
	changed, err := service.RefreshMissionLease("lease-ahead", RefreshLeaseRequest{Actor: actor})
	if err != nil {
		t.Fatalf("RefreshMissionLease changed: %v", err)
	}
	if changed.Decision != DecisionDeny || changed.HumanReason != "mission version changed" {
		t.Fatalf("changed lease response = %#v", changed)
	}

	service = testService()
	mission = approveTestMission(t, service)
	lease, err = service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
	})
	if err != nil {
		t.Fatalf("CreateMissionLease: %v", err)
	}
	service.authorityGuard = fixedAuthorityGuard{err: errors.New("guard unavailable")}
	if _, err := service.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{Actor: actor}); err == nil {
		t.Fatal("expected lease refresh guard error")
	}

	service.authorityGuard = fixedAuthorityGuard{rule: ContainmentRule{RuleID: "ctr_block"}, ok: true}
	blocked, err := service.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{Actor: actor})
	if err != nil {
		t.Fatalf("RefreshMissionLease blocked: %v", err)
	}
	if blocked.Decision != DecisionDeny || !strings.Contains(blocked.HumanReason, "ctr_block") {
		t.Fatalf("blocked lease response = %#v", blocked)
	}

	service.authorityGuard = NewContainmentGuard(containmentGuardStores{MissionStore: service.missions, GovernanceReadStore: service.governanceReads})
	service.projections = failingProjectionStore{ProjectionStore: service.projections, updateLeaseErr: errors.New("update lease")}
	if _, err := service.RefreshMissionLease(lease.LeaseID, RefreshLeaseRequest{Actor: actor}); err == nil {
		t.Fatal("expected lease update error")
	}
}

func TestApprovalSubmissionFailureBranches(t *testing.T) {
	service := testService()
	if _, err := service.CreateApprovalRule(ApprovalRule{AppliesTo: "mission"}); err == nil {
		t.Fatal("expected unsupported approval rule target")
	}
	service.approvals = failingApprovalStore{ApprovalStore: service.approvals, saveRuleErr: errors.New("save rule")}
	if _, err := service.CreateApprovalRule(ApprovalRule{}); err == nil {
		t.Fatal("expected approval rule save error")
	}

	service = testService()
	if _, err := service.SubmitExpansionApproval("missing", SubmitExpansionApprovalRequest{}); err == nil {
		t.Fatal("expected missing approver to fail")
	}
	if err := service.governance.SaveExpansionRequest(ExpansionRequest{
		ExpansionID:        "exp-orphan",
		MissionRef:         "missing",
		MissionVersionSeen: 1,
		TenantID:           "demo",
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "slack_channel", ID: "board", Actions: []string{"post_update"}}}},
		Status:             ExpansionStatusPending,
	}); err != nil {
		t.Fatalf("SaveExpansionRequest orphan: %v", err)
	}
	if _, err := service.SubmitExpansionApproval("exp-orphan", SubmitExpansionApprovalRequest{Approver: Principal{Subject: "alice@example.com"}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SubmitExpansionApproval orphan err = %v, want ErrNotFound", err)
	}

	service = testService()
	mission := approveTestMission(t, service)
	expansion := createPendingExpansionForTest(t, service, mission)
	service.approvals = failingApprovalStore{ApprovalStore: service.approvals, listRulesErr: errors.New("list rules")}
	if _, err := service.SubmitExpansionApproval(expansion.ExpansionID, SubmitExpansionApprovalRequest{Approver: Principal{Subject: "alice@example.com"}}); err == nil {
		t.Fatal("expected approval rule list error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	expansion = createPendingExpansionForTest(t, service, mission)
	service.approvals = failingApprovalStore{ApprovalStore: service.approvals, saveRecordErr: errors.New("save record")}
	if _, err := service.SubmitExpansionApproval(expansion.ExpansionID, SubmitExpansionApprovalRequest{Approver: Principal{Subject: "alice@example.com"}}); err == nil {
		t.Fatal("expected approval record save error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	expansion = createPendingExpansionForTest(t, service, mission)
	service.approvals = failingApprovalStore{ApprovalStore: service.approvals, listRecordsErr: errors.New("list records")}
	if _, err := service.SubmitExpansionApproval(expansion.ExpansionID, SubmitExpansionApprovalRequest{Approver: Principal{Subject: "alice@example.com"}}); err == nil {
		t.Fatal("expected approval record list error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	expansion = createPendingExpansionForTest(t, service, mission)
	service.expansionDecisions = failingExpansionDecisionStore{ExpansionDecisionStore: service.expansionDecisions, err: errors.New("commit expansion")}
	if _, err := service.SubmitExpansionApproval(expansion.ExpansionID, SubmitExpansionApprovalRequest{Approver: Principal{Subject: "alice@example.com"}}); err == nil {
		t.Fatal("expected approval commit error")
	}

	service = testService()
	mission = approveTestMission(t, service)
	expansion = createPendingExpansionForTest(t, service, mission)
	approved, err := service.SubmitExpansionApproval(expansion.ExpansionID, SubmitExpansionApprovalRequest{Approver: Principal{Subject: "alice@example.com"}})
	if err != nil {
		t.Fatalf("SubmitExpansionApproval: %v", err)
	}
	if approved.Status != ExpansionStatusApproved {
		t.Fatalf("approval status = %s, want approved", approved.Status)
	}
	if _, err := service.SubmitExpansionApproval(expansion.ExpansionID, SubmitExpansionApprovalRequest{Approver: Principal{Subject: "bob@example.com"}}); err == nil {
		t.Fatal("expected already-approved expansion to reject approval")
	}
}

func TestAdvancedGovernanceHelperBranches(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	mission := Mission{
		State:   StateActive,
		Version: 3,
		Agent:   Agent{InstanceID: "inst_123", ClientID: "research-agent"},
		Lifecycle: Lifecycle{
			NotBefore: now.Add(-time.Hour),
			ExpiresAt: now.Add(time.Hour),
		},
	}
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}
	if err := ensureMissionUsableForActor(mission, actor, 3, now); err != nil {
		t.Fatalf("ensureMissionUsableForActor valid: %v", err)
	}
	expiredMission := mission
	expiredMission.Lifecycle.ExpiresAt = now.Add(-time.Second)
	if err := ensureMissionUsableForActor(expiredMission, actor, 3, now); err == nil {
		t.Fatal("expected expired mission error")
	}
	inactiveMission := mission
	inactiveMission.State = StateSuspended
	if err := ensureMissionUsableForActor(inactiveMission, actor, 3, now); err == nil {
		t.Fatal("expected inactive mission error")
	}
	if err := ensureMissionUsableForActor(mission, Actor{AgentInstanceID: "wrong", ClientID: "research-agent"}, 3, now); err == nil {
		t.Fatal("expected unauthorized actor error")
	}
	if err := ensureMissionUsableForActor(mission, actor, 2, now); err == nil {
		t.Fatal("expected stale version error")
	}

	if !boundedExpiry(now, 0, defaultProjectionTTLSeconds, time.Time{}).Equal(now.Add(defaultProjectionTTLSeconds * time.Second)) {
		t.Fatal("default projection expiry was not applied")
	}
	if !boundedExpiry(now, maxLeaseTTLSeconds+100, defaultLeaseTTLSeconds, time.Time{}).Equal(now.Add(maxLeaseTTLSeconds * time.Second)) {
		t.Fatal("lease expiry should be capped at max lease TTL")
	}
	missionExpiresAt := now.Add(30 * time.Second)
	if !boundedExpiry(now, 120, defaultLeaseTTLSeconds, missionExpiresAt).Equal(missionExpiresAt) {
		t.Fatal("mission expiry should cap lease expiry")
	}
	if got := projectionStatusResponse(Projection{ProjectionID: "projection-1", Status: ProjectionStatusActive, ExpiresAt: now.Add(-time.Second)}, now); got.Status != ProjectionStatusExpired {
		t.Fatalf("projection status = %s, want expired", got.Status)
	}
	if !actorsEqual(actor, Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}) {
		t.Fatal("expected actorsEqual to match identical actors")
	}
	if actorsEqual(actor, Actor{AgentInstanceID: "inst_123", ClientID: "other"}) {
		t.Fatal("unexpected actorsEqual match")
	}
}

func TestProjectionTokenMalformedBranches(t *testing.T) {
	key := []byte("projection-key")
	if _, err := SignProjectionToken(ProjectionPayload{Claims: map[string]any{"bad": func() {}}}, key); err == nil {
		t.Fatal("expected projection payload marshal error")
	}
	if _, err := VerifyProjectionToken("a.b.not-base64!", key); err == nil {
		t.Fatal("expected signature decode error")
	}
	token := signProjectionSegmentsForTest(
		base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"mission-projection+jws","alg":"HS256","kid":"local"}`)),
		base64.RawURLEncoding.EncodeToString([]byte(`{}`)),
		[]byte("wrong-key"),
	)
	if _, err := VerifyProjectionToken(token, key); err == nil {
		t.Fatal("expected projection signature mismatch")
	}
	token = signProjectionSegmentsForTest("not-base64!", base64.RawURLEncoding.EncodeToString([]byte(`{}`)), key)
	if _, err := VerifyProjectionToken(token, key); err == nil {
		t.Fatal("expected header decode error")
	}
	token = signProjectionSegmentsForTest(base64.RawURLEncoding.EncodeToString([]byte(`{`)), base64.RawURLEncoding.EncodeToString([]byte(`{}`)), key)
	if _, err := VerifyProjectionToken(token, key); err == nil {
		t.Fatal("expected header unmarshal error")
	}
	token = signProjectionSegmentsForTest(base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"other","alg":"HS256","kid":"local"}`)), base64.RawURLEncoding.EncodeToString([]byte(`{}`)), key)
	if _, err := VerifyProjectionToken(token, key); err == nil {
		t.Fatal("expected unsupported header error")
	}
	token = signProjectionSegmentsForTest(base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"mission-projection+jws","alg":"HS256","kid":"local"}`)), "not-base64!", key)
	if _, err := VerifyProjectionToken(token, key); err == nil {
		t.Fatal("expected payload decode error")
	}
	token = signProjectionSegmentsForTest(base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"mission-projection+jws","alg":"HS256","kid":"local"}`)), base64.RawURLEncoding.EncodeToString([]byte(`{`)), key)
	if _, err := VerifyProjectionToken(token, key); err == nil {
		t.Fatal("expected payload unmarshal error")
	}
}

func createPendingExpansionForTest(t *testing.T, service *Service, mission ApproveProposalResponse) ExpansionRequest {
	t.Helper()
	expansion, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "slack_channel", ID: "board", Actions: []string{"post_update"}}}},
		Justification:      "unit test expansion",
	})
	if err != nil {
		t.Fatalf("CreateExpansionRequest: %v", err)
	}
	return expansion
}

func signProjectionSegmentsForTest(headerSegment string, payloadSegment string, key []byte) string {
	signingInput := headerSegment + "." + payloadSegment
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type fixedAuthorityGuard struct {
	rule ContainmentRule
	ok   bool
	err  error
}

func (g fixedAuthorityGuard) Check(context.Context, AuthorityOperation) (ContainmentRule, bool, error) {
	return g.rule, g.ok, g.err
}

type failingProjectionStore struct {
	ProjectionStore
	saveProjectionErr   error
	updateProjectionErr error
	saveLeaseErr        error
	updateLeaseErr      error
}

func (s failingProjectionStore) SaveProjection(projection Projection) error {
	if s.saveProjectionErr != nil {
		return s.saveProjectionErr
	}
	return s.ProjectionStore.SaveProjection(projection)
}

func (s failingProjectionStore) UpdateProjection(projection Projection) error {
	if s.updateProjectionErr != nil {
		return s.updateProjectionErr
	}
	return s.ProjectionStore.UpdateProjection(projection)
}

func (s failingProjectionStore) SaveMissionLease(lease MissionLease) error {
	if s.saveLeaseErr != nil {
		return s.saveLeaseErr
	}
	return s.ProjectionStore.SaveMissionLease(lease)
}

func (s failingProjectionStore) UpdateMissionLease(lease MissionLease) error {
	if s.updateLeaseErr != nil {
		return s.updateLeaseErr
	}
	return s.ProjectionStore.UpdateMissionLease(lease)
}

type failingApprovalStore struct {
	ApprovalStore
	saveRuleErr    error
	listRulesErr   error
	saveRecordErr  error
	listRecordsErr error
}

func (s failingApprovalStore) SaveApprovalRule(rule ApprovalRule) error {
	if s.saveRuleErr != nil {
		return s.saveRuleErr
	}
	return s.ApprovalStore.SaveApprovalRule(rule)
}

func (s failingApprovalStore) ListApprovalRules() ([]ApprovalRule, error) {
	if s.listRulesErr != nil {
		return nil, s.listRulesErr
	}
	return s.ApprovalStore.ListApprovalRules()
}

func (s failingApprovalStore) SaveApprovalRecord(record ApprovalRecord) error {
	if s.saveRecordErr != nil {
		return s.saveRecordErr
	}
	return s.ApprovalStore.SaveApprovalRecord(record)
}

func (s failingApprovalStore) ListApprovalRecords(targetType string, targetID string) ([]ApprovalRecord, error) {
	if s.listRecordsErr != nil {
		return nil, s.listRecordsErr
	}
	return s.ApprovalStore.ListApprovalRecords(targetType, targetID)
}

type failingExpansionDecisionStore struct {
	ExpansionDecisionStore
	err error
}

func (s failingExpansionDecisionStore) CommitExpansionDecision(ctx context.Context, commit ExpansionDecisionCommit) error {
	if s.err != nil {
		return s.err
	}
	return s.ExpansionDecisionStore.CommitExpansionDecision(ctx, commit)
}

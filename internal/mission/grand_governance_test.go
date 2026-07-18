package mission

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGovernanceReadModelsHonorCanceledContext(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.MissionLineageContext(ctx, mission.MissionRef); !errors.Is(err, context.Canceled) {
		t.Fatalf("MissionLineageContext err = %v, want context.Canceled", err)
	}
	if _, err := service.AgentLineageContext(ctx, "inst_123"); !errors.Is(err, context.Canceled) {
		t.Fatalf("AgentLineageContext err = %v, want context.Canceled", err)
	}
}

func TestAuthorityNegotiationStatuses(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	accepted, err := service.CreateAuthorityNegotiation(mission.MissionRef, CreateAuthorityNegotiationRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}}},
	})
	if err != nil {
		t.Fatalf("CreateAuthorityNegotiation accepted: %v", err)
	}
	if accepted.Status != NegotiationStatusAccepted || len(accepted.ProposedAuthority.Resources) != 1 || len(accepted.DeniedAuthority.Resources) != 0 {
		t.Fatalf("unexpected accepted negotiation: %#v", accepted)
	}

	counter, err := service.CreateAuthorityNegotiation(mission.MissionRef, CreateAuthorityNegotiationRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "delete"}}}},
	})
	if err != nil {
		t.Fatalf("CreateAuthorityNegotiation counter: %v", err)
	}
	if counter.Status != NegotiationStatusCounteroffered {
		t.Fatalf("counter status = %s, want %s", counter.Status, NegotiationStatusCounteroffered)
	}
	if got := counter.ProposedAuthority.Resources[0].Actions; len(got) != 1 || got[0] != "read" {
		t.Fatalf("counter proposed actions = %#v, want read", got)
	}
	if got := counter.DeniedAuthority.Resources[0].Actions; len(got) != 1 || got[0] != "delete" {
		t.Fatalf("counter denied actions = %#v, want delete", got)
	}

	needsApproval, err := service.CreateAuthorityNegotiation(mission.MissionRef, CreateAuthorityNegotiationRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "slack_channel", ID: "board", Actions: []string{"post_update"}}}},
		Context:            map[string]any{"risk": "high", "reversible": false},
	})
	if err != nil {
		t.Fatalf("CreateAuthorityNegotiation approval: %v", err)
	}
	if needsApproval.Status != NegotiationStatusRequiresApproval {
		t.Fatalf("approval status = %s, want %s", needsApproval.Status, NegotiationStatusRequiresApproval)
	}

	denied, err := service.CreateAuthorityNegotiation(mission.MissionRef, CreateAuthorityNegotiationRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "slack_channel", ID: "board", Actions: []string{"post_update"}}}},
		Context:            map[string]any{"risk": "low", "reversible": true},
	})
	if err != nil {
		t.Fatalf("CreateAuthorityNegotiation denied: %v", err)
	}
	if denied.Status != NegotiationStatusDenied {
		t.Fatalf("denied status = %s, want %s", denied.Status, NegotiationStatusDenied)
	}
}

func TestContainmentDeniesEvaluationUntilLifted(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	rule, err := service.CreateContainmentRule(ContainmentRule{
		TenantID:   "demo",
		TargetType: ContainmentTargetResource,
		TargetID:   "drive_folder:board",
		Reason:     "suspect data boundary",
		CreatedBy:  Principal{Subject: "security@example.com", Issuer: "https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("CreateContainmentRule: %v", err)
	}

	denied, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate contained: %v", err)
	}
	if denied.Decision != DecisionDeny || !contains(denied.ReasonCodes, "CONTAINMENT_ACTIVE") {
		t.Fatalf("expected containment denial, got %#v", denied)
	}

	lifted, err := service.LiftContainmentRule(rule.RuleID, StateChangeRequest{Reason: "cleared"})
	if err != nil {
		t.Fatalf("LiftContainmentRule: %v", err)
	}
	if lifted.Status != ContainmentStatusLifted || lifted.LiftedAt.IsZero() {
		t.Fatalf("unexpected lifted rule: %#v", lifted)
	}

	allowed, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate after lift: %v", err)
	}
	if allowed.Decision != DecisionAllow {
		t.Fatalf("decision after lift = %s, want %s", allowed.Decision, DecisionAllow)
	}
}

func TestContainmentBlocksProjectionVerificationAndLeases(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	projection, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
		TTLSeconds:         60,
	})
	if err != nil {
		t.Fatalf("CreateProjection: %v", err)
	}
	if verified := service.VerifyProjection(VerifyProjectionRequest{Token: projection.Token}); !verified.Valid {
		t.Fatalf("expected projection valid before containment: %#v", verified)
	}

	rule, err := service.CreateContainmentRule(ContainmentRule{
		TenantID:   "demo",
		TargetType: ContainmentTargetAgent,
		TargetID:   "inst_123",
		Reason:     "compromised runtime",
	})
	if err != nil {
		t.Fatalf("CreateContainmentRule: %v", err)
	}

	verified := service.VerifyProjection(VerifyProjectionRequest{Token: projection.Token})
	if verified.Valid || !strings.Contains(verified.Error, rule.RuleID) {
		t.Fatalf("expected contained projection verification failure, got %#v", verified)
	}

	lease, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		TTLSeconds:         30,
	})
	if err != nil {
		t.Fatalf("CreateMissionLease: %v", err)
	}
	if lease.Decision != DecisionDeny || !strings.Contains(lease.HumanReason, rule.RuleID) {
		t.Fatalf("expected contained lease denial, got %#v", lease)
	}

	if _, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeMCPContext,
	}); err == nil {
		t.Fatal("expected projection issuance to fail while agent is contained")
	}
}

func TestBlastRadiusAndLineageGraphs(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	if _, err := service.RegisterToolContract(ToolContract{ToolName: "drive.read", ResourceType: "drive_folder", ResourceIDParam: "folder_id", Operation: "read"}); err != nil {
		t.Fatalf("RegisterToolContract: %v", err)
	}
	projection, err := service.CreateProjection(mission.MissionRef, CreateProjectionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		Type:               ProjectionTypeToolGatewayToken,
		TTLSeconds:         60,
	})
	if err != nil {
		t.Fatalf("CreateProjection: %v", err)
	}
	lease, err := service.CreateMissionLease(mission.MissionRef, CreateLeaseRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              actor,
		TTLSeconds:         30,
	})
	if err != nil {
		t.Fatalf("CreateMissionLease: %v", err)
	}
	expansion, err := service.CreateExpansionRequest(mission.MissionRef, CreateExpansionRequest{
		MissionVersionSeen: mission.MissionVersion,
		Requester:          actor,
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		Justification:      "manual accountability test",
	})
	if err != nil {
		t.Fatalf("CreateExpansionRequest: %v", err)
	}
	child, err := service.Delegate(mission.MissionRef, DelegationRequest{
		DelegatingActor: actor,
		TargetAgent:     Agent{Provider: "https://agents.example.com", ClientID: "chart-agent", InstanceID: "inst_child"},
		RequestedAuthority: AuthorityRegion{
			Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
			ForbiddenActions: []string{"send_external"},
		},
		Delegation: DelegationPolicy{Permitted: false, CascadeRevocation: true},
	})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}

	rule, err := service.CreateContainmentRule(ContainmentRule{
		TenantID:   "demo",
		TargetType: ContainmentTargetMission,
		TargetID:   mission.MissionRef,
		Reason:     "incident review",
	})
	if err != nil {
		t.Fatalf("CreateContainmentRule: %v", err)
	}
	radius, err := service.ContainmentBlastRadius(rule.RuleID)
	if err != nil {
		t.Fatalf("ContainmentBlastRadius: %v", err)
	}
	if len(radius.Missions) != 1 || radius.Missions[0].MissionRef != mission.MissionRef {
		t.Fatalf("radius missions = %#v", radius.Missions)
	}
	if len(radius.Projections) != 1 || radius.Projections[0].ProjectionID != projection.ProjectionID {
		t.Fatalf("radius projections = %#v", radius.Projections)
	}
	if len(radius.Leases) != 1 || radius.Leases[0].LeaseID != lease.LeaseID {
		t.Fatalf("radius leases = %#v", radius.Leases)
	}
	if len(radius.ExpansionRequests) != 1 || radius.ExpansionRequests[0].ExpansionID != expansion.ExpansionID {
		t.Fatalf("radius expansions = %#v", radius.ExpansionRequests)
	}

	lineage, err := service.MissionLineage(mission.MissionRef)
	if err != nil {
		t.Fatalf("MissionLineage: %v", err)
	}
	if !lineageHasNode(lineage, "mission:"+mission.MissionRef) || !lineageHasNode(lineage, "mission:"+child.ChildMissionRef) {
		t.Fatalf("lineage missing mission nodes: %#v", lineage.Nodes)
	}
	if !lineageHasEdge(lineage, "mission:"+mission.MissionRef, "mission:"+child.ChildMissionRef, "delegated") {
		t.Fatalf("lineage missing delegation edge: %#v", lineage.Edges)
	}
	if !lineageHasNode(lineage, "projection:"+projection.ProjectionID) || !lineageHasNode(lineage, "lease:"+lease.LeaseID) || !lineageHasNode(lineage, "expansion:"+expansion.ExpansionID) {
		t.Fatalf("lineage missing artifact nodes: %#v", lineage.Nodes)
	}

	agentLineage, err := service.AgentLineage("inst_123")
	if err != nil {
		t.Fatalf("AgentLineage: %v", err)
	}
	if !lineageHasNode(agentLineage, "mission:"+mission.MissionRef) {
		t.Fatalf("agent lineage missing mission: %#v", agentLineage.Nodes)
	}
}

func TestGrandGovernanceValidationAndMatcherBranches(t *testing.T) {
	service := testService()
	missionResp := approveTestMission(t, service)
	mission, err := service.Introspect(missionResp.MissionRef)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	actor := Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}

	if _, err := service.GetAuthorityNegotiation(""); err == nil {
		t.Fatal("expected empty negotiation id to fail")
	}
	if _, err := service.CreateAuthorityNegotiation(mission.MissionRef, CreateAuthorityNegotiationRequest{
		MissionVersionSeen: mission.Version,
		Actor:              actor,
		RequestedAuthority: AuthorityRegion{},
	}); err == nil {
		t.Fatal("expected missing requested authority to fail")
	}
	if _, err := service.CreateContainmentRule(ContainmentRule{TargetType: "unknown", TargetID: "x"}); err == nil {
		t.Fatal("expected unsupported containment target")
	}
	if _, err := service.CreateContainmentRule(ContainmentRule{TargetType: ContainmentTargetAgent}); err == nil {
		t.Fatal("expected missing containment target id")
	}

	tenantRule := ContainmentRule{TenantID: "demo", TargetType: ContainmentTargetTenant, TargetID: "demo", Status: ContainmentStatusActive}
	if !containmentRuleMatchesEvaluation(tenantRule, mission, actor, Action{}) {
		t.Fatal("expected tenant containment to match evaluation")
	}
	principalRule := ContainmentRule{TargetType: ContainmentTargetPrincipal, TargetID: "alice@example.com", Status: ContainmentStatusActive}
	if !containmentRuleMatchesMission(principalRule, mission) {
		t.Fatal("expected principal containment to match mission")
	}
	toolRule := ContainmentRule{TargetType: ContainmentTargetTool, TargetID: "drive.read", Status: ContainmentStatusActive}
	if !containmentRuleMatchesEvaluation(toolRule, mission, actor, Action{Name: "drive.read"}) {
		t.Fatal("expected tool containment to match action name")
	}
	resourceRule := ContainmentRule{TargetType: ContainmentTargetResource, TargetID: "drive_folder/*", Status: ContainmentStatusActive}
	if !containmentRuleMatchesEvaluation(resourceRule, mission, actor, Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}}) {
		t.Fatal("expected wildcard resource containment to match")
	}

	projection := Projection{ProjectionID: "projection-1", MissionRef: mission.MissionRef, TenantID: "demo", Actor: actor}
	if !containmentRuleMatchesProjection(tenantRule, projection, []Mission{mission}) {
		t.Fatal("expected tenant containment to match projection")
	}
	lease := MissionLease{LeaseID: "lease-1", MissionRef: mission.MissionRef, TenantID: "demo", Actor: actor}
	if !containmentRuleMatchesLease(tenantRule, lease, []Mission{mission}) {
		t.Fatal("expected tenant containment to match lease")
	}
	expansion := ExpansionRequest{ExpansionID: "expansion-1", MissionRef: mission.MissionRef, TenantID: "demo", Requester: actor, Action: Action{Name: "drive.read", Resource: ActionResource{Type: "drive_folder", ID: "board"}}}
	if !containmentRuleMatchesExpansion(toolRule, expansion, []Mission{mission}) {
		t.Fatal("expected tool containment to match expansion")
	}
	identity := AgentIdentity{AgentID: "agt_1", TenantID: "demo", Agent: Agent{ClientID: "research-agent", InstanceID: "inst_123"}}
	if !containmentRuleMatchesIdentity(ContainmentRule{TargetType: ContainmentTargetAgent, TargetID: "agt_1"}, identity) {
		t.Fatal("expected agent identity containment to match")
	}
	contract := ToolContract{ToolName: "drive.read", ResourceType: "drive_folder", ResourceID: "board"}
	if !containmentRuleMatchesToolContract(resourceRule, contract) {
		t.Fatal("expected resource containment to match tool contract")
	}
	if supportedContainmentTarget("not-real") {
		t.Fatal("unexpected supported containment target")
	}
	if resourceTargetMatches("drive_folder:board", ActionResource{}) {
		t.Fatal("empty resource should not match")
	}

	expiredStore := NewMemoryStore()
	expiredService := NewService(expiredStore, fixedClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)})
	expiredMission := approveTestMission(t, expiredService)
	if _, err := expiredService.CreateContainmentRule(ContainmentRule{TargetType: ContainmentTargetMission, TargetID: expiredMission.MissionRef, ExpiresAt: time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("Create expired containment rule: %v", err)
	}
	allowed, err := expiredService.Evaluate(expiredMission.MissionRef, EvaluateRequest{
		MissionVersionSeen: expiredMission.MissionVersion,
		Actor:              actor,
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate with expired containment: %v", err)
	}
	if allowed.Decision != DecisionAllow {
		t.Fatalf("expired containment decision = %s, want allow", allowed.Decision)
	}
}

func TestAuthZENAndApprovalRuleHelperBranches(t *testing.T) {
	if authZENInt(map[string]any{"v": json.Number("42")}, "v") != 42 {
		t.Fatal("expected json.Number to parse")
	}
	if authZENInt(map[string]any{"v": "7"}, "v") != 7 {
		t.Fatal("expected string int to parse")
	}
	if authZENInt(map[string]any{"v": []string{"nope"}}, "v") != 0 {
		t.Fatal("expected unsupported int value to become zero")
	}

	expansion := ExpansionRequest{
		TenantID: "demo",
		Action:   Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:  map[string]any{"risk": "low"},
	}
	rule := ApprovalRule{AppliesTo: ApprovalAppliesExpansion, TenantID: "demo", ResourceType: "drive_folder", ResourceID: "board", Operation: "read", RiskLevel: "low"}
	if !approvalRuleMatchesExpansion(rule, expansion) {
		t.Fatal("expected approval rule to match")
	}
	if approvalRuleMatchesExpansion(ApprovalRule{AppliesTo: "mission"}, expansion) {
		t.Fatal("unexpected applies_to match")
	}
	if approvalRuleMatchesExpansion(ApprovalRule{AppliesTo: ApprovalAppliesExpansion, TenantID: "other"}, expansion) {
		t.Fatal("unexpected tenant match")
	}
	if approvalRuleMatchesExpansion(ApprovalRule{AppliesTo: ApprovalAppliesExpansion, ResourceType: "slack"}, expansion) {
		t.Fatal("unexpected resource type match")
	}
	if approvalRuleMatchesExpansion(ApprovalRule{AppliesTo: ApprovalAppliesExpansion, ResourceID: "other"}, expansion) {
		t.Fatal("unexpected resource id match")
	}
	if approvalRuleMatchesExpansion(ApprovalRule{AppliesTo: ApprovalAppliesExpansion, Operation: "write"}, expansion) {
		t.Fatal("unexpected operation match")
	}
	if approvalRuleMatchesExpansion(ApprovalRule{AppliesTo: ApprovalAppliesExpansion, RiskLevel: "high"}, expansion) {
		t.Fatal("unexpected risk match")
	}
	if approverAllowed(ApprovalRule{AllowedSubjects: []string{"bob"}}, Principal{Subject: "alice"}) {
		t.Fatal("unexpected subject approval")
	}
	if approverAllowed(ApprovalRule{AllowedIssuers: []string{"issuer-a"}}, Principal{Subject: "alice", Issuer: "issuer-b"}) {
		t.Fatal("unexpected issuer approval")
	}
}

func lineageHasNode(graph LineageGraph, id string) bool {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func lineageHasEdge(graph LineageGraph, from string, to string, edgeType string) bool {
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to && edge.Type == edgeType {
			return true
		}
	}
	return false
}

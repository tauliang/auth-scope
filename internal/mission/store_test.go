package mission

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryStoreProposalLifecycleAndErrors(t *testing.T) {
	store := NewMemoryStore()
	proposal := MissionProposal{ProposalID: "proposal-1", Status: StatePendingApproval}

	if err := store.SaveProposal(proposal); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}
	if err := store.SaveProposal(proposal); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveProposal duplicate err = %v, want ErrConflict", err)
	}

	got, err := store.GetProposal(proposal.ProposalID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if got.ProposalID != proposal.ProposalID {
		t.Fatalf("GetProposal id = %q, want %q", got.ProposalID, proposal.ProposalID)
	}
	if _, err := store.GetProposal("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProposal missing err = %v, want ErrNotFound", err)
	}

	if err := store.DeleteProposal(proposal.ProposalID); err != nil {
		t.Fatalf("DeleteProposal: %v", err)
	}
	if err := store.DeleteProposal(proposal.ProposalID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteProposal missing err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreExpansionDecisionIsAtomicAndVersioned(t *testing.T) {
	store := NewMemoryStore()
	mission := Mission{MissionID: "mission-1", MissionRef: "mref-1", Version: 1, State: StateActive}
	expansion := ExpansionRequest{ExpansionID: "expansion-1", MissionRef: mission.MissionRef, Status: ExpansionStatusPending}
	if err := store.SaveMission(mission); err != nil {
		t.Fatalf("SaveMission: %v", err)
	}
	if err := store.SaveExpansionRequest(expansion); err != nil {
		t.Fatalf("SaveExpansionRequest: %v", err)
	}
	updatedMission := mission
	updatedMission.Version = 2
	decided := expansion
	decided.Status = ExpansionStatusApproved
	commit := ExpansionDecisionCommit{
		Mission:                 &updatedMission,
		ExpectedMissionVersion:  1,
		Expansion:               decided,
		ExpectedExpansionStatus: ExpansionStatusPending,
		Event:                   Event{EventID: "event-1", Type: "mission.expansion_approved"},
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.CommitExpansionDecision(canceled, commit); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled commit err = %v, want context.Canceled", err)
	}
	unchanged, _ := store.GetMission(mission.MissionRef)
	if unchanged.Version != 1 || len(store.Events()) != 0 {
		t.Fatalf("canceled commit changed state: mission=%#v events=%#v", unchanged, store.Events())
	}

	if err := store.CommitExpansionDecision(context.Background(), commit); err != nil {
		t.Fatalf("CommitExpansionDecision: %v", err)
	}
	storedMission, _ := store.GetMission(mission.MissionRef)
	storedExpansion, _ := store.GetExpansionRequest(expansion.ExpansionID)
	if storedMission.Version != 2 || storedExpansion.Status != ExpansionStatusApproved || len(store.Events()) != 1 {
		t.Fatalf("committed state mismatch: mission=%#v expansion=%#v events=%#v", storedMission, storedExpansion, store.Events())
	}
	if err := store.CommitExpansionDecision(context.Background(), commit); !errors.Is(err, ErrConflict) {
		t.Fatalf("repeated commit err = %v, want ErrConflict", err)
	}
	if len(store.Events()) != 1 {
		t.Fatalf("repeated commit appended event: %#v", store.Events())
	}
}

func TestMemoryStoreAgentIdentityAndNonceLifecycle(t *testing.T) {
	store := NewMemoryStore()
	identity := AgentIdentity{
		AgentID:       "agent-1",
		TenantID:      "demo",
		Agent:         Agent{ClientID: "research-agent", InstanceID: "inst_123"},
		KeyThumbprint: "sha256:key",
		PublicKey:     "public-key",
		Status:        AgentStatusActive,
		CreatedAt:     time.Now().UTC(),
	}

	if err := store.SaveAgentIdentity(identity); err != nil {
		t.Fatalf("SaveAgentIdentity: %v", err)
	}
	if err := store.SaveAgentIdentity(identity); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveAgentIdentity duplicate err = %v, want ErrConflict", err)
	}
	got, err := store.GetAgentIdentity(identity.AgentID)
	if err != nil {
		t.Fatalf("GetAgentIdentity: %v", err)
	}
	if got.KeyThumbprint != identity.KeyThumbprint {
		t.Fatalf("GetAgentIdentity thumbprint = %q, want %q", got.KeyThumbprint, identity.KeyThumbprint)
	}
	if _, err := store.GetAgentIdentity("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAgentIdentity missing err = %v, want ErrNotFound", err)
	}

	identity.Status = AgentStatusRevoked
	if err := store.UpdateAgentIdentity(identity); err != nil {
		t.Fatalf("UpdateAgentIdentity: %v", err)
	}
	if err := store.UpdateAgentIdentity(AgentIdentity{AgentID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateAgentIdentity missing err = %v, want ErrNotFound", err)
	}

	nonce := AgentNonce{AgentID: identity.AgentID, Nonce: "nonce-1", RequestHash: "hash", SeenAt: time.Now().UTC()}
	if err := store.SaveAgentNonce(nonce); err != nil {
		t.Fatalf("SaveAgentNonce: %v", err)
	}
	if err := store.SaveAgentNonce(nonce); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveAgentNonce duplicate err = %v, want ErrConflict", err)
	}
	identities, err := store.ListAgentIdentities()
	if err != nil {
		t.Fatalf("ListAgentIdentities: %v", err)
	}
	if len(identities) != 1 || identities[0].AgentID != identity.AgentID {
		t.Fatalf("ListAgentIdentities = %#v", identities)
	}
}

func TestMemoryStoreMissionLifecycleChildrenAndEvents(t *testing.T) {
	store := NewMemoryStore()
	parent := Mission{MissionRef: "parent", State: StateActive, Version: 1}
	childB := Mission{MissionRef: "child-b", State: StateActive, Delegation: DelegationPolicy{ParentMissionRef: "parent"}}
	childA := Mission{MissionRef: "child-a", State: StateActive, Delegation: DelegationPolicy{ParentMissionRef: "parent"}}

	if err := store.SaveMission(parent); err != nil {
		t.Fatalf("SaveMission parent: %v", err)
	}
	if err := store.SaveMission(parent); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveMission duplicate err = %v, want ErrConflict", err)
	}
	if err := store.SaveMission(childB); err != nil {
		t.Fatalf("SaveMission childB: %v", err)
	}
	if err := store.SaveMission(childA); err != nil {
		t.Fatalf("SaveMission childA: %v", err)
	}

	got, err := store.GetMission(parent.MissionRef)
	if err != nil {
		t.Fatalf("GetMission: %v", err)
	}
	if got.MissionRef != parent.MissionRef {
		t.Fatalf("GetMission ref = %q, want %q", got.MissionRef, parent.MissionRef)
	}
	if _, err := store.GetMission("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMission missing err = %v, want ErrNotFound", err)
	}

	parent.Version = 2
	if err := store.UpdateMission(parent); err != nil {
		t.Fatalf("UpdateMission: %v", err)
	}
	if err := store.UpdateMission(Mission{MissionRef: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateMission missing err = %v, want ErrNotFound", err)
	}

	children, err := store.ChildrenOf(parent.MissionRef)
	if err != nil {
		t.Fatalf("ChildrenOf: %v", err)
	}
	if len(children) != 2 || children[0].MissionRef != "child-a" || children[1].MissionRef != "child-b" {
		t.Fatalf("ChildrenOf sorted children = %#v", children)
	}
	missions, err := store.ListMissions()
	if err != nil {
		t.Fatalf("ListMissions: %v", err)
	}
	if len(missions) != 3 || missions[0].MissionRef != "child-a" {
		t.Fatalf("ListMissions sorted missions = %#v", missions)
	}

	event := Event{EventID: "event-1", Type: "mission.test", OccurredAt: time.Now().UTC()}
	if err := store.AppendEvent(event); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	events := store.Events()
	if len(events) != 1 || events[0].EventID != event.EventID {
		t.Fatalf("Events = %#v", events)
	}
	events[0].EventID = "mutated"
	if store.Events()[0].EventID != event.EventID {
		t.Fatal("Events returned slice should be a copy")
	}
}

func TestMemoryStoreGovernanceLifecycle(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	expansion := ExpansionRequest{
		ExpansionID: "expansion-1",
		MissionRef:  "mission-1",
		TenantID:    "demo",
		Status:      ExpansionStatusPending,
		CreatedAt:   now,
	}
	if err := store.SaveExpansionRequest(expansion); err != nil {
		t.Fatalf("SaveExpansionRequest: %v", err)
	}
	if err := store.SaveExpansionRequest(expansion); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveExpansionRequest duplicate err = %v, want ErrConflict", err)
	}
	gotExpansion, err := store.GetExpansionRequest(expansion.ExpansionID)
	if err != nil {
		t.Fatalf("GetExpansionRequest: %v", err)
	}
	if gotExpansion.Status != ExpansionStatusPending {
		t.Fatalf("GetExpansionRequest status = %q, want pending", gotExpansion.Status)
	}
	if _, err := store.GetExpansionRequest("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetExpansionRequest missing err = %v, want ErrNotFound", err)
	}
	expansion.Status = ExpansionStatusApproved
	expansion.DecidedAt = now
	if err := store.UpdateExpansionRequest(expansion); err != nil {
		t.Fatalf("UpdateExpansionRequest: %v", err)
	}
	if err := store.UpdateExpansionRequest(ExpansionRequest{ExpansionID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateExpansionRequest missing err = %v, want ErrNotFound", err)
	}

	evidence := EvaluationEvidence{
		EvidenceID:     "evidence-1",
		MissionRef:     "mission-1",
		MissionVersion: 1,
		PolicyVersion:  DefaultPolicyVersionID,
		Decision:       DecisionAllow,
		Artifact:       "artifact",
		CreatedAt:      now,
	}
	if err := store.SaveEvaluationEvidence(evidence); err != nil {
		t.Fatalf("SaveEvaluationEvidence: %v", err)
	}
	if err := store.SaveEvaluationEvidence(evidence); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveEvaluationEvidence duplicate err = %v, want ErrConflict", err)
	}
	gotEvidence, err := store.GetEvaluationEvidence(evidence.EvidenceID)
	if err != nil {
		t.Fatalf("GetEvaluationEvidence: %v", err)
	}
	if gotEvidence.PolicyVersion != DefaultPolicyVersionID {
		t.Fatalf("GetEvaluationEvidence policy version = %q", gotEvidence.PolicyVersion)
	}
	if _, err := store.GetEvaluationEvidence("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetEvaluationEvidence missing err = %v, want ErrNotFound", err)
	}

	contract := ToolContract{ToolName: "drive.read", ResourceType: "drive_folder", ResourceIDParam: "folder_id", Operation: "read", CreatedAt: now}
	if err := store.SaveToolContract(contract); err != nil {
		t.Fatalf("SaveToolContract: %v", err)
	}
	if err := store.SaveToolContract(contract); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveToolContract duplicate err = %v, want ErrConflict", err)
	}
	gotContract, err := store.GetToolContract(contract.ToolName)
	if err != nil {
		t.Fatalf("GetToolContract: %v", err)
	}
	if gotContract.ResourceIDParam != "folder_id" {
		t.Fatalf("GetToolContract resource param = %q", gotContract.ResourceIDParam)
	}
	if _, err := store.GetToolContract("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetToolContract missing err = %v, want ErrNotFound", err)
	}
	expansions, err := store.ListExpansionRequests()
	if err != nil {
		t.Fatalf("ListExpansionRequests: %v", err)
	}
	if len(expansions) != 1 || expansions[0].ExpansionID != expansion.ExpansionID {
		t.Fatalf("ListExpansionRequests = %#v", expansions)
	}
	contracts, err := store.ListToolContracts()
	if err != nil {
		t.Fatalf("ListToolContracts: %v", err)
	}
	if len(contracts) != 1 || contracts[0].ToolName != contract.ToolName {
		t.Fatalf("ListToolContracts = %#v", contracts)
	}
}

func TestMemoryStoreAdvancedGovernanceLifecycle(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()

	projection := Projection{
		ProjectionID:   "projection-1",
		MissionRef:     "mission-1",
		MissionVersion: 1,
		Type:           ProjectionTypeMCPContext,
		Status:         ProjectionStatusActive,
		IssuedAt:       now,
		ExpiresAt:      now.Add(time.Minute),
	}
	if err := store.SaveProjection(projection); err != nil {
		t.Fatalf("SaveProjection: %v", err)
	}
	if err := store.SaveProjection(projection); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveProjection duplicate err = %v, want ErrConflict", err)
	}
	gotProjection, err := store.GetProjection(projection.ProjectionID)
	if err != nil {
		t.Fatalf("GetProjection: %v", err)
	}
	if gotProjection.Type != ProjectionTypeMCPContext {
		t.Fatalf("projection did not round-trip: %#v", gotProjection)
	}
	projection.Status = ProjectionStatusRevoked
	if err := store.UpdateProjection(projection); err != nil {
		t.Fatalf("UpdateProjection: %v", err)
	}
	if _, err := store.GetProjection("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjection missing err = %v, want ErrNotFound", err)
	}

	lease := MissionLease{
		LeaseID:        "lease-1",
		MissionRef:     "mission-1",
		MissionVersion: 1,
		Actor:          Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		CreatedAt:      now,
		ExpiresAt:      now.Add(time.Minute),
	}
	if err := store.SaveMissionLease(lease); err != nil {
		t.Fatalf("SaveMissionLease: %v", err)
	}
	if err := store.SaveMissionLease(lease); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveMissionLease duplicate err = %v, want ErrConflict", err)
	}
	gotLease, err := store.GetMissionLease(lease.LeaseID)
	if err != nil {
		t.Fatalf("GetMissionLease: %v", err)
	}
	if gotLease.Actor.AgentInstanceID != "inst_123" {
		t.Fatalf("lease did not round-trip: %#v", gotLease)
	}
	lease.RefreshedAt = now
	if err := store.UpdateMissionLease(lease); err != nil {
		t.Fatalf("UpdateMissionLease: %v", err)
	}
	if _, err := store.GetMissionLease("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMissionLease missing err = %v, want ErrNotFound", err)
	}
	projections, err := store.ListProjections()
	if err != nil {
		t.Fatalf("ListProjections: %v", err)
	}
	if len(projections) != 1 || projections[0].ProjectionID != projection.ProjectionID {
		t.Fatalf("ListProjections = %#v", projections)
	}
	leases, err := store.ListMissionLeases()
	if err != nil {
		t.Fatalf("ListMissionLeases: %v", err)
	}
	if len(leases) != 1 || leases[0].LeaseID != lease.LeaseID {
		t.Fatalf("ListMissionLeases = %#v", leases)
	}

	rule := ApprovalRule{RuleID: "rule-1", AppliesTo: ApprovalAppliesExpansion, RequiredApprovals: 2, CreatedAt: now}
	if err := store.SaveApprovalRule(rule); err != nil {
		t.Fatalf("SaveApprovalRule: %v", err)
	}
	if err := store.SaveApprovalRule(rule); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveApprovalRule duplicate err = %v, want ErrConflict", err)
	}
	rules, err := store.ListApprovalRules()
	if err != nil {
		t.Fatalf("ListApprovalRules: %v", err)
	}
	if len(rules) != 1 || rules[0].RuleID != rule.RuleID {
		t.Fatalf("ListApprovalRules = %#v", rules)
	}

	record := ApprovalRecord{
		ApprovalID: "approval-1",
		RuleID:     rule.RuleID,
		TargetType: ApprovalTargetExpansion,
		TargetID:   "expansion-1",
		Approver:   Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:  now,
	}
	if err := store.SaveApprovalRecord(record); err != nil {
		t.Fatalf("SaveApprovalRecord: %v", err)
	}
	if err := store.SaveApprovalRecord(record); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveApprovalRecord duplicate err = %v, want ErrConflict", err)
	}
	records, err := store.ListApprovalRecords(ApprovalTargetExpansion, "expansion-1")
	if err != nil {
		t.Fatalf("ListApprovalRecords: %v", err)
	}
	if len(records) != 1 || records[0].Approver.Subject != "alice@example.com" {
		t.Fatalf("ListApprovalRecords = %#v", records)
	}
}

func TestMemoryStoreGrandGovernanceLifecycle(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()

	negotiation := AuthorityNegotiation{
		NegotiationID:      "mneg-1",
		MissionRef:         "mref-1",
		MissionVersion:     1,
		TenantID:           "demo",
		Status:             NegotiationStatusCounteroffered,
		RequestedAuthority: AuthorityRegion{Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "delete"}}}},
		CreatedAt:          now,
	}
	if err := store.SaveAuthorityNegotiation(negotiation); err != nil {
		t.Fatalf("SaveAuthorityNegotiation: %v", err)
	}
	if err := store.SaveAuthorityNegotiation(negotiation); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveAuthorityNegotiation duplicate err = %v, want ErrConflict", err)
	}
	gotNegotiation, err := store.GetAuthorityNegotiation(negotiation.NegotiationID)
	if err != nil {
		t.Fatalf("GetAuthorityNegotiation: %v", err)
	}
	if gotNegotiation.Status != negotiation.Status {
		t.Fatalf("GetAuthorityNegotiation status = %s, want %s", gotNegotiation.Status, negotiation.Status)
	}
	if _, err := store.GetAuthorityNegotiation("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAuthorityNegotiation missing err = %v, want ErrNotFound", err)
	}

	rule := ContainmentRule{
		RuleID:     "ctr-1",
		TenantID:   "demo",
		TargetType: ContainmentTargetAgent,
		TargetID:   "inst_123",
		Status:     ContainmentStatusActive,
		Reason:     "test",
		CreatedAt:  now,
	}
	if err := store.SaveContainmentRule(rule); err != nil {
		t.Fatalf("SaveContainmentRule: %v", err)
	}
	if err := store.SaveContainmentRule(rule); !errors.Is(err, ErrConflict) {
		t.Fatalf("SaveContainmentRule duplicate err = %v, want ErrConflict", err)
	}
	gotRule, err := store.GetContainmentRule(rule.RuleID)
	if err != nil {
		t.Fatalf("GetContainmentRule: %v", err)
	}
	if gotRule.TargetID != rule.TargetID {
		t.Fatalf("GetContainmentRule = %#v", gotRule)
	}
	rule.Status = ContainmentStatusLifted
	rule.LiftedAt = now
	if err := store.UpdateContainmentRule(rule); err != nil {
		t.Fatalf("UpdateContainmentRule: %v", err)
	}
	if err := store.UpdateContainmentRule(ContainmentRule{RuleID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateContainmentRule missing err = %v, want ErrNotFound", err)
	}
	rules, err := store.ListContainmentRules()
	if err != nil {
		t.Fatalf("ListContainmentRules: %v", err)
	}
	if len(rules) != 1 || rules[0].RuleID != rule.RuleID {
		t.Fatalf("ListContainmentRules = %#v", rules)
	}
	if _, err := store.GetContainmentRule("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetContainmentRule missing err = %v, want ErrNotFound", err)
	}
}

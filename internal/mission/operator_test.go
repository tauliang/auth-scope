package mission

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestOperatorCollectionsFilterPaginateAndSummarize(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, fixedClock{now: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)})

	for i, tenant := range []string{"demo", "demo", "other"} {
		proposal := validProposalRequest()
		proposal.TenantID = tenant
		proposal.Intent.Objective = "Mission " + string(rune('A'+i))
		created, err := service.CreateProposal(proposal)
		if err != nil {
			t.Fatalf("CreateProposal: %v", err)
		}
		if i == 0 {
			if _, err := service.ApproveProposal(created.ProposalID, ApproveProposalRequest{Approver: Principal{Subject: "admin"}}); err != nil {
				t.Fatalf("ApproveProposal: %v", err)
			}
		}
	}

	page, err := service.ListProposals(ListQuery{TenantID: "demo", State: string(StatePendingApproval), Limit: 1})
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Intent.Objective != "Mission B" {
		t.Fatalf("proposal page = %#v", page)
	}

	missions, err := service.ListMissions(ListQuery{TenantID: "demo", Query: "mission a", Limit: 1})
	if err != nil {
		t.Fatalf("ListMissions: %v", err)
	}
	if missions.Total != 1 || missions.Items[0].Purpose.Objective != "Mission A" {
		t.Fatalf("mission page = %#v", missions)
	}

	allEvents, err := service.ListEvents(ListQuery{TenantID: "demo", Limit: 1})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if allEvents.Total < 3 || len(allEvents.Items) != 1 || allEvents.NextCursor == "" {
		t.Fatalf("event page = %#v", allEvents)
	}
	next, err := service.ListEvents(ListQuery{TenantID: "demo", Limit: 1, Cursor: allEvents.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].EventID == allEvents.Items[0].EventID {
		t.Fatalf("next event page = %#v err=%v", next, err)
	}
	if _, err := service.ListEvents(ListQuery{Cursor: "not-base64"}); err == nil {
		t.Fatal("expected invalid cursor error")
	}
	badOffset := base64.RawURLEncoding.EncodeToString([]byte("999"))
	if _, err := service.ListMissions(ListQuery{Cursor: badOffset}); err == nil {
		t.Fatal("expected out-of-range cursor error")
	}

	summary, err := service.OperationsSummary(ListQuery{TenantID: "demo"})
	if err != nil {
		t.Fatalf("OperationsSummary: %v", err)
	}
	if summary.MissionsTotal != 1 || summary.MissionsByState[StateActive] != 1 || summary.PendingProposals != 1 || !summary.ServiceCapabilities["containment"] {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestOperatorCollectionsCoverAgentsToolsProjectionsAndContainment(t *testing.T) {
	service := testService()
	store := service.identities.(*MemoryStore)
	now := service.clock.Now()
	identity := AgentIdentity{AgentID: "agent-1", TenantID: "demo", Agent: Agent{Provider: "provider", ClientID: "client", InstanceID: "instance"}, KeyThumbprint: "key", Status: AgentStatusActive, CreatedAt: now}
	if err := store.SaveAgentIdentity(identity); err != nil {
		t.Fatalf("SaveAgentIdentity: %v", err)
	}
	if err := service.governance.SaveToolContract(ToolContract{ToolName: "drive.read", ResourceType: "drive", Operation: "read", CreatedAt: now}); err != nil {
		t.Fatalf("SaveToolContract: %v", err)
	}
	if err := service.projections.SaveProjection(Projection{ProjectionID: "projection-1", MissionRef: "mission-1", TenantID: "demo", Type: ProjectionTypeOAuthClaims, Status: ProjectionStatusActive, IssuedAt: now}); err != nil {
		t.Fatalf("SaveProjection: %v", err)
	}
	rule, err := service.CreateContainmentRule(ContainmentRule{TenantID: "demo", TargetType: ContainmentTargetAgent, TargetID: "agent-1", CreatedBy: Principal{Subject: "admin"}})
	if err != nil {
		t.Fatalf("CreateContainmentRule: %v", err)
	}

	agents, err := service.ListAgents(ListQuery{TenantID: "demo", Status: AgentStatusActive, Query: "client"})
	if err != nil || agents.Total != 1 {
		t.Fatalf("agents = %#v err=%v", agents, err)
	}
	tools, err := service.ListToolContracts(ListQuery{Query: "drive"})
	if err != nil || tools.Total != 1 {
		t.Fatalf("tools = %#v err=%v", tools, err)
	}
	projections, err := service.ListProjections(ListQuery{TenantID: "demo", Type: ProjectionTypeOAuthClaims, Status: ProjectionStatusActive})
	if err != nil || projections.Total != 1 {
		t.Fatalf("projections = %#v err=%v", projections, err)
	}
	storedRule, err := service.GetContainmentRule(rule.RuleID)
	if err != nil || storedRule.RuleID != rule.RuleID {
		t.Fatalf("containment = %#v err=%v", storedRule, err)
	}
}

func TestOperatorCollectionsEncodeEmptyItemsAsArray(t *testing.T) {
	page, err := testService().ListAgents(ListQuery{Query: "missing"})
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if page.Items == nil || len(page.Items) != 0 || page.Total != 0 {
		t.Fatalf("empty page = %#v, want non-nil empty items", page)
	}
}

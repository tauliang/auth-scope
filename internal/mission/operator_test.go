package mission

import (
	"encoding/base64"
	"errors"
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

func TestOperatorCollectionsPropagateStoreErrors(t *testing.T) {
	boom := errors.New("store failed")
	cases := []struct {
		name  string
		store *failingOperatorStore
		call  func(*Service) error
	}{
		{
			name:  "summary missions",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listMissionsErr: boom},
			call: func(service *Service) error {
				_, err := service.OperationsSummary(ListQuery{})
				return err
			},
		},
		{
			name:  "summary proposals",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listProposalsErr: boom},
			call: func(service *Service) error {
				_, err := service.OperationsSummary(ListQuery{})
				return err
			},
		},
		{
			name:  "summary expansions",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listExpansionsErr: boom},
			call: func(service *Service) error {
				_, err := service.OperationsSummary(ListQuery{})
				return err
			},
		},
		{
			name:  "summary containments",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listContainmentsErr: boom},
			call: func(service *Service) error {
				_, err := service.OperationsSummary(ListQuery{})
				return err
			},
		},
		{
			name:  "summary agents",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listAgentsErr: boom},
			call: func(service *Service) error {
				_, err := service.OperationsSummary(ListQuery{})
				return err
			},
		},
		{
			name:  "summary projections",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listProjectionsErr: boom},
			call: func(service *Service) error {
				_, err := service.OperationsSummary(ListQuery{})
				return err
			},
		},
		{
			name:  "list missions",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listMissionsErr: boom},
			call: func(service *Service) error {
				_, err := service.ListMissions(ListQuery{})
				return err
			},
		},
		{
			name:  "list proposals",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listProposalsErr: boom},
			call: func(service *Service) error {
				_, err := service.ListProposals(ListQuery{})
				return err
			},
		},
		{
			name:  "list expansions",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listExpansionsErr: boom},
			call: func(service *Service) error {
				_, err := service.ListExpansions(ListQuery{})
				return err
			},
		},
		{
			name:  "list agents",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listAgentsErr: boom},
			call: func(service *Service) error {
				_, err := service.ListAgents(ListQuery{})
				return err
			},
		},
		{
			name:  "list tools",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listToolsErr: boom},
			call: func(service *Service) error {
				_, err := service.ListToolContracts(ListQuery{})
				return err
			},
		},
		{
			name:  "list projections",
			store: &failingOperatorStore{MemoryStore: NewMemoryStore(), listProjectionsErr: boom},
			call: func(service *Service) error {
				_, err := service.ListProjections(ListQuery{})
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(tc.store, fixedClock{now: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)})
			if err := tc.call(service); !errors.Is(err, boom) {
				t.Fatalf("err = %v, want %v", err, boom)
			}
		})
	}
}

func TestOperatorPaginationAndFilterHelpers(t *testing.T) {
	items := make([]int, MaxCollectionLimit+5)
	for i := range items {
		items[i] = i
	}
	page, err := paginate(items, ListQuery{Limit: MaxCollectionLimit + 100})
	if err != nil {
		t.Fatalf("paginate capped: %v", err)
	}
	if len(page.Items) != MaxCollectionLimit || page.NextCursor == "" {
		t.Fatalf("capped page = %#v", page)
	}
	nonNumericCursor := base64.RawURLEncoding.EncodeToString([]byte("nope"))
	if _, err := paginate(items, ListQuery{Cursor: nonNumericCursor}); err == nil {
		t.Fatal("expected non-numeric cursor to fail")
	}
	if tenantMatches("demo", "other") {
		t.Fatal("unexpected tenant match")
	}
	if valueMatches("active", "revoked") {
		t.Fatal("unexpected value match")
	}
	if textMatches("missing", "alpha", "beta") {
		t.Fatal("unexpected text match")
	}
}

type failingOperatorStore struct {
	*MemoryStore
	listMissionsErr     error
	listProposalsErr    error
	listExpansionsErr   error
	listContainmentsErr error
	listAgentsErr       error
	listProjectionsErr  error
	listToolsErr        error
}

func (s *failingOperatorStore) ListMissions() ([]Mission, error) {
	if s.listMissionsErr != nil {
		return nil, s.listMissionsErr
	}
	return s.MemoryStore.ListMissions()
}

func (s *failingOperatorStore) ListProposals() ([]MissionProposal, error) {
	if s.listProposalsErr != nil {
		return nil, s.listProposalsErr
	}
	return s.MemoryStore.ListProposals()
}

func (s *failingOperatorStore) ListExpansionRequests() ([]ExpansionRequest, error) {
	if s.listExpansionsErr != nil {
		return nil, s.listExpansionsErr
	}
	return s.MemoryStore.ListExpansionRequests()
}

func (s *failingOperatorStore) ListContainmentRules() ([]ContainmentRule, error) {
	if s.listContainmentsErr != nil {
		return nil, s.listContainmentsErr
	}
	return s.MemoryStore.ListContainmentRules()
}

func (s *failingOperatorStore) ListAgentIdentities() ([]AgentIdentity, error) {
	if s.listAgentsErr != nil {
		return nil, s.listAgentsErr
	}
	return s.MemoryStore.ListAgentIdentities()
}

func (s *failingOperatorStore) ListProjections() ([]Projection, error) {
	if s.listProjectionsErr != nil {
		return nil, s.listProjectionsErr
	}
	return s.MemoryStore.ListProjections()
}

func (s *failingOperatorStore) ListToolContracts() ([]ToolContract, error) {
	if s.listToolsErr != nil {
		return nil, s.listToolsErr
	}
	return s.MemoryStore.ListToolContracts()
}

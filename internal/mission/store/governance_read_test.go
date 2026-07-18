package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/tauliang/auth-scope/internal/mission"
)

func TestPostgresGovernanceReadStoreScopesBlastRadiusByTenant(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	m := sampleMission()
	projection := sampleProjection()
	lease := sampleMissionLease()
	expansion := sampleExpansionRequest()
	identity := sampleAgentIdentity()
	contract := sampleToolContract()
	tenantID := m.TenantID

	mock.ExpectQuery("SELECT mission_json FROM missions WHERE tenant_id").WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}).AddRow(mustJSON(t, m)))
	mock.ExpectQuery("SELECT projection_json FROM projections WHERE tenant_id").WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"projection_json"}).AddRow(mustJSON(t, projection)))
	mock.ExpectQuery("SELECT lease_json FROM mission_leases WHERE tenant_id").WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"lease_json"}).AddRow(mustJSON(t, lease)))
	mock.ExpectQuery("SELECT expansion_json FROM expansion_requests WHERE tenant_id").WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"expansion_json"}).AddRow(mustJSON(t, expansion)))
	mock.ExpectQuery("SELECT identity_json FROM agent_identities WHERE tenant_id").WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"identity_json"}).AddRow(mustJSON(t, identity)))
	mock.ExpectQuery("SELECT contract_json FROM tool_contracts").
		WillReturnRows(sqlmock.NewRows([]string{"contract_json"}).AddRow(mustJSON(t, contract)))

	snapshot, err := store.LoadBlastRadiusSnapshot(context.Background(), mission.ContainmentRule{
		TenantID:   tenantID,
		TargetType: mission.ContainmentTargetAgent,
		TargetID:   identity.Agent.InstanceID,
	})
	if err != nil {
		t.Fatalf("LoadBlastRadiusSnapshot: %v", err)
	}
	if len(snapshot.Missions) != 1 || len(snapshot.Projections) != 1 || len(snapshot.Agents) != 1 || len(snapshot.ToolContracts) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestPostgresGovernanceReadStoreLoadsOnlyActiveContainmentRules(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	rule := mission.ContainmentRule{
		RuleID:     "rule-1",
		TargetType: mission.ContainmentTargetMission,
		TargetID:   "mref_test",
		Status:     mission.ContainmentStatusActive,
	}
	at := testUnitNow()
	mock.ExpectQuery("SELECT rule_json FROM containment_rules").
		WithArgs(mission.ContainmentStatusActive, at).
		WillReturnRows(sqlmock.NewRows([]string{"rule_json"}).AddRow(mustJSON(t, rule)))
	rules, err := store.ListActiveContainmentRules(context.Background(), at)
	if err != nil {
		t.Fatalf("ListActiveContainmentRules: %v", err)
	}
	if len(rules) != 1 || rules[0].RuleID != rule.RuleID {
		t.Fatalf("active rules = %#v", rules)
	}
}

func TestPostgresGovernanceReadStoreLoadsMissionLineageArtifacts(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	m := sampleMission()
	expansion := sampleExpansionRequest()
	projection := sampleProjection()
	lease := sampleMissionLease()
	mock.ExpectQuery("WITH RECURSIVE").WithArgs(m.MissionRef).
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}).AddRow(mustJSON(t, m)))
	mock.ExpectQuery("SELECT expansion_json FROM expansion_requests").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"expansion_json"}).AddRow(mustJSON(t, expansion)))
	mock.ExpectQuery("SELECT projection_json FROM projections").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"projection_json"}).AddRow(mustJSON(t, projection)))
	mock.ExpectQuery("SELECT lease_json FROM mission_leases").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"lease_json"}).AddRow(mustJSON(t, lease)))

	snapshot, err := store.LoadMissionLineageSnapshot(context.Background(), m.MissionRef)
	if err != nil {
		t.Fatalf("LoadMissionLineageSnapshot: %v", err)
	}
	if len(snapshot.Missions) != 1 || len(snapshot.ExpansionRequests) != 1 || len(snapshot.Projections) != 1 || len(snapshot.Leases) != 1 {
		t.Fatalf("unexpected lineage snapshot: %#v", snapshot)
	}
}

func TestPostgresGovernanceReadStoreReturnsNotFoundForMissingMission(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	mock.ExpectQuery("WITH RECURSIVE").WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}))
	if _, err := store.LoadMissionLineageSnapshot(context.Background(), "missing"); err != mission.ErrNotFound {
		t.Fatalf("missing lineage err = %v, want ErrNotFound", err)
	}
}

func TestPostgresGovernanceReadStoreLoadsRegisteredAgentLineage(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	identity := sampleAgentIdentity()
	m := sampleMission()
	expansion := sampleExpansionRequest()
	projection := sampleProjection()
	lease := sampleMissionLease()
	mock.ExpectQuery("SELECT identity_json FROM agent_identities WHERE id").WithArgs(identity.AgentID).
		WillReturnRows(sqlmock.NewRows([]string{"identity_json"}).AddRow(mustJSON(t, identity)))
	mock.ExpectQuery("SELECT mission_json FROM missions").
		WithArgs(identity.TenantID, identity.Agent.InstanceID, identity.Agent.ClientID, identity.Agent.InstanceID).
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}).AddRow(mustJSON(t, m)))
	mock.ExpectQuery("SELECT expansion_json FROM expansion_requests").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"expansion_json"}).AddRow(mustJSON(t, expansion)))
	mock.ExpectQuery("SELECT projection_json FROM projections").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"projection_json"}).AddRow(mustJSON(t, projection)))
	mock.ExpectQuery("SELECT lease_json FROM mission_leases").WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"lease_json"}).AddRow(mustJSON(t, lease)))

	snapshot, err := store.LoadAgentLineageSnapshot(context.Background(), identity.AgentID)
	if err != nil {
		t.Fatalf("LoadAgentLineageSnapshot: %v", err)
	}
	if snapshot.Identity == nil || snapshot.Identity.AgentID != identity.AgentID || len(snapshot.Missions) != 1 || len(snapshot.ExpansionRequests) != 1 {
		t.Fatalf("unexpected registered agent lineage: %#v", snapshot)
	}
}

func TestPostgresGovernanceReadStoreLoadsUnregisteredAgentLineage(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	mock.ExpectQuery("SELECT identity_json FROM agent_identities WHERE id").WithArgs("inst_unknown").
		WillReturnRows(sqlmock.NewRows([]string{"identity_json"}))
	mock.ExpectQuery("SELECT mission_json FROM missions").WithArgs("inst_unknown").
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}))
	snapshot, err := store.LoadAgentLineageSnapshot(context.Background(), "inst_unknown")
	if err != nil {
		t.Fatalf("LoadAgentLineageSnapshot: %v", err)
	}
	if snapshot.Identity != nil || len(snapshot.Missions) != 0 {
		t.Fatalf("unexpected unregistered agent lineage: %#v", snapshot)
	}
}

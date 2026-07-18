package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/tauliang/auth-scope/internal/mission"
)

type unitClock struct {
	now time.Time
}

func (c unitClock) Now() time.Time {
	return c.now
}

func TestNewPostgresStoreValidationAndClose(t *testing.T) {
	if _, err := NewPostgresStore(nil, nil); err == nil {
		t.Fatal("expected nil db validation error")
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	mock.ExpectClose()

	store, err := NewPostgresStore(db, nil)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	if store.clock == nil {
		t.Fatal("expected default clock")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApplyMigrationsAppliesEmbeddedMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	names, err := MigrationNames()
	if err != nil {
		t.Fatalf("MigrationNames: %v", err)
	}
	for _, name := range names {
		mock.ExpectQuery("SELECT EXISTS").WithArgs(name).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectBegin()
		mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("INSERT INTO schema_migrations").WithArgs(name).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
	}

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApplyMigrationsSkipsAppliedMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	names, err := MigrationNames()
	if err != nil {
		t.Fatalf("MigrationNames: %v", err)
	}
	for _, name := range names {
		mock.ExpectQuery("SELECT EXISTS").WithArgs(name).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	}

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresStoreProposalMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	proposal := sampleProposal()
	mock.ExpectExec("INSERT INTO mission_proposals").
		WithArgs(
			proposal.ProposalID,
			proposal.TenantID,
			proposal.Principal.Subject,
			proposal.Agent.InstanceID,
			string(proposal.Status),
			proposal.Intent.Objective,
			sqlmock.AnyArg(),
			proposal.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveProposal(proposal); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}

	proposalJSON := mustJSON(t, proposal)
	mock.ExpectQuery("SELECT proposal_json FROM mission_proposals").
		WithArgs(proposal.ProposalID).
		WillReturnRows(sqlmock.NewRows([]string{"proposal_json"}).AddRow(proposalJSON))
	got, err := store.GetProposal(proposal.ProposalID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if got.Agent.InstanceID != proposal.Agent.InstanceID || got.AuthorityRegion.ForbiddenActions[0] != "send_external" {
		t.Fatalf("proposal did not round-trip: %#v", got)
	}

	mock.ExpectQuery("SELECT proposal_json FROM mission_proposals").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetProposal("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetProposal missing err = %v, want ErrNotFound", err)
	}

	mock.ExpectExec("DELETE FROM mission_proposals").WithArgs(proposal.ProposalID).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.DeleteProposal(proposal.ProposalID); err != nil {
		t.Fatalf("DeleteProposal: %v", err)
	}

	mock.ExpectExec("DELETE FROM mission_proposals").WithArgs("missing").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.DeleteProposal("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("DeleteProposal missing err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStoreMissionMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	m := sampleMission()
	mock.ExpectExec("INSERT INTO missions").
		WithArgs(
			m.MissionRef,
			m.MissionID,
			m.TenantID,
			string(m.State),
			m.Version,
			m.Principal.Subject,
			m.Agent.InstanceID,
			nullableString(m.Delegation.ParentMissionRef),
			m.Purpose.Objective,
			sqlmock.AnyArg(),
			m.Lifecycle.CreatedAt,
			store.clock.Now(),
			nullableTime(m.Lifecycle.ExpiresAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveMission(m); err != nil {
		t.Fatalf("SaveMission: %v", err)
	}

	missionJSON := mustJSON(t, m)
	mock.ExpectQuery("SELECT mission_json FROM missions").
		WithArgs(m.MissionRef).
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}).AddRow(missionJSON))
	got, err := store.GetMission(m.MissionRef)
	if err != nil {
		t.Fatalf("GetMission: %v", err)
	}
	if got.Agent.InstanceID != m.Agent.InstanceID || got.Delegation.ParentMissionRef != m.Delegation.ParentMissionRef {
		t.Fatalf("mission did not round-trip: %#v", got)
	}

	mock.ExpectQuery("SELECT mission_json FROM missions").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetMission("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetMission missing err = %v, want ErrNotFound", err)
	}

	m.State = mission.StateSuspended
	m.Version = 2
	mock.ExpectExec("UPDATE missions SET").
		WithArgs(
			string(m.State),
			m.Version,
			m.Principal.Subject,
			m.Agent.InstanceID,
			nullableString(m.Delegation.ParentMissionRef),
			m.Purpose.Objective,
			sqlmock.AnyArg(),
			store.clock.Now(),
			nullableTime(m.Lifecycle.ExpiresAt),
			m.MissionRef,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateMission(m); err != nil {
		t.Fatalf("UpdateMission: %v", err)
	}

	mock.ExpectExec("UPDATE missions SET").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.UpdateMission(m); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("UpdateMission missing err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStoreChildrenEventsAndOutbox(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	child := sampleMission()
	child.MissionRef = "mref_child"
	child.Delegation.ParentMissionRef = "mref_parent"
	childJSON := mustJSON(t, child)

	mock.ExpectQuery("SELECT mission_json").
		WithArgs("mref_parent").
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}).AddRow(childJSON))
	children, err := store.ChildrenOf("mref_parent")
	if err != nil {
		t.Fatalf("ChildrenOf: %v", err)
	}
	if len(children) != 1 || children[0].MissionRef != child.MissionRef {
		t.Fatalf("ChildrenOf = %#v, want child %s", children, child.MissionRef)
	}

	event := sampleEvent()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO events").
		WithArgs(event.EventID, event.Type, nullableString(event.MissionRef), nullableString(event.TenantID), sqlmock.AnyArg(), sqlmock.AnyArg(), event.OccurredAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO outbox_events").
		WithArgs(event.EventID, event.Type, nullableString(event.MissionRef), sqlmock.AnyArg(), sqlmock.AnyArg(), event.OccurredAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.AppendEvent(event); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	eventJSON := mustJSON(t, event)
	mock.ExpectQuery("SELECT event_json").
		WillReturnRows(sqlmock.NewRows([]string{"event_json"}).AddRow(eventJSON))
	events := store.Events()
	if len(events) != 1 || events[0].EventID != event.EventID {
		t.Fatalf("Events = %#v, want event %s", events, event.EventID)
	}

	payloadJSON := mustJSON(t, event.Payload)
	mock.ExpectQuery("WITH pending").
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "mission_ref", "payload", "created_at"}).
			AddRow(event.EventID, event.Type, event.MissionRef, payloadJSON, event.OccurredAt))
	outboxEvents, err := store.PublishOutboxEvents()
	if err != nil {
		t.Fatalf("PublishOutboxEvents: %v", err)
	}
	if len(outboxEvents) != 1 || outboxEvents[0].ID != event.EventID {
		t.Fatalf("PublishOutboxEvents = %#v, want event %s", outboxEvents, event.EventID)
	}

	mock.ExpectQuery("WITH pending").
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "mission_ref", "payload", "created_at"}))
	outboxEvents, err = store.PublishOutboxEvents()
	if err != nil {
		t.Fatalf("PublishOutboxEvents empty: %v", err)
	}
	if len(outboxEvents) != 0 {
		t.Fatalf("PublishOutboxEvents empty length = %d, want 0", len(outboxEvents))
	}
}

func TestPostgresStoreHelpers(t *testing.T) {
	if !isUniqueViolation(&pq.Error{Code: "23505"}) {
		t.Fatal("expected unique violation")
	}
	if isUniqueViolation(errors.New("not postgres")) {
		t.Fatal("expected non-postgres error not to be unique violation")
	}
	if nullableString("").Valid {
		t.Fatal("expected empty string to become SQL null")
	}
	if !nullableString("value").Valid {
		t.Fatal("expected non-empty string to be valid")
	}
	if nullableTime(time.Time{}).Valid {
		t.Fatal("expected zero time to become SQL null")
	}
	if !nullableTime(testUnitNow()).Valid {
		t.Fatal("expected non-zero time to be valid")
	}
}

func newMockPostgresStore(t *testing.T) (*PostgresStore, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	store, err := NewPostgresStore(db, unitClock{now: testUnitNow()})
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	cleanup := func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
		_ = db.Close()
	}
	return store, mock, cleanup
}

func sampleProposal() mission.MissionProposal {
	return mission.MissionProposal{
		ProposalID: "mprp_test",
		Status:     mission.StatePendingApproval,
		TenantID:   "tenant_1",
		Principal:  mission.Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		Agent:      mission.Agent{Provider: "https://agents.example.com", ClientID: "research-agent", InstanceID: "inst_123"},
		Intent:     mission.Purpose{Objective: "Prepare Q3 board packet", BusinessContext: "Finance close"},
		AuthorityRegion: mission.AuthorityRegion{
			Resources:        []mission.ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "write_draft"}}},
			ForbiddenActions: []string{"send_external"},
		},
		Conditions: []mission.Condition{{ID: "close-open", Expression: "finance.close.status == 'open'", Evaluation: "per_action", OnFailure: "suspend"}},
		Lifecycle:  mission.Lifecycle{CreatedAt: testUnitNow(), NotBefore: testUnitNow(), ExpiresAt: testUnitNow().Add(24 * time.Hour)},
		Delegation: mission.DelegationPolicy{Permitted: true, MaxDepth: 1, Attenuation: "strict_subset", CascadeRevocation: true},
		CreatedAt:  testUnitNow(),
	}
}

func sampleMission() mission.Mission {
	proposal := sampleProposal()
	return mission.Mission{
		MissionID:       "mis_test",
		MissionRef:      "mref_test",
		TenantID:        proposal.TenantID,
		State:           mission.StateActive,
		Version:         1,
		Principal:       proposal.Principal,
		Agent:           proposal.Agent,
		Purpose:         proposal.Intent,
		AuthorityRegion: proposal.AuthorityRegion,
		Conditions:      proposal.Conditions,
		Lifecycle:       proposal.Lifecycle,
		Delegation:      mission.DelegationPolicy{ParentMissionRef: "mref_parent", CascadeRevocation: true},
		Risk:            mission.RiskPolicy{DefaultMode: "signal_based"},
	}
}

func sampleEvent() mission.Event {
	return mission.Event{
		EventID:    "mev_test",
		MissionRef: "mref_test",
		TenantID:   "tenant_1",
		Type:       "mission.evaluated",
		Payload: map[string]any{
			"decision": "allow",
		},
		OccurredAt: testUnitNow(),
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return data
}

func testUnitNow() time.Time {
	return time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
}

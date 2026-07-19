package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
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
	t.Setenv("DATABASE_URL", "")
	if _, err := NewPostgresStoreFromEnv(); err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}

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

func TestPostgresStoreAgentIdentityMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	identity := sampleAgentIdentity()
	mock.ExpectExec("INSERT INTO agent_identities").
		WithArgs(
			identity.AgentID,
			identity.TenantID,
			identity.Agent.Provider,
			identity.Agent.ClientID,
			identity.Agent.InstanceID,
			identity.KeyThumbprint,
			identity.PublicKey,
			identity.Status,
			sqlmock.AnyArg(),
			identity.CreatedAt,
			nullableTime(identity.RevokedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveAgentIdentity(identity); err != nil {
		t.Fatalf("SaveAgentIdentity: %v", err)
	}
	mock.ExpectExec("INSERT INTO agent_identities").
		WithArgs(
			identity.AgentID,
			identity.TenantID,
			identity.Agent.Provider,
			identity.Agent.ClientID,
			identity.Agent.InstanceID,
			identity.KeyThumbprint,
			identity.PublicKey,
			identity.Status,
			sqlmock.AnyArg(),
			identity.CreatedAt,
			nullableTime(identity.RevokedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveAgentIdentity(identity); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveAgentIdentity duplicate err = %v, want ErrConflict", err)
	}

	identityJSON := mustJSON(t, identity)
	mock.ExpectQuery("SELECT identity_json FROM agent_identities").
		WithArgs(identity.AgentID).
		WillReturnRows(sqlmock.NewRows([]string{"identity_json"}).AddRow(identityJSON))
	got, err := store.GetAgentIdentity(identity.AgentID)
	if err != nil {
		t.Fatalf("GetAgentIdentity: %v", err)
	}
	if got.KeyThumbprint != identity.KeyThumbprint {
		t.Fatalf("identity did not round-trip: %#v", got)
	}

	mock.ExpectQuery("SELECT identity_json FROM agent_identities").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetAgentIdentity("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetAgentIdentity missing err = %v, want ErrNotFound", err)
	}

	identity.Status = mission.AgentStatusRevoked
	identity.RevokedAt = testUnitNow()
	mock.ExpectExec("UPDATE agent_identities SET").
		WithArgs(identity.Status, sqlmock.AnyArg(), nullableTime(identity.RevokedAt), identity.AgentID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateAgentIdentity(identity); err != nil {
		t.Fatalf("UpdateAgentIdentity: %v", err)
	}

	nonce := mission.AgentNonce{AgentID: identity.AgentID, Nonce: "nonce-1", RequestHash: "hash", SeenAt: testUnitNow()}
	mock.ExpectExec("INSERT INTO agent_nonces").
		WithArgs(nonce.AgentID, nonce.Nonce, nonce.RequestHash, nonce.SeenAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveAgentNonce(nonce); err != nil {
		t.Fatalf("SaveAgentNonce: %v", err)
	}
	mock.ExpectExec("INSERT INTO agent_nonces").
		WithArgs(nonce.AgentID, nonce.Nonce, nonce.RequestHash, nonce.SeenAt).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveAgentNonce(nonce); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveAgentNonce duplicate err = %v, want ErrConflict", err)
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

	mock.ExpectQuery("SELECT proposal_json").
		WillReturnRows(sqlmock.NewRows([]string{"proposal_json"}).AddRow(proposalJSON))
	proposals, err := store.ListProposals()
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(proposals) != 1 || proposals[0].ProposalID != proposal.ProposalID {
		t.Fatalf("ListProposals = %#v", proposals)
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

func TestPostgresStoreEventErrorBranches(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	event := sampleEvent()
	mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
	if err := store.AppendEvent(event); err == nil {
		t.Fatal("expected append event begin error")
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO events").
		WithArgs(event.EventID, event.Type, nullableString(event.MissionRef), nullableString(event.TenantID), sqlmock.AnyArg(), sqlmock.AnyArg(), event.OccurredAt).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()
	if err := store.AppendEvent(event); err == nil {
		t.Fatal("expected append event insert error")
	}

	mock.ExpectQuery("SELECT event_json").WillReturnError(errors.New("query failed"))
	if events := store.Events(); len(events) != 0 {
		t.Fatalf("Events query error = %#v, want empty", events)
	}

	mock.ExpectQuery("SELECT event_json").
		WillReturnRows(sqlmock.NewRows([]string{"event_json"}).AddRow([]byte("{")))
	if events := store.Events(); len(events) != 0 {
		t.Fatalf("Events malformed JSON = %#v, want empty", events)
	}
}

func TestPostgresStoreGovernanceMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	expansion := sampleExpansionRequest()
	mock.ExpectExec("INSERT INTO expansion_requests").
		WithArgs(
			expansion.ExpansionID,
			expansion.MissionRef,
			expansion.TenantID,
			expansion.Status,
			sqlmock.AnyArg(),
			expansion.CreatedAt,
			nullableTime(expansion.DecidedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveExpansionRequest(expansion); err != nil {
		t.Fatalf("SaveExpansionRequest: %v", err)
	}
	mock.ExpectExec("INSERT INTO expansion_requests").
		WithArgs(
			expansion.ExpansionID,
			expansion.MissionRef,
			expansion.TenantID,
			expansion.Status,
			sqlmock.AnyArg(),
			expansion.CreatedAt,
			nullableTime(expansion.DecidedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveExpansionRequest(expansion); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveExpansionRequest duplicate err = %v, want ErrConflict", err)
	}

	expansionJSON := mustJSON(t, expansion)
	mock.ExpectQuery("SELECT expansion_json FROM expansion_requests").
		WithArgs(expansion.ExpansionID).
		WillReturnRows(sqlmock.NewRows([]string{"expansion_json"}).AddRow(expansionJSON))
	gotExpansion, err := store.GetExpansionRequest(expansion.ExpansionID)
	if err != nil {
		t.Fatalf("GetExpansionRequest: %v", err)
	}
	if gotExpansion.Status != expansion.Status || gotExpansion.Action.Operation != expansion.Action.Operation {
		t.Fatalf("expansion did not round-trip: %#v", gotExpansion)
	}

	mock.ExpectQuery("SELECT expansion_json FROM expansion_requests").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetExpansionRequest("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetExpansionRequest missing err = %v, want ErrNotFound", err)
	}

	expansion.Status = mission.ExpansionStatusApproved
	expansion.DecidedAt = testUnitNow()
	mock.ExpectExec("UPDATE expansion_requests SET").
		WithArgs(expansion.Status, sqlmock.AnyArg(), nullableTime(expansion.DecidedAt), expansion.ExpansionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateExpansionRequest(expansion); err != nil {
		t.Fatalf("UpdateExpansionRequest: %v", err)
	}
	mock.ExpectExec("UPDATE expansion_requests SET").
		WithArgs(expansion.Status, sqlmock.AnyArg(), nullableTime(expansion.DecidedAt), expansion.ExpansionID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.UpdateExpansionRequest(expansion); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("UpdateExpansionRequest missing err = %v, want ErrNotFound", err)
	}

	evidence := sampleEvaluationEvidence()
	mock.ExpectExec("INSERT INTO evaluation_evidence").
		WithArgs(
			evidence.EvidenceID,
			evidence.MissionRef,
			nullableString(evidence.TenantID),
			evidence.Artifact,
			sqlmock.AnyArg(),
			evidence.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveEvaluationEvidence(evidence); err != nil {
		t.Fatalf("SaveEvaluationEvidence: %v", err)
	}
	mock.ExpectExec("INSERT INTO evaluation_evidence").
		WithArgs(
			evidence.EvidenceID,
			evidence.MissionRef,
			nullableString(evidence.TenantID),
			evidence.Artifact,
			sqlmock.AnyArg(),
			evidence.CreatedAt,
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveEvaluationEvidence(evidence); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveEvaluationEvidence duplicate err = %v, want ErrConflict", err)
	}

	evidenceJSON := mustJSON(t, evidence)
	mock.ExpectQuery("SELECT evidence_json FROM evaluation_evidence").
		WithArgs(evidence.EvidenceID).
		WillReturnRows(sqlmock.NewRows([]string{"evidence_json"}).AddRow(evidenceJSON))
	gotEvidence, err := store.GetEvaluationEvidence(evidence.EvidenceID)
	if err != nil {
		t.Fatalf("GetEvaluationEvidence: %v", err)
	}
	if gotEvidence.PolicyVersion != mission.DefaultPolicyVersionID || gotEvidence.Artifact != evidence.Artifact {
		t.Fatalf("evidence did not round-trip: %#v", gotEvidence)
	}
	mock.ExpectQuery("SELECT evidence_json FROM evaluation_evidence").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetEvaluationEvidence("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetEvaluationEvidence missing err = %v, want ErrNotFound", err)
	}

	contract := sampleToolContract()
	mock.ExpectExec("INSERT INTO tool_contracts").
		WithArgs(contract.ToolName, sqlmock.AnyArg(), contract.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveToolContract(contract); err != nil {
		t.Fatalf("SaveToolContract: %v", err)
	}
	mock.ExpectExec("INSERT INTO tool_contracts").
		WithArgs(contract.ToolName, sqlmock.AnyArg(), contract.CreatedAt).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveToolContract(contract); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveToolContract duplicate err = %v, want ErrConflict", err)
	}

	contractJSON := mustJSON(t, contract)
	mock.ExpectQuery("SELECT contract_json FROM tool_contracts").
		WithArgs(contract.ToolName).
		WillReturnRows(sqlmock.NewRows([]string{"contract_json"}).AddRow(contractJSON))
	gotContract, err := store.GetToolContract(contract.ToolName)
	if err != nil {
		t.Fatalf("GetToolContract: %v", err)
	}
	if gotContract.ResourceIDParam != contract.ResourceIDParam {
		t.Fatalf("tool contract did not round-trip: %#v", gotContract)
	}
	mock.ExpectQuery("SELECT contract_json FROM tool_contracts").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetToolContract("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetToolContract missing err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStoreCommitExpansionDecisionTransaction(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	m := sampleMission()
	expectedVersion := m.Version
	m.Version++
	expansion := sampleExpansionRequest()
	expansion.Status = mission.ExpansionStatusApproved
	expansion.DecidedAt = testUnitNow()
	event := sampleEvent()
	commit := mission.ExpansionDecisionCommit{
		Mission:                 &m,
		ExpectedMissionVersion:  expectedVersion,
		Expansion:               expansion,
		ExpectedExpansionStatus: mission.ExpansionStatusPending,
		Event:                   event,
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE missions SET").
		WithArgs(
			string(m.State), m.Version, m.Principal.Subject, m.Agent.InstanceID,
			nullableString(m.Delegation.ParentMissionRef), m.Purpose.Objective, sqlmock.AnyArg(),
			testUnitNow(), nullableTime(m.Lifecycle.ExpiresAt), m.MissionRef, expectedVersion,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE expansion_requests SET").
		WithArgs(expansion.Status, sqlmock.AnyArg(), nullableTime(expansion.DecidedAt), expansion.ExpansionID, mission.ExpansionStatusPending).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events").
		WithArgs(event.EventID, event.Type, nullableString(event.MissionRef), nullableString(event.TenantID), sqlmock.AnyArg(), sqlmock.AnyArg(), event.OccurredAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO outbox_events").
		WithArgs(event.EventID, event.Type, nullableString(event.MissionRef), sqlmock.AnyArg(), sqlmock.AnyArg(), event.OccurredAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.CommitExpansionDecision(context.Background(), commit); err != nil {
		t.Fatalf("CommitExpansionDecision: %v", err)
	}
}

func TestPostgresStoreCommitProposalApprovalTransaction(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	m := sampleMission()
	event := sampleEvent()
	commit := mission.ProposalApprovalCommit{
		ProposalID: sampleProposal().ProposalID,
		Mission:    m,
		Event:      event,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO missions").
		WithArgs(
			m.MissionRef, m.MissionID, m.TenantID, string(m.State), m.Version,
			m.Principal.Subject, m.Agent.InstanceID, nullableString(m.Delegation.ParentMissionRef),
			m.Purpose.Objective, sqlmock.AnyArg(), m.Lifecycle.CreatedAt, testUnitNow(), nullableTime(m.Lifecycle.ExpiresAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM mission_proposals").WithArgs(commit.ProposalID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events").
		WithArgs(event.EventID, event.Type, nullableString(event.MissionRef), nullableString(event.TenantID), sqlmock.AnyArg(), sqlmock.AnyArg(), event.OccurredAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO outbox_events").
		WithArgs(event.EventID, event.Type, nullableString(event.MissionRef), sqlmock.AnyArg(), sqlmock.AnyArg(), event.OccurredAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.CommitProposalApproval(context.Background(), commit); err != nil {
		t.Fatalf("CommitProposalApproval: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO missions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM mission_proposals").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if err := store.CommitProposalApproval(context.Background(), commit); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("CommitProposalApproval conflict err = %v, want ErrNotFound", err)
	}
}

func TestPostgresStoreCommitExpansionDecisionRollsBackOnConflictAndEventFailure(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	m := sampleMission()
	expectedVersion := m.Version
	m.Version++
	expansion := sampleExpansionRequest()
	expansion.Status = mission.ExpansionStatusApproved
	event := sampleEvent()
	commit := mission.ExpansionDecisionCommit{
		Mission:                 &m,
		ExpectedMissionVersion:  expectedVersion,
		Expansion:               expansion,
		ExpectedExpansionStatus: mission.ExpansionStatusPending,
		Event:                   event,
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE missions SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if err := store.CommitExpansionDecision(context.Background(), commit); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("conflicting commit err = %v, want ErrConflict", err)
	}

	commit.Mission = nil
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE expansion_requests SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO events").WillReturnError(errors.New("event unavailable"))
	mock.ExpectRollback()
	if err := store.CommitExpansionDecision(context.Background(), commit); err == nil {
		t.Fatal("expected event failure to roll back expansion decision")
	}
}

func TestPostgresStoreAdvancedGovernanceMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	projection := sampleProjection()
	mock.ExpectExec("INSERT INTO projections").
		WithArgs(
			projection.ProjectionID,
			projection.MissionRef,
			nullableString(projection.TenantID),
			projection.Status,
			sqlmock.AnyArg(),
			projection.IssuedAt,
			projection.ExpiresAt,
			nullableTime(projection.RevokedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveProjection(projection); err != nil {
		t.Fatalf("SaveProjection: %v", err)
	}
	mock.ExpectExec("INSERT INTO projections").
		WithArgs(
			projection.ProjectionID,
			projection.MissionRef,
			nullableString(projection.TenantID),
			projection.Status,
			sqlmock.AnyArg(),
			projection.IssuedAt,
			projection.ExpiresAt,
			nullableTime(projection.RevokedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveProjection(projection); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveProjection duplicate err = %v, want ErrConflict", err)
	}
	projectionJSON := mustJSON(t, projection)
	mock.ExpectQuery("SELECT projection_json FROM projections").
		WithArgs(projection.ProjectionID).
		WillReturnRows(sqlmock.NewRows([]string{"projection_json"}).AddRow(projectionJSON))
	gotProjection, err := store.GetProjection(projection.ProjectionID)
	if err != nil {
		t.Fatalf("GetProjection: %v", err)
	}
	if gotProjection.Type != projection.Type {
		t.Fatalf("projection did not round-trip: %#v", gotProjection)
	}
	mock.ExpectQuery("SELECT projection_json FROM projections").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetProjection("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetProjection missing err = %v, want ErrNotFound", err)
	}
	projection.Status = mission.ProjectionStatusRevoked
	projection.RevokedAt = testUnitNow()
	mock.ExpectExec("UPDATE projections SET").
		WithArgs(projection.Status, sqlmock.AnyArg(), projection.ExpiresAt, nullableTime(projection.RevokedAt), projection.ProjectionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateProjection(projection); err != nil {
		t.Fatalf("UpdateProjection: %v", err)
	}

	lease := sampleMissionLease()
	mock.ExpectExec("INSERT INTO mission_leases").
		WithArgs(lease.LeaseID, lease.MissionRef, nullableString(lease.TenantID), lease.MissionVersion, sqlmock.AnyArg(), lease.CreatedAt, lease.ExpiresAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveMissionLease(lease); err != nil {
		t.Fatalf("SaveMissionLease: %v", err)
	}
	mock.ExpectExec("INSERT INTO mission_leases").
		WithArgs(lease.LeaseID, lease.MissionRef, nullableString(lease.TenantID), lease.MissionVersion, sqlmock.AnyArg(), lease.CreatedAt, lease.ExpiresAt).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveMissionLease(lease); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveMissionLease duplicate err = %v, want ErrConflict", err)
	}
	leaseJSON := mustJSON(t, lease)
	mock.ExpectQuery("SELECT lease_json FROM mission_leases").
		WithArgs(lease.LeaseID).
		WillReturnRows(sqlmock.NewRows([]string{"lease_json"}).AddRow(leaseJSON))
	gotLease, err := store.GetMissionLease(lease.LeaseID)
	if err != nil {
		t.Fatalf("GetMissionLease: %v", err)
	}
	if gotLease.Actor.AgentInstanceID != lease.Actor.AgentInstanceID {
		t.Fatalf("lease did not round-trip: %#v", gotLease)
	}
	lease.RefreshedAt = testUnitNow()
	mock.ExpectExec("UPDATE mission_leases SET").
		WithArgs(lease.MissionVersion, sqlmock.AnyArg(), lease.ExpiresAt, lease.LeaseID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateMissionLease(lease); err != nil {
		t.Fatalf("UpdateMissionLease: %v", err)
	}

	rule := sampleApprovalRule()
	mock.ExpectExec("INSERT INTO approval_rules").
		WithArgs(rule.RuleID, nullableString(rule.TenantID), rule.AppliesTo, sqlmock.AnyArg(), rule.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveApprovalRule(rule); err != nil {
		t.Fatalf("SaveApprovalRule: %v", err)
	}
	mock.ExpectExec("INSERT INTO approval_rules").
		WithArgs(rule.RuleID, nullableString(rule.TenantID), rule.AppliesTo, sqlmock.AnyArg(), rule.CreatedAt).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveApprovalRule(rule); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveApprovalRule duplicate err = %v, want ErrConflict", err)
	}
	ruleJSON := mustJSON(t, rule)
	mock.ExpectQuery("SELECT rule_json").
		WillReturnRows(sqlmock.NewRows([]string{"rule_json"}).AddRow(ruleJSON))
	rules, err := store.ListApprovalRules()
	if err != nil {
		t.Fatalf("ListApprovalRules: %v", err)
	}
	if len(rules) != 1 || rules[0].RuleID != rule.RuleID {
		t.Fatalf("ListApprovalRules = %#v", rules)
	}

	record := sampleApprovalRecord()
	mock.ExpectExec("INSERT INTO approval_records").
		WithArgs(record.ApprovalID, record.TargetType, record.TargetID, nullableString(record.TenantID), record.Approver.Subject, record.Approver.Issuer, sqlmock.AnyArg(), record.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveApprovalRecord(record); err != nil {
		t.Fatalf("SaveApprovalRecord: %v", err)
	}
	mock.ExpectExec("INSERT INTO approval_records").
		WithArgs(record.ApprovalID, record.TargetType, record.TargetID, nullableString(record.TenantID), record.Approver.Subject, record.Approver.Issuer, sqlmock.AnyArg(), record.CreatedAt).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveApprovalRecord(record); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveApprovalRecord duplicate err = %v, want ErrConflict", err)
	}
	recordJSON := mustJSON(t, record)
	mock.ExpectQuery("SELECT record_json").
		WithArgs(record.TargetType, record.TargetID).
		WillReturnRows(sqlmock.NewRows([]string{"record_json"}).AddRow(recordJSON))
	records, err := store.ListApprovalRecords(record.TargetType, record.TargetID)
	if err != nil {
		t.Fatalf("ListApprovalRecords: %v", err)
	}
	if len(records) != 1 || records[0].ApprovalID != record.ApprovalID {
		t.Fatalf("ListApprovalRecords = %#v", records)
	}
}

func TestPostgresStoreGrandGovernanceMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	negotiation := sampleAuthorityNegotiation()
	mock.ExpectExec("INSERT INTO authority_negotiations").
		WithArgs(negotiation.NegotiationID, negotiation.MissionRef, nullableString(negotiation.TenantID), negotiation.Status, sqlmock.AnyArg(), negotiation.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveAuthorityNegotiation(negotiation); err != nil {
		t.Fatalf("SaveAuthorityNegotiation: %v", err)
	}
	mock.ExpectExec("INSERT INTO authority_negotiations").
		WithArgs(negotiation.NegotiationID, negotiation.MissionRef, nullableString(negotiation.TenantID), negotiation.Status, sqlmock.AnyArg(), negotiation.CreatedAt).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveAuthorityNegotiation(negotiation); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveAuthorityNegotiation duplicate err = %v, want ErrConflict", err)
	}
	negotiationJSON := mustJSON(t, negotiation)
	mock.ExpectQuery("SELECT negotiation_json FROM authority_negotiations").
		WithArgs(negotiation.NegotiationID).
		WillReturnRows(sqlmock.NewRows([]string{"negotiation_json"}).AddRow(negotiationJSON))
	gotNegotiation, err := store.GetAuthorityNegotiation(negotiation.NegotiationID)
	if err != nil {
		t.Fatalf("GetAuthorityNegotiation: %v", err)
	}
	if gotNegotiation.Status != negotiation.Status {
		t.Fatalf("negotiation did not round-trip: %#v", gotNegotiation)
	}
	mock.ExpectQuery("SELECT negotiation_json FROM authority_negotiations").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetAuthorityNegotiation("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetAuthorityNegotiation missing err = %v, want ErrNotFound", err)
	}

	rule := sampleContainmentRule()
	mock.ExpectExec("INSERT INTO containment_rules").
		WithArgs(rule.RuleID, nullableString(rule.TenantID), rule.TargetType, rule.TargetID, rule.Status, sqlmock.AnyArg(), rule.CreatedAt, nullableTime(rule.ExpiresAt), nullableTime(rule.LiftedAt)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveContainmentRule(rule); err != nil {
		t.Fatalf("SaveContainmentRule: %v", err)
	}
	mock.ExpectExec("INSERT INTO containment_rules").
		WithArgs(rule.RuleID, nullableString(rule.TenantID), rule.TargetType, rule.TargetID, rule.Status, sqlmock.AnyArg(), rule.CreatedAt, nullableTime(rule.ExpiresAt), nullableTime(rule.LiftedAt)).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveContainmentRule(rule); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveContainmentRule duplicate err = %v, want ErrConflict", err)
	}
	ruleJSON := mustJSON(t, rule)
	mock.ExpectQuery("SELECT rule_json FROM containment_rules").
		WithArgs(rule.RuleID).
		WillReturnRows(sqlmock.NewRows([]string{"rule_json"}).AddRow(ruleJSON))
	gotRule, err := store.GetContainmentRule(rule.RuleID)
	if err != nil {
		t.Fatalf("GetContainmentRule: %v", err)
	}
	if gotRule.TargetID != rule.TargetID {
		t.Fatalf("containment rule did not round-trip: %#v", gotRule)
	}
	mock.ExpectQuery("SELECT rule_json FROM containment_rules").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetContainmentRule("missing"); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("GetContainmentRule missing err = %v, want ErrNotFound", err)
	}
	rule.Status = mission.ContainmentStatusLifted
	rule.LiftedAt = testUnitNow()
	mock.ExpectExec("UPDATE containment_rules SET").
		WithArgs(rule.Status, sqlmock.AnyArg(), nullableTime(rule.ExpiresAt), nullableTime(rule.LiftedAt), rule.RuleID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateContainmentRule(rule); err != nil {
		t.Fatalf("UpdateContainmentRule: %v", err)
	}
	mock.ExpectExec("UPDATE containment_rules SET").
		WithArgs(rule.Status, sqlmock.AnyArg(), nullableTime(rule.ExpiresAt), nullableTime(rule.LiftedAt), rule.RuleID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.UpdateContainmentRule(rule); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("UpdateContainmentRule missing err = %v, want ErrNotFound", err)
	}
	mock.ExpectQuery("SELECT rule_json").
		WillReturnRows(sqlmock.NewRows([]string{"rule_json"}).AddRow(mustJSON(t, rule)))
	rules, err := store.ListContainmentRules()
	if err != nil {
		t.Fatalf("ListContainmentRules: %v", err)
	}
	if len(rules) != 1 || rules[0].RuleID != rule.RuleID {
		t.Fatalf("ListContainmentRules = %#v", rules)
	}
}

func TestPostgresStoreGitHubIntegrationMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	binding := sampleGitHubRepositoryBinding()
	bindingJSON := mustJSON(t, binding)
	mock.ExpectExec("INSERT INTO github_repository_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.Repository,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastWebhookAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveGitHubRepositoryBinding(binding); err != nil {
		t.Fatalf("SaveGitHubRepositoryBinding: %v", err)
	}
	mock.ExpectExec("INSERT INTO github_repository_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.Repository,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastWebhookAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveGitHubRepositoryBinding(binding); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveGitHubRepositoryBinding duplicate err = %v, want ErrConflict", err)
	}

	mock.ExpectQuery("SELECT binding_json FROM github_repository_bindings").
		WithArgs(binding.BindingID).
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(bindingJSON))
	gotBinding, err := store.GetGitHubRepositoryBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetGitHubRepositoryBinding: %v", err)
	}
	if gotBinding.BindingID != binding.BindingID {
		t.Fatalf("GetGitHubRepositoryBinding = %#v", gotBinding)
	}

	binding.LastCheckSHA = "abc123"
	updatedBindingJSON := mustJSON(t, binding)
	mock.ExpectExec("UPDATE github_repository_bindings").
		WithArgs(nullableString(binding.TenantID), binding.Repository, binding.Status, updatedBindingJSON, nullableTime(binding.LastWebhookAt), binding.BindingID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateGitHubRepositoryBinding(binding); err != nil {
		t.Fatalf("UpdateGitHubRepositoryBinding: %v", err)
	}

	mock.ExpectQuery("SELECT binding_json").
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(updatedBindingJSON))
	bindings, err := store.ListGitHubRepositoryBindings()
	if err != nil {
		t.Fatalf("ListGitHubRepositoryBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].Repository != binding.Repository {
		t.Fatalf("ListGitHubRepositoryBindings = %#v", bindings)
	}

	delivery := sampleGitHubWebhookDelivery()
	deliveryJSON := mustJSON(t, delivery)
	mock.ExpectExec("INSERT INTO github_webhook_deliveries").
		WithArgs(
			delivery.DeliveryID,
			delivery.Event,
			nullableString(delivery.Repository),
			nullableString(delivery.BindingID),
			nullableString(delivery.TenantID),
			nullableString(delivery.MissionRef),
			delivery.Status,
			deliveryJSON,
			sqlmock.AnyArg(),
			delivery.ReceivedAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveGitHubWebhookDelivery(delivery); err != nil {
		t.Fatalf("SaveGitHubWebhookDelivery: %v", err)
	}
	mock.ExpectExec("INSERT INTO github_webhook_deliveries").
		WithArgs(
			delivery.DeliveryID,
			delivery.Event,
			nullableString(delivery.Repository),
			nullableString(delivery.BindingID),
			nullableString(delivery.TenantID),
			nullableString(delivery.MissionRef),
			delivery.Status,
			deliveryJSON,
			sqlmock.AnyArg(),
			delivery.ReceivedAt,
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveGitHubWebhookDelivery(delivery); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveGitHubWebhookDelivery duplicate err = %v, want ErrConflict", err)
	}

	mock.ExpectQuery("SELECT delivery_json FROM github_webhook_deliveries").
		WithArgs(delivery.DeliveryID).
		WillReturnRows(sqlmock.NewRows([]string{"delivery_json"}).AddRow(deliveryJSON))
	gotDelivery, err := store.GetGitHubWebhookDelivery(delivery.DeliveryID)
	if err != nil {
		t.Fatalf("GetGitHubWebhookDelivery: %v", err)
	}
	if gotDelivery.DeliveryID != delivery.DeliveryID {
		t.Fatalf("GetGitHubWebhookDelivery = %#v", gotDelivery)
	}
}

func TestPostgresStoreOktaIntegrationMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	binding := sampleOktaAppBinding()
	bindingJSON := mustJSON(t, binding)
	mock.ExpectExec("INSERT INTO okta_app_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.Issuer,
			binding.ClientID,
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveOktaAppBinding(binding); err != nil {
		t.Fatalf("SaveOktaAppBinding: %v", err)
	}
	mock.ExpectExec("INSERT INTO okta_app_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.Issuer,
			binding.ClientID,
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveOktaAppBinding(binding); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveOktaAppBinding duplicate err = %v, want ErrConflict", err)
	}

	mock.ExpectQuery("SELECT binding_json FROM okta_app_bindings").
		WithArgs(binding.BindingID).
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(bindingJSON))
	gotBinding, err := store.GetOktaAppBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetOktaAppBinding: %v", err)
	}
	if gotBinding.BindingID != binding.BindingID {
		t.Fatalf("GetOktaAppBinding = %#v", gotBinding)
	}

	binding.LastResolvedAt = testUnitNow()
	binding.LastSubject = "00u1agent"
	binding.LastResolutionStatus = mission.OktaResolutionStatusAccepted
	updatedBindingJSON := mustJSON(t, binding)
	mock.ExpectExec("UPDATE okta_app_bindings").
		WithArgs(nullableString(binding.TenantID), binding.Issuer, binding.ClientID, binding.MissionRef, binding.Status, updatedBindingJSON, nullableTime(binding.LastResolvedAt), binding.BindingID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateOktaAppBinding(binding); err != nil {
		t.Fatalf("UpdateOktaAppBinding: %v", err)
	}

	mock.ExpectQuery("SELECT binding_json").
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(updatedBindingJSON))
	bindings, err := store.ListOktaAppBindings()
	if err != nil {
		t.Fatalf("ListOktaAppBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].ClientID != binding.ClientID {
		t.Fatalf("ListOktaAppBindings = %#v", bindings)
	}
}

func TestPostgresStoreEntraIntegrationMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	registration := sampleEntraAppRegistration()
	registrationJSON := mustJSON(t, registration)
	mock.ExpectExec("INSERT INTO entra_app_registrations").
		WithArgs(
			registration.RegistrationID,
			nullableString(registration.TenantID),
			registration.Issuer,
			registration.ClientID,
			registration.MissionRef,
			registration.Status,
			registrationJSON,
			registration.CreatedAt,
			nullableTime(registration.LastResolvedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveEntraAppRegistration(registration); err != nil {
		t.Fatalf("SaveEntraAppRegistration: %v", err)
	}
	mock.ExpectExec("INSERT INTO entra_app_registrations").
		WithArgs(
			registration.RegistrationID,
			nullableString(registration.TenantID),
			registration.Issuer,
			registration.ClientID,
			registration.MissionRef,
			registration.Status,
			registrationJSON,
			registration.CreatedAt,
			nullableTime(registration.LastResolvedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveEntraAppRegistration(registration); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveEntraAppRegistration duplicate err = %v, want ErrConflict", err)
	}

	mock.ExpectQuery("SELECT registration_json FROM entra_app_registrations").
		WithArgs(registration.RegistrationID).
		WillReturnRows(sqlmock.NewRows([]string{"registration_json"}).AddRow(registrationJSON))
	gotRegistration, err := store.GetEntraAppRegistration(registration.RegistrationID)
	if err != nil {
		t.Fatalf("GetEntraAppRegistration: %v", err)
	}
	if gotRegistration.RegistrationID != registration.RegistrationID {
		t.Fatalf("GetEntraAppRegistration = %#v", gotRegistration)
	}

	registration.LastResolvedAt = testUnitNow()
	registration.LastSubject = "user@example.onmicrosoft.com"
	registration.LastResolutionStatus = mission.EntraResolutionStatusAccepted
	updatedRegistrationJSON := mustJSON(t, registration)
	mock.ExpectExec("UPDATE entra_app_registrations").
		WithArgs(nullableString(registration.TenantID), registration.Issuer, registration.ClientID, registration.MissionRef, registration.Status, updatedRegistrationJSON, nullableTime(registration.LastResolvedAt), registration.RegistrationID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateEntraAppRegistration(registration); err != nil {
		t.Fatalf("UpdateEntraAppRegistration: %v", err)
	}

	mock.ExpectQuery("SELECT registration_json").
		WillReturnRows(sqlmock.NewRows([]string{"registration_json"}).AddRow(updatedRegistrationJSON))
	registrations, err := store.ListEntraAppRegistrations()
	if err != nil {
		t.Fatalf("ListEntraAppRegistrations: %v", err)
	}
	if len(registrations) != 1 || registrations[0].ClientID != registration.ClientID {
		t.Fatalf("ListEntraAppRegistrations = %#v", registrations)
	}
}

func TestPostgresStoreSlackIntegrationMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	binding := sampleSlackWorkspaceBinding()
	bindingJSON := mustJSON(t, binding)
	mock.ExpectExec("INSERT INTO slack_workspace_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.WorkspaceID,
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveSlackWorkspaceBinding(binding); err != nil {
		t.Fatalf("SaveSlackWorkspaceBinding: %v", err)
	}
	mock.ExpectExec("INSERT INTO slack_workspace_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.WorkspaceID,
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveSlackWorkspaceBinding(binding); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveSlackWorkspaceBinding duplicate err = %v, want ErrConflict", err)
	}

	mock.ExpectQuery("SELECT binding_json FROM slack_workspace_bindings").
		WithArgs(binding.BindingID).
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(bindingJSON))
	gotBinding, err := store.GetSlackWorkspaceBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetSlackWorkspaceBinding: %v", err)
	}
	if gotBinding.BindingID != binding.BindingID {
		t.Fatalf("GetSlackWorkspaceBinding = %#v", gotBinding)
	}

	binding.LastResolvedAt = testUnitNow()
	binding.LastUserID = "U12345678"
	binding.LastResolutionStatus = mission.SlackResolutionStatusAccepted
	updatedBindingJSON := mustJSON(t, binding)
	mock.ExpectExec("UPDATE slack_workspace_bindings").
		WithArgs(nullableString(binding.TenantID), binding.WorkspaceID, binding.MissionRef, binding.Status, updatedBindingJSON, nullableTime(binding.LastResolvedAt), binding.BindingID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateSlackWorkspaceBinding(binding); err != nil {
		t.Fatalf("UpdateSlackWorkspaceBinding: %v", err)
	}

	mock.ExpectQuery("SELECT binding_json").
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(updatedBindingJSON))
	bindings, err := store.ListSlackWorkspaceBindings()
	if err != nil {
		t.Fatalf("ListSlackWorkspaceBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].WorkspaceID != binding.WorkspaceID {
		t.Fatalf("ListSlackWorkspaceBindings = %#v", bindings)
	}
}

func TestPostgresStoreAtlassianIntegrationMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	binding := sampleAtlassianSiteBinding()
	bindingJSON := mustJSON(t, binding)
	mock.ExpectExec("INSERT INTO atlassian_site_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.SiteURL,
			nullableString(binding.CloudID),
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveAtlassianSiteBinding(binding); err != nil {
		t.Fatalf("SaveAtlassianSiteBinding: %v", err)
	}
	mock.ExpectExec("INSERT INTO atlassian_site_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.SiteURL,
			nullableString(binding.CloudID),
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveAtlassianSiteBinding(binding); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveAtlassianSiteBinding duplicate err = %v, want ErrConflict", err)
	}

	mock.ExpectQuery("SELECT binding_json FROM atlassian_site_bindings").
		WithArgs(binding.BindingID).
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(bindingJSON))
	gotBinding, err := store.GetAtlassianSiteBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetAtlassianSiteBinding: %v", err)
	}
	if gotBinding.BindingID != binding.BindingID {
		t.Fatalf("GetAtlassianSiteBinding = %#v", gotBinding)
	}

	binding.LastResolvedAt = testUnitNow()
	binding.LastSubject = "agent@example.com"
	binding.LastResolutionStatus = mission.AtlassianResolutionStatusAccepted
	updatedBindingJSON := mustJSON(t, binding)
	mock.ExpectExec("UPDATE atlassian_site_bindings SET").
		WithArgs(nullableString(binding.TenantID), binding.SiteURL, nullableString(binding.CloudID), binding.MissionRef, binding.Status, updatedBindingJSON, nullableTime(binding.LastResolvedAt), binding.BindingID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateAtlassianSiteBinding(binding); err != nil {
		t.Fatalf("UpdateAtlassianSiteBinding: %v", err)
	}

	mock.ExpectQuery("SELECT binding_json").
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(updatedBindingJSON))
	bindings, err := store.ListAtlassianSiteBindings()
	if err != nil {
		t.Fatalf("ListAtlassianSiteBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].SiteURL != binding.SiteURL {
		t.Fatalf("ListAtlassianSiteBindings = %#v", bindings)
	}
}

func TestPostgresStoreSalesforceIntegrationMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	binding := sampleSalesforceOrgBinding()
	bindingJSON := mustJSON(t, binding)
	mock.ExpectExec("INSERT INTO salesforce_org_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.InstanceURL,
			nullableString(binding.OrgID),
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveSalesforceOrgBinding(binding); err != nil {
		t.Fatalf("SaveSalesforceOrgBinding: %v", err)
	}
	mock.ExpectExec("INSERT INTO salesforce_org_bindings").
		WithArgs(
			binding.BindingID,
			nullableString(binding.TenantID),
			binding.InstanceURL,
			nullableString(binding.OrgID),
			binding.MissionRef,
			binding.Status,
			bindingJSON,
			binding.CreatedAt,
			nullableTime(binding.LastResolvedAt),
		).
		WillReturnError(&pq.Error{Code: "23505"})
	if err := store.SaveSalesforceOrgBinding(binding); !errors.Is(err, mission.ErrConflict) {
		t.Fatalf("SaveSalesforceOrgBinding duplicate err = %v, want ErrConflict", err)
	}

	mock.ExpectQuery("SELECT binding_json FROM salesforce_org_bindings").
		WithArgs(binding.BindingID).
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(bindingJSON))
	gotBinding, err := store.GetSalesforceOrgBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetSalesforceOrgBinding: %v", err)
	}
	if gotBinding.BindingID != binding.BindingID {
		t.Fatalf("GetSalesforceOrgBinding = %#v", gotBinding)
	}

	binding.LastResolvedAt = testUnitNow()
	binding.LastSubject = "agent@example.com"
	binding.LastResolutionStatus = mission.SalesforceResolutionStatusAccepted
	updatedBindingJSON := mustJSON(t, binding)
	mock.ExpectExec("UPDATE salesforce_org_bindings SET").
		WithArgs(nullableString(binding.TenantID), binding.InstanceURL, nullableString(binding.OrgID), binding.MissionRef, binding.Status, updatedBindingJSON, nullableTime(binding.LastResolvedAt), binding.BindingID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateSalesforceOrgBinding(binding); err != nil {
		t.Fatalf("UpdateSalesforceOrgBinding: %v", err)
	}

	mock.ExpectQuery("SELECT binding_json").
		WillReturnRows(sqlmock.NewRows([]string{"binding_json"}).AddRow(updatedBindingJSON))
	bindings, err := store.ListSalesforceOrgBindings()
	if err != nil {
		t.Fatalf("ListSalesforceOrgBindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].InstanceURL != binding.InstanceURL {
		t.Fatalf("ListSalesforceOrgBindings = %#v", bindings)
	}
}

func TestPostgresStoreListMethods(t *testing.T) {
	store, mock, cleanup := newMockPostgresStore(t)
	defer cleanup()

	identity := sampleAgentIdentity()
	mock.ExpectQuery("SELECT identity_json").
		WillReturnRows(sqlmock.NewRows([]string{"identity_json"}).AddRow(mustJSON(t, identity)))
	identities, err := store.ListAgentIdentities()
	if err != nil {
		t.Fatalf("ListAgentIdentities: %v", err)
	}
	if len(identities) != 1 || identities[0].AgentID != identity.AgentID {
		t.Fatalf("ListAgentIdentities = %#v", identities)
	}

	m := sampleMission()
	mock.ExpectQuery("SELECT mission_json").
		WillReturnRows(sqlmock.NewRows([]string{"mission_json"}).AddRow(mustJSON(t, m)))
	missions, err := store.ListMissions()
	if err != nil {
		t.Fatalf("ListMissions: %v", err)
	}
	if len(missions) != 1 || missions[0].MissionRef != m.MissionRef {
		t.Fatalf("ListMissions = %#v", missions)
	}

	expansion := sampleExpansionRequest()
	mock.ExpectQuery("SELECT expansion_json").
		WillReturnRows(sqlmock.NewRows([]string{"expansion_json"}).AddRow(mustJSON(t, expansion)))
	expansions, err := store.ListExpansionRequests()
	if err != nil {
		t.Fatalf("ListExpansionRequests: %v", err)
	}
	if len(expansions) != 1 || expansions[0].ExpansionID != expansion.ExpansionID {
		t.Fatalf("ListExpansionRequests = %#v", expansions)
	}

	contract := sampleToolContract()
	mock.ExpectQuery("SELECT contract_json").
		WillReturnRows(sqlmock.NewRows([]string{"contract_json"}).AddRow(mustJSON(t, contract)))
	contracts, err := store.ListToolContracts()
	if err != nil {
		t.Fatalf("ListToolContracts: %v", err)
	}
	if len(contracts) != 1 || contracts[0].ToolName != contract.ToolName {
		t.Fatalf("ListToolContracts = %#v", contracts)
	}

	projection := sampleProjection()
	mock.ExpectQuery("SELECT projection_json").
		WillReturnRows(sqlmock.NewRows([]string{"projection_json"}).AddRow(mustJSON(t, projection)))
	projections, err := store.ListProjections()
	if err != nil {
		t.Fatalf("ListProjections: %v", err)
	}
	if len(projections) != 1 || projections[0].ProjectionID != projection.ProjectionID {
		t.Fatalf("ListProjections = %#v", projections)
	}

	lease := sampleMissionLease()
	mock.ExpectQuery("SELECT lease_json").
		WillReturnRows(sqlmock.NewRows([]string{"lease_json"}).AddRow(mustJSON(t, lease)))
	leases, err := store.ListMissionLeases()
	if err != nil {
		t.Fatalf("ListMissionLeases: %v", err)
	}
	if len(leases) != 1 || leases[0].LeaseID != lease.LeaseID {
		t.Fatalf("ListMissionLeases = %#v", leases)
	}
}

func TestPostgresStoreHelpers(t *testing.T) {
	store := &PostgresStore{logger: slog.Default()}
	if err := store.Close(); err != nil {
		t.Fatalf("Close nil db: %v", err)
	}
	store.logSlowQuery("fast", time.Now())
	store.logSlowQuery("slow", time.Now().Add(-200*time.Millisecond))

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

func sampleAgentIdentity() mission.AgentIdentity {
	return mission.AgentIdentity{
		AgentID:       "agt_test",
		TenantID:      "tenant_1",
		Agent:         mission.Agent{Provider: "https://agents.example.com", ClientID: "research-agent", InstanceID: "inst_123", KeyThumbprint: "sha256:key"},
		PublicKey:     "public-key",
		KeyThumbprint: "sha256:key",
		Status:        mission.AgentStatusActive,
		CreatedAt:     testUnitNow(),
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

func sampleExpansionRequest() mission.ExpansionRequest {
	return mission.ExpansionRequest{
		ExpansionID:        "mex_test",
		MissionRef:         "mref_test",
		MissionVersionSeen: 1,
		TenantID:           "tenant_1",
		Requester:          mission.Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             mission.Action{Type: "tool_call", Resource: mission.ActionResource{Type: "slack_channel", ID: "board"}, Operation: "post_update"},
		RequestedAuthority: mission.AuthorityRegion{Resources: []mission.ResourceGrant{{Type: "slack_channel", ID: "board", Actions: []string{"post_update"}}}},
		Status:             mission.ExpansionStatusPending,
		CreatedAt:          testUnitNow(),
	}
}

func sampleEvaluationEvidence() mission.EvaluationEvidence {
	return mission.EvaluationEvidence{
		EvidenceID:     "mevd_test",
		MissionRef:     "mref_test",
		MissionVersion: 1,
		TenantID:       "tenant_1",
		PolicyVersion:  mission.DefaultPolicyVersionID,
		Actor:          mission.Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:         mission.Action{Type: "tool_call", Resource: mission.ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		ContextHash:    "sha256:test",
		Decision:       mission.DecisionAllow,
		Artifact:       "artifact",
		CreatedAt:      testUnitNow(),
	}
}

func sampleToolContract() mission.ToolContract {
	return mission.ToolContract{
		ToolName:        "drive.read",
		ResourceType:    "drive_folder",
		ResourceIDParam: "folder_id",
		Operation:       "read",
		ActionType:      "tool_call",
		RequiredContext: []string{"finance.close.status"},
		CreatedAt:       testUnitNow(),
	}
}

func sampleProjection() mission.Projection {
	return mission.Projection{
		ProjectionID:   "mprj_test",
		MissionRef:     "mref_test",
		MissionVersion: 1,
		TenantID:       "tenant_1",
		Type:           mission.ProjectionTypeMCPContext,
		Actor:          mission.Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Token:          "projection-token",
		Status:         mission.ProjectionStatusActive,
		IssuedAt:       testUnitNow(),
		ExpiresAt:      testUnitNow().Add(5 * time.Minute),
	}
}

func sampleMissionLease() mission.MissionLease {
	return mission.MissionLease{
		LeaseID:        "mlse_test",
		MissionRef:     "mref_test",
		MissionVersion: 1,
		TenantID:       "tenant_1",
		Actor:          mission.Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		CreatedAt:      testUnitNow(),
		ExpiresAt:      testUnitNow().Add(time.Minute),
	}
}

func sampleApprovalRule() mission.ApprovalRule {
	return mission.ApprovalRule{
		RuleID:            "apr_test",
		TenantID:          "tenant_1",
		AppliesTo:         mission.ApprovalAppliesExpansion,
		ResourceType:      "slack_channel",
		ResourceID:        "board",
		Operation:         "post_update",
		RiskLevel:         "high",
		RequiredApprovals: 2,
		AllowedIssuers:    []string{"https://idp.example.com"},
		CreatedAt:         testUnitNow(),
	}
}

func sampleApprovalRecord() mission.ApprovalRecord {
	return mission.ApprovalRecord{
		ApprovalID: "aprv_test",
		RuleID:     "apr_test",
		TargetType: mission.ApprovalTargetExpansion,
		TargetID:   "mex_test",
		TenantID:   "tenant_1",
		Approver:   mission.Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:  testUnitNow(),
	}
}

func sampleAuthorityNegotiation() mission.AuthorityNegotiation {
	return mission.AuthorityNegotiation{
		NegotiationID:  "mneg_test",
		MissionRef:     "mref_test",
		MissionVersion: 1,
		TenantID:       "tenant_1",
		Actor:          mission.Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		RequestedAuthority: mission.AuthorityRegion{
			Resources: []mission.ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "delete"}}},
		},
		ProposedAuthority: mission.AuthorityRegion{
			Resources: []mission.ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
		},
		DeniedAuthority: mission.AuthorityRegion{
			Resources: []mission.ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"delete"}}},
		},
		Status:    mission.NegotiationStatusCounteroffered,
		CreatedAt: testUnitNow(),
	}
}

func sampleContainmentRule() mission.ContainmentRule {
	return mission.ContainmentRule{
		RuleID:     "ctr_test",
		TenantID:   "tenant_1",
		TargetType: mission.ContainmentTargetAgent,
		TargetID:   "inst_123",
		Status:     mission.ContainmentStatusActive,
		Reason:     "unit test",
		CreatedBy:  mission.Principal{Subject: "security@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:  testUnitNow(),
		ExpiresAt:  testUnitNow().Add(time.Hour),
	}
}

func sampleGitHubRepositoryBinding() mission.GitHubRepositoryBinding {
	return mission.GitHubRepositoryBinding{
		BindingID:      "ghr_test",
		TenantID:       "tenant_1",
		Owner:          "tauliang",
		Repo:           "auth-scope",
		Repository:     "tauliang/auth-scope",
		DefaultBranch:  "main",
		InstallationID: 42,
		MissionRef:     "mref_test",
		BranchPatterns: []string{"main"},
		RequiredChecks: []string{"Auth Scope Mission Authority"},
		Status:         mission.GitHubRepositoryBindingStatusActive,
		CreatedBy:      mission.GitHubPrincipal{Subject: "admin@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:      testUnitNow(),
	}
}

func sampleGitHubWebhookDelivery() mission.GitHubWebhookDelivery {
	return mission.GitHubWebhookDelivery{
		DeliveryID:  "delivery_test",
		Event:       "pull_request",
		Action:      "synchronize",
		Repository:  "tauliang/auth-scope",
		SHA:         "abc123",
		PullRequest: 42,
		Branch:      "agent/fix-filter",
		BindingID:   "ghr_test",
		TenantID:    "tenant_1",
		MissionRef:  "mref_test",
		Status:      mission.GitHubWebhookDeliveryStatusAccepted,
		ReceivedAt:  testUnitNow(),
		PayloadSummary: map[string]any{
			"event":      "pull_request",
			"repository": "tauliang/auth-scope",
		},
	}
}

func sampleOktaAppBinding() mission.OktaAppBinding {
	return mission.OktaAppBinding{
		BindingID:             "okb_test",
		TenantID:              "tenant_1",
		Issuer:                "https://acme.okta.com/oauth2/default",
		AuthorizationServerID: "default",
		DiscoveryURL:          "https://acme.okta.com/oauth2/default/.well-known/openid-configuration",
		JWKSURI:               "https://acme.okta.com/oauth2/default/v1/keys",
		ClientID:              "0oaabc123client",
		AppID:                 "0oaapp123",
		AppLabel:              "Auth Scope Console",
		MissionRef:            "mref_test",
		RequiredGroups:        []string{"Mission Operators"},
		AdminGroups:           []string{"Mission Admins"},
		GroupClaim:            "groups",
		SubjectClaim:          "sub",
		ScopeClaim:            "scp",
		GroupMatchMode:        mission.OktaGroupMatchAny,
		Status:                mission.OktaAppBindingStatusActive,
		CreatedBy:             mission.OktaPrincipal{Subject: "admin@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:             testUnitNow(),
	}
}

func sampleEntraAppRegistration() mission.EntraAppRegistration {
	return mission.EntraAppRegistration{
		RegistrationID: "enr_test",
		TenantID:       "tenant_1",
		TenantName:     "Contoso",
		Issuer:         "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
		DiscoveryURL:   "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0/.well-known/openid-configuration",
		JWKSURI:        "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/discovery/v2.0/keys",
		ClientID:       "00000000-0000-0000-0000-000000000000",
		AppID:          "app_entra_001",
		AppName:        "Auth Scope Console",
		MissionRef:     "mref_test",
		RequiredGroups: []string{"Mission Operators"},
		AdminGroups:    []string{"Mission Admins"},
		GroupClaim:     "groups",
		SubjectClaim:   "sub",
		RolesClaim:     "roles",
		GroupMatchMode: mission.EntraGroupMatchAny,
		Status:         mission.EntraAppRegistrationStatusActive,
		CreatedBy:      mission.EntraPrincipal{Subject: "admin@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:      testUnitNow(),
	}
}

func sampleSlackWorkspaceBinding() mission.SlackWorkspaceBinding {
	return mission.SlackWorkspaceBinding{
		BindingID:       "slb_test",
		TenantID:        "tenant_1",
		WorkspaceID:     "T12345678",
		WorkspaceName:   "Acme Corp",
		WorkspaceURL:    "https://acme-corp.slack.com",
		MissionRef:      "mref_test",
		RequiredRoles:   []string{"Workspace Admin"},
		AdminRoles:      []string{"Owner"},
		AllowedChannels: []string{"C11111111"},
		BlockedChannels: []string{"C99999999"},
		AllowedUsers:    []string{"U12345678"},
		AllowedActions:  []string{mission.SlackActionTypePostMessage},
		RoleClaim:       "roles",
		RoleMatchMode:   mission.SlackRoleMatchAny,
		Status:          mission.SlackWorkspaceBindingStatusActive,
		Metadata:        map[string]string{"environment": "production"},
		CreatedBy:       mission.SlackPrincipal{UserID: "admin@example.com"},
		CreatedAt:       testUnitNow(),
	}
}

func sampleAtlassianSiteBinding() mission.AtlassianSiteBinding {
	return mission.AtlassianSiteBinding{
		BindingID:            "atb_test",
		TenantID:             "tenant_1",
		SiteURL:              "https://acme.atlassian.net",
		CloudID:              "cloud-acme",
		SiteName:             "Acme Atlassian",
		MissionRef:           "mref_test",
		JiraProjectKeys:      []string{"FIN"},
		ConfluenceSpaceKeys:  []string{"ENG"},
		AllowedJiraActions:   []string{mission.AtlassianJiraActionTransitionIssue},
		AllowedPageActions:   []string{mission.AtlassianConfluenceActionUpdatePage},
		RequiredGroups:       []string{"Mission Operators"},
		AdminGroups:          []string{"Mission Admins"},
		AllowedSubjects:      []string{"agent@example.com"},
		GroupClaim:           "groups",
		SubjectClaim:         "sub",
		EmailClaim:           "email",
		GroupMatchMode:       mission.AtlassianGroupMatchAny,
		Status:               mission.AtlassianSiteBindingStatusActive,
		Metadata:             map[string]string{"environment": "production"},
		CreatedBy:            mission.AtlassianPrincipal{Subject: "admin@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:            testUnitNow(),
		LastResolutionStatus: mission.AtlassianResolutionStatusDenied,
	}
}

func sampleSalesforceOrgBinding() mission.SalesforceOrgBinding {
	return mission.SalesforceOrgBinding{
		BindingID:              "sfb_test",
		TenantID:               "tenant_1",
		InstanceURL:            "https://acme.my.salesforce.com",
		OrgID:                  "00Dxx0000001ABC",
		OrgName:                "Acme Salesforce",
		MissionRef:             "mref_test",
		AllowedObjectAPINames:  []string{"Account"},
		AllowedRecordTypeNames: []string{"Customer"},
		AllowedActions:         []string{mission.SalesforceActionUpdateRecord},
		RequiredProfiles:       []string{"Standard User"},
		RequiredPermissionSets: []string{"CRM Agent"},
		AdminPermissionSets:    []string{"Mission Admin"},
		AllowedSubjects:        []string{"agent@example.com"},
		ProfileClaim:           "profile",
		PermissionSetsClaim:    "permission_sets",
		SubjectClaim:           "sub",
		UsernameClaim:          "username",
		EmailClaim:             "email",
		PermissionSetMatchMode: mission.SalesforcePermissionMatchAny,
		Status:                 mission.SalesforceOrgBindingStatusActive,
		Metadata:               map[string]string{"environment": "production"},
		CreatedBy:              mission.SalesforcePrincipal{Subject: "admin@example.com", Issuer: "https://idp.example.com"},
		CreatedAt:              testUnitNow(),
		LastResolutionStatus:   mission.SalesforceResolutionStatusDenied,
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

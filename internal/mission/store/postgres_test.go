package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/tauliang/auth-scope/internal/mission"
	"github.com/tauliang/auth-scope/internal/mission/store"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

func TestMigrationNamesAreEmbeddedInOrder(t *testing.T) {
	got, err := store.MigrationNames()
	if err != nil {
		t.Fatalf("MigrationNames: %v", err)
	}
	want := []string{
		"migrations/001_initial_schema.up.sql",
		"migrations/002_outbox_table.up.sql",
		"migrations/003_outbox_processed_table.up.sql",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MigrationNames() = %#v, want %#v", got, want)
	}
}

func TestPostgresStoreConformance(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	schema := fmt.Sprintf("auth_scope_test_%d", time.Now().UnixNano())
	quotedSchema := pq.QuoteIdentifier(schema)
	if _, err := db.Exec(`CREATE SCHEMA ` + quotedSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP SCHEMA IF EXISTS ` + quotedSchema + ` CASCADE`)
	})
	if _, err := db.Exec(`SET search_path TO ` + quotedSchema); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	if err := store.ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations should be idempotent: %v", err)
	}

	pgStore, err := store.NewPostgresStore(db, fixedClock{now: testNow()})
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	runStoreConformance(t, pgStore)
}

func runStoreConformance(t *testing.T, store mission.Store) {
	t.Helper()
	service := mission.NewService(store, fixedClock{now: testNow()})

	proposalResp, err := service.CreateProposal(validProposalRequest())
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	proposal, err := store.GetProposal(proposalResp.ProposalID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if proposal.Agent.InstanceID != "inst_123" {
		t.Fatalf("proposal Agent.InstanceID = %q, want inst_123", proposal.Agent.InstanceID)
	}
	if len(proposal.AuthorityRegion.ForbiddenActions) != 1 || proposal.AuthorityRegion.ForbiddenActions[0] != "send_external" {
		t.Fatalf("proposal forbidden actions did not round-trip: %#v", proposal.AuthorityRegion.ForbiddenActions)
	}
	if len(proposal.Conditions) != 1 || proposal.Conditions[0].OnFailure != "suspend" {
		t.Fatalf("proposal conditions did not round-trip: %#v", proposal.Conditions)
	}

	missionResp, err := service.ApproveProposal(proposalResp.ProposalID, mission.ApproveProposalRequest{
		Approver: mission.Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: mission.ApprovalEvidence{
			DisplayHash: "sha256:test",
			Method:      "integration-test",
		},
	})
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}
	if _, err := store.GetProposal(proposalResp.ProposalID); !errors.Is(err, mission.ErrNotFound) {
		t.Fatalf("approved proposal should be deleted, err = %v", err)
	}

	allowed, err := service.Evaluate(missionResp.MissionRef, mission.EvaluateRequest{
		MissionVersionSeen: missionResp.MissionVersion,
		Actor:              mission.Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             mission.Action{Type: "tool_call", Resource: mission.ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if allowed.Decision != mission.DecisionAllow {
		t.Fatalf("Evaluate decision = %s, want %s", allowed.Decision, mission.DecisionAllow)
	}

	child, err := service.Delegate(missionResp.MissionRef, mission.DelegationRequest{
		DelegatingActor: mission.Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TargetAgent:     mission.Agent{Provider: "https://agents.example.com", ClientID: "chart-agent", InstanceID: "inst_child"},
		RequestedAuthority: mission.AuthorityRegion{
			Resources:        []mission.ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
			ForbiddenActions: []string{"send_external"},
		},
		Delegation: mission.DelegationPolicy{Permitted: false, CascadeRevocation: true},
	})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}
	children, err := store.ChildrenOf(missionResp.MissionRef)
	if err != nil {
		t.Fatalf("ChildrenOf: %v", err)
	}
	if len(children) != 1 || children[0].MissionRef != child.ChildMissionRef {
		t.Fatalf("ChildrenOf() = %#v, want child %s", children, child.ChildMissionRef)
	}
	if children[0].Agent.InstanceID != "inst_child" {
		t.Fatalf("child Agent.InstanceID = %q, want inst_child", children[0].Agent.InstanceID)
	}

	if _, err := service.Revoke(missionResp.MissionRef, mission.StateChangeRequest{Reason: "principal revoked"}); err != nil {
		t.Fatalf("Revoke parent: %v", err)
	}
	childMission, err := service.Introspect(child.ChildMissionRef)
	if err != nil {
		t.Fatalf("Introspect child: %v", err)
	}
	if childMission.State != mission.StateRevoked {
		t.Fatalf("child state = %s, want %s", childMission.State, mission.StateRevoked)
	}

	events := service.Events()
	if len(events) < 5 {
		t.Fatalf("Events length = %d, want at least 5", len(events))
	}

	outboxEvents, err := store.PublishOutboxEvents()
	if err != nil {
		t.Fatalf("PublishOutboxEvents: %v", err)
	}
	if len(outboxEvents) != len(events) {
		t.Fatalf("published outbox events = %d, want %d", len(outboxEvents), len(events))
	}
	outboxEvents, err = store.PublishOutboxEvents()
	if err != nil {
		t.Fatalf("second PublishOutboxEvents: %v", err)
	}
	if len(outboxEvents) != 0 {
		t.Fatalf("second PublishOutboxEvents length = %d, want 0", len(outboxEvents))
	}
}

func validProposalRequest() mission.CreateProposalRequest {
	return mission.CreateProposalRequest{
		TenantID:  "demo",
		Principal: mission.Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		Agent:     mission.Agent{Provider: "https://agents.example.com", ClientID: "research-agent", InstanceID: "inst_123"},
		Intent:    mission.Purpose{Objective: "Prepare Q3 board packet", BusinessContext: "Finance close"},
		AuthorityRegion: mission.AuthorityRegion{
			Resources:        []mission.ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "write_draft"}}},
			ForbiddenActions: []string{"send_external"},
		},
		Conditions: []mission.Condition{{ID: "close-open", Expression: "finance.close.status == 'open'", Evaluation: "per_action", OnFailure: "suspend"}},
		Lifecycle:  mission.Lifecycle{ExpiresAt: time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)},
		Delegation: mission.DelegationPolicy{Permitted: true, MaxDepth: 1, Attenuation: "strict_subset", CascadeRevocation: true},
	}
}

func testNow() time.Time {
	return time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
}

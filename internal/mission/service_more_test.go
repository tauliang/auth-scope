package mission

import (
	"errors"
	"testing"
	"time"
)

func TestCreateProposalValidationAndDefaults(t *testing.T) {
	service := testService()

	tests := []struct {
		name string
		req  CreateProposalRequest
	}{
		{name: "missing principal", req: CreateProposalRequest{}},
		{name: "missing agent", req: CreateProposalRequest{Principal: Principal{Subject: "alice"}}},
		{name: "missing objective", req: CreateProposalRequest{Principal: Principal{Subject: "alice"}, Agent: Agent{ClientID: "agent", InstanceID: "inst"}}},
		{name: "missing resources", req: CreateProposalRequest{Principal: Principal{Subject: "alice"}, Agent: Agent{ClientID: "agent", InstanceID: "inst"}, Intent: Purpose{Objective: "do work"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.CreateProposal(tt.req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	resp, err := service.CreateProposal(CreateProposalRequest{
		Principal: Principal{Subject: "alice@example.com"},
		Agent:     Agent{ClientID: "agent", InstanceID: "inst"},
		Intent:    Purpose{Objective: "do work"},
		AuthorityRegion: AuthorityRegion{
			Resources: []ResourceGrant{{Type: "doc", ID: "1", Actions: []string{"read"}}},
		},
	})
	if err != nil {
		t.Fatalf("CreateProposal with defaults: %v", err)
	}
	proposal, err := service.missions.GetProposal(resp.ProposalID)
	if err != nil {
		t.Fatalf("GetProposal: %v", err)
	}
	if proposal.TenantID != "default" || proposal.Delegation.MaxDepth != 1 || proposal.Risk.DefaultMode != "signal_based" {
		t.Fatalf("defaults not applied: %#v", proposal)
	}
}

func TestApproveProposalNotFound(t *testing.T) {
	service := testService()
	if _, err := service.ApproveProposal("missing", ApproveProposalRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ApproveProposal missing err = %v, want ErrNotFound", err)
	}
}

func TestEvaluateExpiredUnauthorizedStaleAndExpansionBranches(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	stored, err := service.Introspect(mission.MissionRef)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	stored.Version = 3
	if err := service.missions.UpdateMission(stored); err != nil {
		t.Fatalf("UpdateMission: %v", err)
	}

	stale, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: 1,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate stale: %v", err)
	}
	if stale.Decision != DecisionRequireRefresh {
		t.Fatalf("stale decision = %s, want %s", stale.Decision, DecisionRequireRefresh)
	}

	unauthorized, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		Actor:   Actor{AgentInstanceID: "other", ClientID: "research-agent"},
		Action:  Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context: map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("Evaluate unauthorized: %v", err)
	}
	if unauthorized.Decision != DecisionDeny {
		t.Fatalf("unauthorized decision = %s, want %s", unauthorized.Decision, DecisionDeny)
	}

	expansion, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		Actor:   Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:  Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "comment"},
		Context: map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
	})
	if err != nil {
		t.Fatalf("Evaluate expansion: %v", err)
	}
	if expansion.Decision != DecisionRequireExpansion {
		t.Fatalf("expansion decision = %s, want %s", expansion.Decision, DecisionRequireExpansion)
	}

	missing, err := service.Evaluate("missing", EvaluateRequest{})
	if !errors.Is(err, ErrNotFound) || missing.Decision != "" {
		t.Fatalf("Evaluate missing response=%#v err=%v, want ErrNotFound", missing, err)
	}
}

func TestEvaluateExpiredMissionTransitionsToExpired(t *testing.T) {
	service := NewService(NewMemoryStore(), fixedClock{now: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)})
	mission := approveTestMission(t, service)

	resp, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		Actor:  Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action: Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
	})
	if err != nil {
		t.Fatalf("Evaluate expired: %v", err)
	}
	if resp.Decision != DecisionDeny {
		t.Fatalf("expired decision = %s, want %s", resp.Decision, DecisionDeny)
	}
	stored, err := service.Introspect(mission.MissionRef)
	if err != nil {
		t.Fatalf("Introspect expired: %v", err)
	}
	if stored.State != StateExpired {
		t.Fatalf("state = %s, want %s", stored.State, StateExpired)
	}
}

func TestResumeBranches(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	allowed, err := service.Resume(mission.MissionRef, ResumeRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("Resume allow: %v", err)
	}
	if allowed.Decision != DecisionAllow {
		t.Fatalf("resume decision = %s, want %s", allowed.Decision, DecisionAllow)
	}

	stored, _ := service.Introspect(mission.MissionRef)
	stored.Version = 5
	_ = service.missions.UpdateMission(stored)
	stale, err := service.Resume(mission.MissionRef, ResumeRequest{
		MissionVersionSeen: 1,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("Resume stale: %v", err)
	}
	if stale.Decision != DecisionRequireRefresh {
		t.Fatalf("stale resume decision = %s, want %s", stale.Decision, DecisionRequireRefresh)
	}

	unauthorized, err := service.Resume(mission.MissionRef, ResumeRequest{
		Actor: Actor{AgentInstanceID: "wrong", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("Resume unauthorized: %v", err)
	}
	if unauthorized.Decision != DecisionDeny {
		t.Fatalf("unauthorized resume = %s, want %s", unauthorized.Decision, DecisionDeny)
	}

	if _, err := service.Complete(mission.MissionRef, StateChangeRequest{Reason: "done"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	completed, err := service.Resume(mission.MissionRef, ResumeRequest{
		Actor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("Resume completed: %v", err)
	}
	if completed.Decision != DecisionDeny {
		t.Fatalf("completed resume = %s, want %s", completed.Decision, DecisionDeny)
	}
}

func TestResumeExpiredAndMissing(t *testing.T) {
	service := NewService(NewMemoryStore(), fixedClock{now: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)})
	mission := approveTestMission(t, service)

	expired, err := service.Resume(mission.MissionRef, ResumeRequest{
		Actor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
	})
	if err != nil {
		t.Fatalf("Resume expired: %v", err)
	}
	if expired.Decision != DecisionDeny {
		t.Fatalf("expired resume = %s, want %s", expired.Decision, DecisionDeny)
	}
	if _, err := service.Resume("missing", ResumeRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resume missing err = %v, want ErrNotFound", err)
	}
}

func TestDelegateFailureBranches(t *testing.T) {
	service := testService()
	parent := approveTestMission(t, service)

	if _, err := service.Delegate("missing", DelegationRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delegate missing err = %v, want ErrNotFound", err)
	}
	if _, err := service.Delegate(parent.MissionRef, DelegationRequest{DelegatingActor: Actor{AgentInstanceID: "wrong", ClientID: "research-agent"}}); err == nil {
		t.Fatal("expected unauthorized delegating actor error")
	}

	stored, _ := service.Introspect(parent.MissionRef)
	stored.Delegation.Permitted = false
	_ = service.missions.UpdateMission(stored)
	if _, err := service.Delegate(parent.MissionRef, DelegationRequest{DelegatingActor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}}); err == nil {
		t.Fatal("expected delegation not permitted error")
	}

	stored.Delegation.Permitted = true
	stored.Delegation.CurrentDepth = stored.Delegation.MaxDepth
	_ = service.missions.UpdateMission(stored)
	if _, err := service.Delegate(parent.MissionRef, DelegationRequest{DelegatingActor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}}); err == nil {
		t.Fatal("expected delegation depth exceeded error")
	}

	stored.Delegation.CurrentDepth = 0
	_ = service.missions.UpdateMission(stored)
	if _, err := service.Delegate(parent.MissionRef, DelegationRequest{
		DelegatingActor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TargetAgent:     Agent{Provider: "https://agents.example.com", ClientID: "child", InstanceID: "inst_child"},
		RequestedAuthority: AuthorityRegion{
			Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
			ForbiddenActions: []string{"send_external"},
		},
		ExpiresAt: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected child expiry exceeds parent error")
	}

	if _, err := service.Complete(parent.MissionRef, StateChangeRequest{Reason: "done"}); err != nil {
		t.Fatalf("Complete parent: %v", err)
	}
	if _, err := service.Delegate(parent.MissionRef, DelegationRequest{DelegatingActor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"}}); err == nil {
		t.Fatal("expected inactive parent error")
	}
}

func TestStateChangeMissingAndEvents(t *testing.T) {
	service := testService()
	if _, err := service.Revoke("missing", StateChangeRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke missing err = %v, want ErrNotFound", err)
	}
	if _, err := service.Complete("missing", StateChangeRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Complete missing err = %v, want ErrNotFound", err)
	}
	if len(service.Events()) != 0 {
		t.Fatalf("new service events = %#v, want empty", service.Events())
	}
}

func TestActorMatchesKeyThumbprintAndHelpers(t *testing.T) {
	mission := Mission{Agent: Agent{ClientID: "agent", InstanceID: "inst", KeyThumbprint: "key-1"}}
	if !actorMatches(mission, Actor{ClientID: "agent", AgentInstanceID: "inst", KeyThumbprint: "key-1"}) {
		t.Fatal("expected actor with matching key thumbprint to match")
	}
	if actorMatches(mission, Actor{ClientID: "agent", AgentInstanceID: "inst", KeyThumbprint: "key-2"}) {
		t.Fatal("expected mismatched key thumbprint to fail")
	}
	if actorMatches(mission, Actor{ClientID: "agent"}) {
		t.Fatal("expected missing agent instance id to fail")
	}
	if !highRisk(map[string]any{"risk": "critical"}) {
		t.Fatal("expected critical risk to be high risk")
	}
	if expired(Mission{Lifecycle: Lifecycle{NotBefore: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)}}, time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)) != true {
		t.Fatal("expected mission before not_before to be considered expired/not usable")
	}
}

func validProposalRequest() CreateProposalRequest {
	return CreateProposalRequest{
		TenantID:  "demo",
		Principal: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		Agent:     Agent{Provider: "https://agents.example.com", ClientID: "research-agent", InstanceID: "inst_123"},
		Intent:    Purpose{Objective: "Prepare Q3 board packet", BusinessContext: "Finance close"},
		AuthorityRegion: AuthorityRegion{
			Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read", "write_draft"}}},
			ForbiddenActions: []string{"send_external"},
		},
		Conditions: []Condition{{ID: "close-open", Expression: "finance.close.status == 'open'", Evaluation: "per_action", OnFailure: "suspend"}},
		Lifecycle:  Lifecycle{ExpiresAt: time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)},
		Delegation: DelegationPolicy{Permitted: true, MaxDepth: 1, Attenuation: "strict_subset", CascadeRevocation: true},
	}
}

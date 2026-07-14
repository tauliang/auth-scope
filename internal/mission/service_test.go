package mission

import (
	"testing"
	"time"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

func TestMissionCompletionStopsFutureEvaluation(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	allowed, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
	})
	if err != nil {
		t.Fatalf("evaluate active mission: %v", err)
	}
	if allowed.Decision != DecisionAllow {
		t.Fatalf("expected allow, got %s: %#v", allowed.Decision, allowed)
	}

	if _, err := service.Complete(mission.MissionRef, StateChangeRequest{Reason: "board packet approved"}); err != nil {
		t.Fatalf("complete mission: %v", err)
	}

	denied, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		Actor:   Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:  Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context: map[string]any{"finance.close.status": "open"},
	})
	if err != nil {
		t.Fatalf("evaluate completed mission: %v", err)
	}
	if denied.Decision != DecisionDeny {
		t.Fatalf("expected deny after completion, got %s", denied.Decision)
	}
}

func TestOutOfScopeIrreversibleActionRequiresApproval(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	resp, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "email", ID: "board"}, Operation: "send_external"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "high", "reversible": false},
	})
	if err != nil {
		t.Fatalf("evaluate irreversible action: %v", err)
	}
	if resp.Decision != DecisionRequireApproval {
		t.Fatalf("expected require_approval, got %s: %#v", resp.Decision, resp)
	}
	if resp.Escalation == nil || resp.Escalation.Type != "mission_expansion" {
		t.Fatalf("expected mission expansion escalation, got %#v", resp.Escalation)
	}
}

func TestDelegationRequiresStrictSubsetAndCascadeRevokesChild(t *testing.T) {
	service := testService()
	parent := approveTestMission(t, service)

	child, err := service.Delegate(parent.MissionRef, DelegationRequest{
		DelegatingActor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TargetAgent:     Agent{Provider: "https://agents.example.com", ClientID: "chart-agent", InstanceID: "inst_child"},
		RequestedAuthority: AuthorityRegion{
			Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
			ForbiddenActions: []string{"send_external"},
		},
		Delegation: DelegationPolicy{Permitted: false, MaxDepth: 0, CascadeRevocation: true},
	})
	if err != nil {
		t.Fatalf("delegate strict subset: %v", err)
	}

	_, err = service.Delegate(parent.MissionRef, DelegationRequest{
		DelegatingActor: Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		TargetAgent:     Agent{Provider: "https://agents.example.com", ClientID: "writer-agent", InstanceID: "inst_writer"},
		RequestedAuthority: AuthorityRegion{
			Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"delete"}}},
			ForbiddenActions: []string{"send_external"},
		},
	})
	if err == nil {
		t.Fatal("expected delegation outside parent authority to fail")
	}

	if _, err := service.Revoke(parent.MissionRef, StateChangeRequest{Reason: "principal revoked"}); err != nil {
		t.Fatalf("revoke parent: %v", err)
	}
	childMission, err := service.Introspect(child.ChildMissionRef)
	if err != nil {
		t.Fatalf("introspect child: %v", err)
	}
	if childMission.State != StateRevoked {
		t.Fatalf("expected child revoked by cascade, got %s", childMission.State)
	}
}

func TestConditionFailureSuspendsMission(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)

	resp, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "closed"},
	})
	if err != nil {
		t.Fatalf("evaluate with failed condition: %v", err)
	}
	if resp.Decision != DecisionSuspend {
		t.Fatalf("expected suspend, got %s: %#v", resp.Decision, resp)
	}
	updated, err := service.Introspect(mission.MissionRef)
	if err != nil {
		t.Fatalf("introspect suspended mission: %v", err)
	}
	if updated.State != StateSuspended {
		t.Fatalf("expected mission suspended, got %s", updated.State)
	}
}

func testService() *Service {
	return NewService(NewMemoryStore(), fixedClock{now: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)})
}

func approveTestMission(t *testing.T, service *Service) ApproveProposalResponse {
	t.Helper()
	proposal, err := service.CreateProposal(CreateProposalRequest{
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
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	mission, err := service.ApproveProposal(proposal.ProposalID, ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		ApprovalEvidence: ApprovalEvidence{
			DisplayHash: "sha256:test",
			Method:      "unit-test",
		},
	})
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	return mission
}

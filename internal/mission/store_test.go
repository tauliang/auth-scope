package mission

import (
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

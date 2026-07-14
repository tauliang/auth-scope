package mission

import (
	"errors"
	"slices"
	"sync"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Store interface {
	SaveProposal(MissionProposal) error
	GetProposal(string) (MissionProposal, error)
	DeleteProposal(string) error
	SaveMission(Mission) error
	GetMission(string) (Mission, error)
	UpdateMission(Mission) error
	ChildrenOf(string) ([]Mission, error)
	AppendEvent(Event) error
	Events() []Event
}

type MemoryStore struct {
	mu        sync.RWMutex
	proposals map[string]MissionProposal
	missions  map[string]Mission
	events    []Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		proposals: make(map[string]MissionProposal),
		missions:  make(map[string]Mission),
	}
}

func (s *MemoryStore) SaveProposal(proposal MissionProposal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.proposals[proposal.ProposalID]; ok {
		return ErrConflict
	}
	s.proposals[proposal.ProposalID] = proposal
	return nil
}

func (s *MemoryStore) GetProposal(id string) (MissionProposal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	proposal, ok := s.proposals[id]
	if !ok {
		return MissionProposal{}, ErrNotFound
	}
	return proposal, nil
}

func (s *MemoryStore) DeleteProposal(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.proposals[id]; !ok {
		return ErrNotFound
	}
	delete(s.proposals, id)
	return nil
}

func (s *MemoryStore) SaveMission(mission Mission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.missions[mission.MissionRef]; ok {
		return ErrConflict
	}
	s.missions[mission.MissionRef] = mission
	return nil
}

func (s *MemoryStore) GetMission(ref string) (Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mission, ok := s.missions[ref]
	if !ok {
		return Mission{}, ErrNotFound
	}
	return mission, nil
}

func (s *MemoryStore) UpdateMission(mission Mission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.missions[mission.MissionRef]; !ok {
		return ErrNotFound
	}
	s.missions[mission.MissionRef] = mission
	return nil
}

func (s *MemoryStore) ChildrenOf(parentRef string) ([]Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	children := make([]Mission, 0)
	for _, mission := range s.missions {
		if mission.Delegation.ParentMissionRef == parentRef {
			children = append(children, mission)
		}
	}
	slices.SortFunc(children, func(a, b Mission) int {
		if a.MissionRef < b.MissionRef {
			return -1
		}
		if a.MissionRef > b.MissionRef {
			return 1
		}
		return 0
	})
	return children, nil
}

func (s *MemoryStore) AppendEvent(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *MemoryStore) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]Event, len(s.events))
	copy(events, s.events)
	return events
}

package mission

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// OutboxEvent represents an event in the outbox table.
type OutboxEvent struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	MissionRef string         `json:"mission_ref,omitempty"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  time.Time      `json:"created_at"`
}

type ExpansionDecisionCommit struct {
	Mission                 *Mission
	ExpectedMissionVersion  int
	Expansion               ExpansionRequest
	ExpectedExpansionStatus string
	Event                   Event
}

type Store interface {
	IdentityStore
	MissionStore
	GovernanceStore
	ProjectionStore
	ApprovalStore
	NegotiationStore
	ContainmentStore
	ExpansionDecisionStore
	EventStore
	OutboxStore
	GovernanceReadStore
}

type MemoryStore struct {
	mu            sync.RWMutex
	agents        map[string]AgentIdentity
	nonces        map[string]AgentNonce
	proposals     map[string]MissionProposal
	missions      map[string]Mission
	expansions    map[string]ExpansionRequest
	evidence      map[string]EvaluationEvidence
	toolContracts map[string]ToolContract
	projections   map[string]Projection
	leases        map[string]MissionLease
	approvalRules map[string]ApprovalRule
	approvals     map[string][]ApprovalRecord
	negotiations  map[string]AuthorityNegotiation
	containments  map[string]ContainmentRule
	events        []Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		agents:        make(map[string]AgentIdentity),
		nonces:        make(map[string]AgentNonce),
		proposals:     make(map[string]MissionProposal),
		missions:      make(map[string]Mission),
		expansions:    make(map[string]ExpansionRequest),
		evidence:      make(map[string]EvaluationEvidence),
		toolContracts: make(map[string]ToolContract),
		projections:   make(map[string]Projection),
		leases:        make(map[string]MissionLease),
		approvalRules: make(map[string]ApprovalRule),
		approvals:     make(map[string][]ApprovalRecord),
		negotiations:  make(map[string]AuthorityNegotiation),
		containments:  make(map[string]ContainmentRule),
	}
}

func (s *MemoryStore) SaveAgentIdentity(identity AgentIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[identity.AgentID]; ok {
		return ErrConflict
	}
	s.agents[identity.AgentID] = identity
	return nil
}

func (s *MemoryStore) GetAgentIdentity(agentID string) (AgentIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	identity, ok := s.agents[agentID]
	if !ok {
		return AgentIdentity{}, ErrNotFound
	}
	return identity, nil
}

func (s *MemoryStore) UpdateAgentIdentity(identity AgentIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[identity.AgentID]; !ok {
		return ErrNotFound
	}
	s.agents[identity.AgentID] = identity
	return nil
}

func (s *MemoryStore) ListAgentIdentities() ([]AgentIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	identities := make([]AgentIdentity, 0, len(s.agents))
	for _, identity := range s.agents {
		identities = append(identities, identity)
	}
	slices.SortFunc(identities, func(a, b AgentIdentity) int {
		if a.AgentID < b.AgentID {
			return -1
		}
		if a.AgentID > b.AgentID {
			return 1
		}
		return 0
	})
	return identities, nil
}

func (s *MemoryStore) SaveAgentNonce(nonce AgentNonce) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := nonce.AgentID + "\x00" + nonce.Nonce
	if _, ok := s.nonces[key]; ok {
		return ErrConflict
	}
	s.nonces[key] = nonce
	return nil
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

func (s *MemoryStore) ListMissions() ([]Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	missions := make([]Mission, 0, len(s.missions))
	for _, mission := range s.missions {
		missions = append(missions, mission)
	}
	slices.SortFunc(missions, func(a, b Mission) int {
		if a.MissionRef < b.MissionRef {
			return -1
		}
		if a.MissionRef > b.MissionRef {
			return 1
		}
		return 0
	})
	return missions, nil
}

func (s *MemoryStore) SaveExpansionRequest(expansion ExpansionRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.expansions[expansion.ExpansionID]; ok {
		return ErrConflict
	}
	s.expansions[expansion.ExpansionID] = expansion
	return nil
}

func (s *MemoryStore) GetExpansionRequest(id string) (ExpansionRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	expansion, ok := s.expansions[id]
	if !ok {
		return ExpansionRequest{}, ErrNotFound
	}
	return expansion, nil
}

func (s *MemoryStore) UpdateExpansionRequest(expansion ExpansionRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.expansions[expansion.ExpansionID]; !ok {
		return ErrNotFound
	}
	s.expansions[expansion.ExpansionID] = expansion
	return nil
}

func (s *MemoryStore) CommitExpansionDecision(ctx context.Context, commit ExpansionDecisionCommit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	currentExpansion, ok := s.expansions[commit.Expansion.ExpansionID]
	if !ok {
		return ErrNotFound
	}
	if currentExpansion.Status != commit.ExpectedExpansionStatus {
		return ErrConflict
	}
	if commit.Mission != nil {
		currentMission, ok := s.missions[commit.Mission.MissionRef]
		if !ok {
			return ErrNotFound
		}
		if currentMission.Version != commit.ExpectedMissionVersion {
			return ErrConflict
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if commit.Mission != nil {
		s.missions[commit.Mission.MissionRef] = *commit.Mission
	}
	s.expansions[commit.Expansion.ExpansionID] = commit.Expansion
	s.events = append(s.events, commit.Event)
	return nil
}

func (s *MemoryStore) ListExpansionRequests() ([]ExpansionRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	expansions := make([]ExpansionRequest, 0, len(s.expansions))
	for _, expansion := range s.expansions {
		expansions = append(expansions, expansion)
	}
	slices.SortFunc(expansions, func(a, b ExpansionRequest) int {
		if a.ExpansionID < b.ExpansionID {
			return -1
		}
		if a.ExpansionID > b.ExpansionID {
			return 1
		}
		return 0
	})
	return expansions, nil
}

func (s *MemoryStore) SaveEvaluationEvidence(evidence EvaluationEvidence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.evidence[evidence.EvidenceID]; ok {
		return ErrConflict
	}
	s.evidence[evidence.EvidenceID] = evidence
	return nil
}

func (s *MemoryStore) GetEvaluationEvidence(id string) (EvaluationEvidence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evidence, ok := s.evidence[id]
	if !ok {
		return EvaluationEvidence{}, ErrNotFound
	}
	return evidence, nil
}

func (s *MemoryStore) SaveToolContract(contract ToolContract) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.toolContracts[contract.ToolName]; ok {
		return ErrConflict
	}
	s.toolContracts[contract.ToolName] = contract
	return nil
}

func (s *MemoryStore) GetToolContract(toolName string) (ToolContract, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	contract, ok := s.toolContracts[toolName]
	if !ok {
		return ToolContract{}, ErrNotFound
	}
	return contract, nil
}

func (s *MemoryStore) ListToolContracts() ([]ToolContract, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	contracts := make([]ToolContract, 0, len(s.toolContracts))
	for _, contract := range s.toolContracts {
		contracts = append(contracts, contract)
	}
	slices.SortFunc(contracts, func(a, b ToolContract) int {
		if a.ToolName < b.ToolName {
			return -1
		}
		if a.ToolName > b.ToolName {
			return 1
		}
		return 0
	})
	return contracts, nil
}

func (s *MemoryStore) SaveProjection(projection Projection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projections[projection.ProjectionID]; ok {
		return ErrConflict
	}
	s.projections[projection.ProjectionID] = projection
	return nil
}

func (s *MemoryStore) GetProjection(id string) (Projection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	projection, ok := s.projections[id]
	if !ok {
		return Projection{}, ErrNotFound
	}
	return projection, nil
}

func (s *MemoryStore) UpdateProjection(projection Projection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projections[projection.ProjectionID]; !ok {
		return ErrNotFound
	}
	s.projections[projection.ProjectionID] = projection
	return nil
}

func (s *MemoryStore) ListProjections() ([]Projection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	projections := make([]Projection, 0, len(s.projections))
	for _, projection := range s.projections {
		projections = append(projections, projection)
	}
	slices.SortFunc(projections, func(a, b Projection) int {
		if a.ProjectionID < b.ProjectionID {
			return -1
		}
		if a.ProjectionID > b.ProjectionID {
			return 1
		}
		return 0
	})
	return projections, nil
}

func (s *MemoryStore) SaveMissionLease(lease MissionLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.leases[lease.LeaseID]; ok {
		return ErrConflict
	}
	s.leases[lease.LeaseID] = lease
	return nil
}

func (s *MemoryStore) GetMissionLease(id string) (MissionLease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lease, ok := s.leases[id]
	if !ok {
		return MissionLease{}, ErrNotFound
	}
	return lease, nil
}

func (s *MemoryStore) UpdateMissionLease(lease MissionLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.leases[lease.LeaseID]; !ok {
		return ErrNotFound
	}
	s.leases[lease.LeaseID] = lease
	return nil
}

func (s *MemoryStore) ListMissionLeases() ([]MissionLease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	leases := make([]MissionLease, 0, len(s.leases))
	for _, lease := range s.leases {
		leases = append(leases, lease)
	}
	slices.SortFunc(leases, func(a, b MissionLease) int {
		if a.LeaseID < b.LeaseID {
			return -1
		}
		if a.LeaseID > b.LeaseID {
			return 1
		}
		return 0
	})
	return leases, nil
}

func (s *MemoryStore) SaveApprovalRule(rule ApprovalRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.approvalRules[rule.RuleID]; ok {
		return ErrConflict
	}
	s.approvalRules[rule.RuleID] = rule
	return nil
}

func (s *MemoryStore) ListApprovalRules() ([]ApprovalRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rules := make([]ApprovalRule, 0, len(s.approvalRules))
	for _, rule := range s.approvalRules {
		rules = append(rules, rule)
	}
	slices.SortFunc(rules, func(a, b ApprovalRule) int {
		if a.RuleID < b.RuleID {
			return -1
		}
		if a.RuleID > b.RuleID {
			return 1
		}
		return 0
	})
	return rules, nil
}

func (s *MemoryStore) SaveApprovalRecord(record ApprovalRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := approvalTargetKey(record.TargetType, record.TargetID)
	for _, existing := range s.approvals[key] {
		if existing.Approver.Subject == record.Approver.Subject && existing.Approver.Issuer == record.Approver.Issuer {
			return ErrConflict
		}
	}
	s.approvals[key] = append(s.approvals[key], record)
	return nil
}

func (s *MemoryStore) ListApprovalRecords(targetType string, targetID string) ([]ApprovalRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := s.approvals[approvalTargetKey(targetType, targetID)]
	copied := make([]ApprovalRecord, len(records))
	copy(copied, records)
	return copied, nil
}

func (s *MemoryStore) SaveAuthorityNegotiation(negotiation AuthorityNegotiation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.negotiations[negotiation.NegotiationID]; ok {
		return ErrConflict
	}
	s.negotiations[negotiation.NegotiationID] = negotiation
	return nil
}

func (s *MemoryStore) GetAuthorityNegotiation(id string) (AuthorityNegotiation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	negotiation, ok := s.negotiations[id]
	if !ok {
		return AuthorityNegotiation{}, ErrNotFound
	}
	return negotiation, nil
}

func (s *MemoryStore) SaveContainmentRule(rule ContainmentRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.containments[rule.RuleID]; ok {
		return ErrConflict
	}
	s.containments[rule.RuleID] = rule
	return nil
}

func (s *MemoryStore) GetContainmentRule(id string) (ContainmentRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rule, ok := s.containments[id]
	if !ok {
		return ContainmentRule{}, ErrNotFound
	}
	return rule, nil
}

func (s *MemoryStore) UpdateContainmentRule(rule ContainmentRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.containments[rule.RuleID]; !ok {
		return ErrNotFound
	}
	s.containments[rule.RuleID] = rule
	return nil
}

func (s *MemoryStore) ListContainmentRules() ([]ContainmentRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rules := make([]ContainmentRule, 0, len(s.containments))
	for _, rule := range s.containments {
		rules = append(rules, rule)
	}
	slices.SortFunc(rules, func(a, b ContainmentRule) int {
		if a.RuleID < b.RuleID {
			return -1
		}
		if a.RuleID > b.RuleID {
			return 1
		}
		return 0
	})
	return rules, nil
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

// PublishOutboxEvents returns empty for MemoryStore (no outbox in memory mode).
func (s *MemoryStore) PublishOutboxEvents() ([]OutboxEvent, error) {
	return nil, nil
}

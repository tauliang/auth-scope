package mission

import (
	"context"
	"time"
)

type GovernanceSnapshot struct {
	Missions          []Mission
	Projections       []Projection
	Leases            []MissionLease
	ExpansionRequests []ExpansionRequest
	Agents            []AgentIdentity
	ToolContracts     []ToolContract
}

type LineageSnapshot struct {
	Missions          []Mission
	ExpansionRequests []ExpansionRequest
	Projections       []Projection
	Leases            []MissionLease
	Identity          *AgentIdentity
}

type GovernanceReadStore interface {
	ListActiveContainmentRules(context.Context, time.Time) ([]ContainmentRule, error)
	LoadBlastRadiusSnapshot(context.Context, ContainmentRule) (GovernanceSnapshot, error)
	LoadMissionLineageSnapshot(context.Context, string) (LineageSnapshot, error)
	LoadAgentLineageSnapshot(context.Context, string) (LineageSnapshot, error)
}

func (s *MemoryStore) ListActiveContainmentRules(ctx context.Context, at time.Time) ([]ContainmentRule, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rules, err := s.ListContainmentRules()
	if err != nil {
		return nil, err
	}
	active := make([]ContainmentRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Status != ContainmentStatusActive {
			continue
		}
		if !rule.ExpiresAt.IsZero() && at.After(rule.ExpiresAt) {
			continue
		}
		active = append(active, rule)
	}
	return active, nil
}

func (s *MemoryStore) LoadBlastRadiusSnapshot(ctx context.Context, _ ContainmentRule) (GovernanceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return GovernanceSnapshot{}, err
	}
	missions, err := s.ListMissions()
	if err != nil {
		return GovernanceSnapshot{}, err
	}
	projections, err := s.ListProjections()
	if err != nil {
		return GovernanceSnapshot{}, err
	}
	leases, err := s.ListMissionLeases()
	if err != nil {
		return GovernanceSnapshot{}, err
	}
	expansions, err := s.ListExpansionRequests()
	if err != nil {
		return GovernanceSnapshot{}, err
	}
	agents, err := s.ListAgentIdentities()
	if err != nil {
		return GovernanceSnapshot{}, err
	}
	contracts, err := s.ListToolContracts()
	if err != nil {
		return GovernanceSnapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return GovernanceSnapshot{}, err
	}
	return GovernanceSnapshot{
		Missions:          missions,
		Projections:       projections,
		Leases:            leases,
		ExpansionRequests: expansions,
		Agents:            agents,
		ToolContracts:     contracts,
	}, nil
}

func (s *MemoryStore) LoadMissionLineageSnapshot(ctx context.Context, ref string) (LineageSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return LineageSnapshot{}, err
	}
	missions, err := s.ListMissions()
	if err != nil {
		return LineageSnapshot{}, err
	}
	related, ok := relatedMissionClosure(missions, ref)
	if !ok {
		return LineageSnapshot{}, ErrNotFound
	}
	return s.loadLineageArtifacts(ctx, sortedMissionMap(related), nil)
}

func (s *MemoryStore) LoadAgentLineageSnapshot(ctx context.Context, agentID string) (LineageSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return LineageSnapshot{}, err
	}
	identity, identityErr := s.GetAgentIdentity(agentID)
	target := Agent{InstanceID: agentID}
	var identityPtr *AgentIdentity
	if identityErr == nil {
		target = identity.Agent
		identityPtr = &identity
	} else if identityErr != ErrNotFound {
		return LineageSnapshot{}, identityErr
	}
	missions, err := s.ListMissions()
	if err != nil {
		return LineageSnapshot{}, err
	}
	related := make([]Mission, 0)
	for _, mission := range missions {
		if agentMatchesLineageTarget(mission.Agent, target, agentID) {
			related = append(related, mission)
		}
	}
	return s.loadLineageArtifacts(ctx, related, identityPtr)
}

func (s *MemoryStore) loadLineageArtifacts(ctx context.Context, missions []Mission, identity *AgentIdentity) (LineageSnapshot, error) {
	refs := make(map[string]struct{}, len(missions))
	for _, mission := range missions {
		refs[mission.MissionRef] = struct{}{}
	}
	expansions, err := s.ListExpansionRequests()
	if err != nil {
		return LineageSnapshot{}, err
	}
	projections, err := s.ListProjections()
	if err != nil {
		return LineageSnapshot{}, err
	}
	leases, err := s.ListMissionLeases()
	if err != nil {
		return LineageSnapshot{}, err
	}
	snapshot := LineageSnapshot{Missions: missions, Identity: identity}
	for _, expansion := range expansions {
		if _, ok := refs[expansion.MissionRef]; ok {
			snapshot.ExpansionRequests = append(snapshot.ExpansionRequests, expansion)
		}
	}
	for _, projection := range projections {
		if _, ok := refs[projection.MissionRef]; ok {
			snapshot.Projections = append(snapshot.Projections, projection)
		}
	}
	for _, lease := range leases {
		if _, ok := refs[lease.MissionRef]; ok {
			snapshot.Leases = append(snapshot.Leases, lease)
		}
	}
	if err := ctx.Err(); err != nil {
		return LineageSnapshot{}, err
	}
	return snapshot, nil
}

func relatedMissionClosure(missions []Mission, ref string) (map[string]Mission, bool) {
	target, ok := missionByRef(missions, ref)
	if !ok {
		return nil, false
	}
	related := map[string]Mission{target.MissionRef: target}
	changed := true
	for changed {
		changed = false
		for _, mission := range missions {
			if _, exists := related[mission.MissionRef]; exists {
				if mission.Delegation.ParentMissionRef != "" {
					if parent, ok := missionByRef(missions, mission.Delegation.ParentMissionRef); ok {
						if _, seen := related[parent.MissionRef]; !seen {
							related[parent.MissionRef] = parent
							changed = true
						}
					}
				}
				continue
			}
			if _, parentSeen := related[mission.Delegation.ParentMissionRef]; parentSeen {
				related[mission.MissionRef] = mission
				changed = true
			}
		}
	}
	return related, true
}

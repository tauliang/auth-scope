package mission

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

func (s *Service) CreateAuthorityNegotiation(ref string, req CreateAuthorityNegotiationRequest) (AuthorityNegotiation, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return AuthorityNegotiation{}, err
	}
	now := s.clock.Now()
	if err := ensureMissionUsableForActor(mission, req.Actor, req.MissionVersionSeen, now); err != nil {
		return AuthorityNegotiation{}, err
	}
	if err := validateRequestedAuthority(req.RequestedAuthority); err != nil {
		return AuthorityNegotiation{}, err
	}

	proposed, denied := authorityIntersection(mission.AuthorityRegion, req.RequestedAuthority)
	status := NegotiationStatusDenied
	rationale := []string{"requested authority is outside the active mission scope"}
	if allRequestedActionsInScope(mission.AuthorityRegion, req.RequestedAuthority) {
		status = NegotiationStatusAccepted
		proposed = req.RequestedAuthority
		denied = AuthorityRegion{}
		rationale = []string{"requested authority is already covered by the active mission"}
	} else if len(proposed.Resources) > 0 {
		status = NegotiationStatusCounteroffered
		rationale = []string{"safe subset can be granted without mission expansion", "remaining authority requires expansion or approval"}
	} else if highRisk(req.Context) || irreversible(req.Context) {
		status = NegotiationStatusRequiresApproval
		rationale = []string{"requested authority needs human approval before expansion"}
	}

	negotiation := AuthorityNegotiation{
		NegotiationID:      newID("mneg"),
		MissionRef:         mission.MissionRef,
		MissionVersion:     mission.Version,
		TenantID:           mission.TenantID,
		Actor:              req.Actor,
		RequestedAuthority: req.RequestedAuthority,
		ProposedAuthority:  proposed,
		DeniedAuthority:    denied,
		Status:             status,
		Rationale:          rationale,
		CreatedAt:          now,
	}
	if err := s.negotiations.SaveAuthorityNegotiation(negotiation); err != nil {
		return AuthorityNegotiation{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.authority_negotiated",
		Actor:         map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID, "key_thumbprint": req.Actor.KeyThumbprint},
		Payload:       map[string]any{"negotiation_id": negotiation.NegotiationID, "status": negotiation.Status},
		VersionBefore: mission.Version,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	})
	return negotiation, nil
}

func (s *Service) GetAuthorityNegotiation(id string) (AuthorityNegotiation, error) {
	if strings.TrimSpace(id) == "" {
		return AuthorityNegotiation{}, fmt.Errorf("negotiation_id is required")
	}
	return s.negotiations.GetAuthorityNegotiation(id)
}

func (s *Service) CreateContainmentRule(req ContainmentRule) (ContainmentRule, error) {
	rule := req
	rule.TargetType = strings.TrimSpace(rule.TargetType)
	rule.TargetID = strings.TrimSpace(rule.TargetID)
	if !supportedContainmentTarget(rule.TargetType) {
		return ContainmentRule{}, fmt.Errorf("unsupported containment target_type %q", rule.TargetType)
	}
	if rule.TargetID == "" {
		return ContainmentRule{}, fmt.Errorf("target_id is required")
	}
	if rule.RuleID == "" {
		rule.RuleID = newID("ctr")
	}
	if rule.Status == "" {
		rule.Status = ContainmentStatusActive
	}
	if rule.Status != ContainmentStatusActive {
		return ContainmentRule{}, fmt.Errorf("containment rule must be created active")
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = s.clock.Now()
	}
	if err := s.containments.SaveContainmentRule(rule); err != nil {
		return ContainmentRule{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
		TenantID:   rule.TenantID,
		Type:       "containment.rule_created",
		Actor:      map[string]any{"subject": rule.CreatedBy.Subject, "issuer": rule.CreatedBy.Issuer},
		Payload:    map[string]any{"rule_id": rule.RuleID, "target_type": rule.TargetType, "target_id": rule.TargetID, "reason": rule.Reason},
		OccurredAt: rule.CreatedAt,
	})
	return rule, nil
}

func (s *Service) ListContainmentRules() ([]ContainmentRule, error) {
	return s.containments.ListContainmentRules()
}

func (s *Service) LiftContainmentRule(id string, req StateChangeRequest) (ContainmentRule, error) {
	rule, err := s.containments.GetContainmentRule(id)
	if err != nil {
		return ContainmentRule{}, err
	}
	if rule.Status != ContainmentStatusLifted {
		rule.Status = ContainmentStatusLifted
		rule.LiftedAt = s.clock.Now()
		if err := s.containments.UpdateContainmentRule(rule); err != nil {
			return ContainmentRule{}, err
		}
		_ = s.events.AppendEvent(Event{
			EventID:     newID("mev"),
			TenantID:    rule.TenantID,
			Type:        "containment.rule_lifted",
			Actor:       req.Actor,
			Payload:     map[string]any{"rule_id": rule.RuleID, "reason": req.Reason},
			OccurredAt:  rule.LiftedAt,
			CausationID: req.CausationID,
		})
	}
	return rule, nil
}

func (s *Service) ContainmentBlastRadius(id string) (BlastRadius, error) {
	return s.ContainmentBlastRadiusContext(context.Background(), id)
}

func (s *Service) ContainmentBlastRadiusContext(ctx context.Context, id string) (BlastRadius, error) {
	rule, err := s.containments.GetContainmentRule(id)
	if err != nil {
		return BlastRadius{}, err
	}
	snapshot, err := s.governanceReads.LoadBlastRadiusSnapshot(ctx, rule)
	if err != nil {
		return BlastRadius{}, err
	}

	radius := BlastRadius{Rule: rule}
	for _, mission := range snapshot.Missions {
		if containmentRuleMatchesMission(rule, mission) {
			radius.Missions = append(radius.Missions, mission)
		}
	}
	for _, projection := range snapshot.Projections {
		if containmentRuleMatchesProjection(rule, projection, snapshot.Missions) {
			radius.Projections = append(radius.Projections, projection)
		}
	}
	for _, lease := range snapshot.Leases {
		if containmentRuleMatchesLease(rule, lease, snapshot.Missions) {
			radius.Leases = append(radius.Leases, lease)
		}
	}
	for _, expansion := range snapshot.ExpansionRequests {
		if containmentRuleMatchesExpansion(rule, expansion, snapshot.Missions) {
			radius.ExpansionRequests = append(radius.ExpansionRequests, expansion)
		}
	}
	for _, identity := range snapshot.Agents {
		if containmentRuleMatchesIdentity(rule, identity) {
			radius.Agents = append(radius.Agents, identity)
		}
	}
	for _, contract := range snapshot.ToolContracts {
		if containmentRuleMatchesToolContract(rule, contract) {
			radius.ToolContracts = append(radius.ToolContracts, contract)
		}
	}
	return radius, nil
}

func (s *Service) MissionLineage(ref string) (LineageGraph, error) {
	return s.MissionLineageContext(context.Background(), ref)
}

func (s *Service) MissionLineageContext(ctx context.Context, ref string) (LineageGraph, error) {
	snapshot, err := s.governanceReads.LoadMissionLineageSnapshot(ctx, ref)
	if err != nil {
		return LineageGraph{}, err
	}
	related := make(map[string]Mission, len(snapshot.Missions))
	for _, mission := range snapshot.Missions {
		related[mission.MissionRef] = mission
	}
	builder := newLineageBuilder()
	for _, mission := range sortedMissionMap(related) {
		addMissionToLineage(builder, mission)
		if mission.Delegation.ParentMissionRef != "" {
			builder.edge(lineageMissionID(mission.Delegation.ParentMissionRef), lineageMissionID(mission.MissionRef), "delegated", nil)
		}
	}
	addMissionArtifactsToLineage(builder, snapshot, related)
	return builder.graph(), nil
}

func (s *Service) AgentLineage(agentID string) (LineageGraph, error) {
	return s.AgentLineageContext(context.Background(), agentID)
}

func (s *Service) AgentLineageContext(ctx context.Context, agentID string) (LineageGraph, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return LineageGraph{}, fmt.Errorf("agent_id is required")
	}
	snapshot, err := s.governanceReads.LoadAgentLineageSnapshot(ctx, agentID)
	if err != nil {
		return LineageGraph{}, err
	}

	builder := newLineageBuilder()
	if snapshot.Identity != nil {
		identity := *snapshot.Identity
		builder.node(lineageAgentID(identity.Agent), "agent", identity.AgentID, map[string]any{"agent_id": identity.AgentID, "status": identity.Status})
	} else {
		builder.node("agent:"+agentID, "agent", agentID, nil)
	}

	related := map[string]Mission{}
	for _, mission := range snapshot.Missions {
		related[mission.MissionRef] = mission
		addMissionToLineage(builder, mission)
	}
	addMissionArtifactsToLineage(builder, snapshot, related)
	return builder.graph(), nil
}

func addMissionArtifactsToLineage(builder *lineageBuilder, snapshot LineageSnapshot, related map[string]Mission) {
	for _, expansion := range snapshot.ExpansionRequests {
		if _, ok := related[expansion.MissionRef]; !ok {
			continue
		}
		nodeID := "expansion:" + expansion.ExpansionID
		builder.node(nodeID, "expansion_request", expansion.ExpansionID, map[string]any{"status": expansion.Status})
		builder.edge(lineageMissionID(expansion.MissionRef), nodeID, "requested_expansion", map[string]any{"operation": expansion.Action.Operation})
	}

	for _, projection := range snapshot.Projections {
		if _, ok := related[projection.MissionRef]; !ok {
			continue
		}
		nodeID := "projection:" + projection.ProjectionID
		builder.node(nodeID, "projection", projection.ProjectionID, map[string]any{"status": projection.Status, "type": projection.Type})
		builder.edge(lineageMissionID(projection.MissionRef), nodeID, "issued_projection", nil)
	}

	for _, lease := range snapshot.Leases {
		if _, ok := related[lease.MissionRef]; !ok {
			continue
		}
		nodeID := "lease:" + lease.LeaseID
		builder.node(nodeID, "mission_lease", lease.LeaseID, map[string]any{"expires_at": lease.ExpiresAt})
		builder.edge(lineageMissionID(lease.MissionRef), nodeID, "leased", nil)
	}
}

func (s *Service) matchingActiveContainmentForEvaluation(mission Mission, actor Actor, action Action, now time.Time) (ContainmentRule, bool, error) {
	return s.authorityGuard.Check(context.Background(), AuthorityOperation{
		Mission: &mission,
		Actor:   actor,
		Action:  action,
		At:      now,
	})
}

func (s *Service) matchingActiveContainmentForProjection(projection Projection, now time.Time) (ContainmentRule, bool, error) {
	return s.authorityGuard.Check(context.Background(), AuthorityOperation{
		Projection: &projection,
		Actor:      projection.Actor,
		At:         now,
	})
}

func validateRequestedAuthority(authority AuthorityRegion) error {
	if len(authority.Resources) == 0 {
		return fmt.Errorf("requested_authority.resources is required")
	}
	for _, grant := range authority.Resources {
		if grant.Type == "" || grant.ID == "" || len(grant.Actions) == 0 {
			return fmt.Errorf("requested authority grants require type, id, and actions")
		}
	}
	return nil
}

func authorityIntersection(parent AuthorityRegion, requested AuthorityRegion) (AuthorityRegion, AuthorityRegion) {
	var proposed AuthorityRegion
	var denied AuthorityRegion
	proposed.ForbiddenActions = append([]string(nil), parent.ForbiddenActions...)
	denied.ForbiddenActions = append([]string(nil), requested.ForbiddenActions...)
	for _, grant := range requested.Resources {
		allowedGrant := ResourceGrant{Type: grant.Type, ID: grant.ID, Constraints: grant.Constraints}
		deniedGrant := ResourceGrant{Type: grant.Type, ID: grant.ID, Constraints: grant.Constraints}
		for _, action := range grant.Actions {
			if actionInScope(parent, Action{Resource: ActionResource{Type: grant.Type, ID: grant.ID}, Operation: action, Name: action}) {
				allowedGrant.Actions = append(allowedGrant.Actions, action)
			} else {
				deniedGrant.Actions = append(deniedGrant.Actions, action)
			}
		}
		if len(allowedGrant.Actions) > 0 {
			proposed.Resources = append(proposed.Resources, allowedGrant)
		}
		if len(deniedGrant.Actions) > 0 {
			denied.Resources = append(denied.Resources, deniedGrant)
		}
	}
	return proposed, denied
}

func allRequestedActionsInScope(parent AuthorityRegion, requested AuthorityRegion) bool {
	for _, grant := range requested.Resources {
		for _, action := range grant.Actions {
			if !actionInScope(parent, Action{Resource: ActionResource{Type: grant.Type, ID: grant.ID}, Operation: action, Name: action}) {
				return false
			}
		}
	}
	return true
}

func supportedContainmentTarget(targetType string) bool {
	switch targetType {
	case ContainmentTargetAgent, ContainmentTargetPrincipal, ContainmentTargetTool, ContainmentTargetResource, ContainmentTargetMission, ContainmentTargetTenant:
		return true
	default:
		return false
	}
}

func containmentRuleMatchesEvaluation(rule ContainmentRule, mission Mission, actor Actor, action Action) bool {
	if rule.TenantID != "" && rule.TenantID != mission.TenantID {
		return false
	}
	if containmentRuleMatchesMission(rule, mission) {
		return true
	}
	switch rule.TargetType {
	case ContainmentTargetAgent:
		return targetMatches(rule.TargetID, actor.AgentInstanceID, actor.ClientID, mission.Agent.InstanceID, mission.Agent.ClientID)
	case ContainmentTargetPrincipal:
		return targetMatches(rule.TargetID, mission.Principal.Subject, mission.Principal.TenantSubject)
	case ContainmentTargetTool:
		return action.Name != "" && targetMatches(rule.TargetID, action.Name)
	case ContainmentTargetResource:
		return resourceTargetMatches(rule.TargetID, action.Resource)
	default:
		return false
	}
}

func containmentRuleMatchesMission(rule ContainmentRule, mission Mission) bool {
	if rule.TenantID != "" && rule.TenantID != mission.TenantID {
		return false
	}
	switch rule.TargetType {
	case ContainmentTargetTenant:
		return targetMatches(rule.TargetID, mission.TenantID)
	case ContainmentTargetMission:
		return targetMatches(rule.TargetID, mission.MissionRef, mission.MissionID)
	case ContainmentTargetAgent:
		return targetMatches(rule.TargetID, mission.Agent.InstanceID, mission.Agent.ClientID)
	case ContainmentTargetPrincipal:
		return targetMatches(rule.TargetID, mission.Principal.Subject, mission.Principal.TenantSubject)
	default:
		return false
	}
}

func containmentRuleMatchesProjection(rule ContainmentRule, projection Projection, missions []Mission) bool {
	if rule.TenantID != "" && rule.TenantID != projection.TenantID {
		return false
	}
	switch rule.TargetType {
	case ContainmentTargetTenant:
		return targetMatches(rule.TargetID, projection.TenantID)
	case ContainmentTargetMission:
		return targetMatches(rule.TargetID, projection.MissionRef)
	case ContainmentTargetAgent:
		return targetMatches(rule.TargetID, projection.Actor.AgentInstanceID, projection.Actor.ClientID)
	}
	if mission, ok := missionByRef(missions, projection.MissionRef); ok {
		return containmentRuleMatchesMission(rule, mission)
	}
	return false
}

func containmentRuleMatchesLease(rule ContainmentRule, lease MissionLease, missions []Mission) bool {
	if rule.TenantID != "" && rule.TenantID != lease.TenantID {
		return false
	}
	switch rule.TargetType {
	case ContainmentTargetTenant:
		return targetMatches(rule.TargetID, lease.TenantID)
	case ContainmentTargetMission:
		return targetMatches(rule.TargetID, lease.MissionRef)
	case ContainmentTargetAgent:
		return targetMatches(rule.TargetID, lease.Actor.AgentInstanceID, lease.Actor.ClientID)
	}
	if mission, ok := missionByRef(missions, lease.MissionRef); ok {
		return containmentRuleMatchesMission(rule, mission)
	}
	return false
}

func containmentRuleMatchesExpansion(rule ContainmentRule, expansion ExpansionRequest, missions []Mission) bool {
	if rule.TenantID != "" && rule.TenantID != expansion.TenantID {
		return false
	}
	switch rule.TargetType {
	case ContainmentTargetTenant:
		return targetMatches(rule.TargetID, expansion.TenantID)
	case ContainmentTargetMission:
		return targetMatches(rule.TargetID, expansion.MissionRef)
	case ContainmentTargetAgent:
		return targetMatches(rule.TargetID, expansion.Requester.AgentInstanceID, expansion.Requester.ClientID)
	case ContainmentTargetTool:
		return targetMatches(rule.TargetID, expansion.Action.Name)
	case ContainmentTargetResource:
		return resourceTargetMatches(rule.TargetID, expansion.Action.Resource)
	}
	if mission, ok := missionByRef(missions, expansion.MissionRef); ok {
		return containmentRuleMatchesMission(rule, mission)
	}
	return false
}

func containmentRuleMatchesIdentity(rule ContainmentRule, identity AgentIdentity) bool {
	if rule.TenantID != "" && rule.TenantID != identity.TenantID {
		return false
	}
	switch rule.TargetType {
	case ContainmentTargetTenant:
		return targetMatches(rule.TargetID, identity.TenantID)
	case ContainmentTargetAgent:
		return targetMatches(rule.TargetID, identity.AgentID, identity.Agent.InstanceID, identity.Agent.ClientID)
	default:
		return false
	}
}

func containmentRuleMatchesToolContract(rule ContainmentRule, contract ToolContract) bool {
	switch rule.TargetType {
	case ContainmentTargetTool:
		return targetMatches(rule.TargetID, contract.ToolName)
	case ContainmentTargetResource:
		return resourceTargetMatches(rule.TargetID, ActionResource{Type: contract.ResourceType, ID: contract.ResourceID})
	default:
		return false
	}
}

func targetMatches(target string, values ...string) bool {
	for _, value := range values {
		if value != "" && target == value {
			return true
		}
	}
	return false
}

func resourceTargetMatches(target string, resource ActionResource) bool {
	if resource.Type == "" && resource.ID == "" {
		return false
	}
	return targetMatches(target, resource.ID, resource.Type+":"+resource.ID, resource.Type+"/"+resource.ID, resource.Type+":*", resource.Type+"/*")
}

func missionByRef(missions []Mission, ref string) (Mission, bool) {
	for _, mission := range missions {
		if mission.MissionRef == ref {
			return mission, true
		}
	}
	return Mission{}, false
}

func sortedMissionMap(missions map[string]Mission) []Mission {
	out := make([]Mission, 0, len(missions))
	for _, mission := range missions {
		out = append(out, mission)
	}
	slices.SortFunc(out, func(a, b Mission) int {
		if a.MissionRef < b.MissionRef {
			return -1
		}
		if a.MissionRef > b.MissionRef {
			return 1
		}
		return 0
	})
	return out
}

type lineageBuilder struct {
	nodes []LineageNode
	edges []LineageEdge
	seen  map[string]bool
}

func newLineageBuilder() *lineageBuilder {
	return &lineageBuilder{seen: make(map[string]bool)}
}

func (b *lineageBuilder) node(id string, nodeType string, label string, metadata map[string]any) {
	if id == "" || b.seen["node:"+id] {
		return
	}
	b.seen["node:"+id] = true
	b.nodes = append(b.nodes, LineageNode{ID: id, Type: nodeType, Label: label, Metadata: metadata})
}

func (b *lineageBuilder) edge(from string, to string, edgeType string, metadata map[string]any) {
	if from == "" || to == "" {
		return
	}
	key := "edge:" + from + "\x00" + to + "\x00" + edgeType
	if b.seen[key] {
		return
	}
	b.seen[key] = true
	b.edges = append(b.edges, LineageEdge{From: from, To: to, Type: edgeType, Metadata: metadata})
}

func (b *lineageBuilder) graph() LineageGraph {
	return LineageGraph{Nodes: b.nodes, Edges: b.edges}
}

func addMissionToLineage(builder *lineageBuilder, mission Mission) {
	missionNode := lineageMissionID(mission.MissionRef)
	principalNode := "principal:" + mission.Principal.Subject
	agentNode := lineageAgentID(mission.Agent)
	builder.node(principalNode, "principal", mission.Principal.Subject, map[string]any{"issuer": mission.Principal.Issuer})
	builder.node(agentNode, "agent", mission.Agent.InstanceID, map[string]any{"client_id": mission.Agent.ClientID})
	builder.node(missionNode, "mission", mission.MissionRef, map[string]any{"state": mission.State, "version": mission.Version, "tenant_id": mission.TenantID})
	builder.edge(principalNode, missionNode, "authorized", nil)
	builder.edge(missionNode, agentNode, "assigned_agent", nil)
}

func lineageMissionID(ref string) string {
	return "mission:" + ref
}

func lineageAgentID(agent Agent) string {
	if agent.InstanceID != "" {
		return "agent:" + agent.InstanceID
	}
	return "agent:" + agent.ClientID
}

func agentMatchesLineageTarget(agent Agent, target Agent, fallbackID string) bool {
	return targetMatches(fallbackID, agent.InstanceID, agent.ClientID) ||
		(target.InstanceID != "" && target.InstanceID == agent.InstanceID) ||
		(target.ClientID != "" && target.ClientID == agent.ClientID)
}

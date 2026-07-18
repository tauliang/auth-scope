package mission

import (
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func (s *Service) OperationsSummary(query ListQuery) (OperationsSummary, error) {
	missions, err := s.missions.ListMissions()
	if err != nil {
		return OperationsSummary{}, err
	}
	proposals, err := s.missions.ListProposals()
	if err != nil {
		return OperationsSummary{}, err
	}
	expansions, err := s.governance.ListExpansionRequests()
	if err != nil {
		return OperationsSummary{}, err
	}
	containments, err := s.containments.ListContainmentRules()
	if err != nil {
		return OperationsSummary{}, err
	}
	agents, err := s.identities.ListAgentIdentities()
	if err != nil {
		return OperationsSummary{}, err
	}
	projections, err := s.projections.ListProjections()
	if err != nil {
		return OperationsSummary{}, err
	}

	summary := OperationsSummary{
		MissionsByState: make(map[State]int),
		ServiceCapabilities: map[string]bool{
			"approvals":   true,
			"containment": true,
			"lineage":     true,
			"projections": true,
		},
	}
	for _, item := range missions {
		if !tenantMatches(query.TenantID, item.TenantID) {
			continue
		}
		summary.MissionsTotal++
		summary.MissionsByState[item.State]++
	}
	for _, item := range proposals {
		if tenantMatches(query.TenantID, item.TenantID) && item.Status == StatePendingApproval {
			summary.PendingProposals++
		}
	}
	for _, item := range expansions {
		if tenantMatches(query.TenantID, item.TenantID) && item.Status == ExpansionStatusPending {
			summary.PendingExpansions++
		}
	}
	for _, item := range containments {
		if tenantMatches(query.TenantID, item.TenantID) && item.Status == ContainmentStatusActive {
			summary.ActiveContainments++
		}
	}
	for _, item := range agents {
		if tenantMatches(query.TenantID, item.TenantID) && item.Status == AgentStatusActive {
			summary.ActiveAgents++
		}
	}
	for _, item := range projections {
		if tenantMatches(query.TenantID, item.TenantID) && item.Status == ProjectionStatusActive {
			summary.ActiveProjections++
		}
	}
	for _, item := range s.events.Events() {
		if tenantMatches(query.TenantID, item.TenantID) {
			summary.RecentEventCount++
		}
	}
	return summary, nil
}

func (s *Service) ListMissions(query ListQuery) (CollectionPage[Mission], error) {
	items, err := s.missions.ListMissions()
	if err != nil {
		return CollectionPage[Mission]{}, err
	}
	filtered := make([]Mission, 0, len(items))
	for _, item := range items {
		if tenantMatches(query.TenantID, item.TenantID) &&
			valueMatches(query.State, string(item.State)) &&
			textMatches(query.Query, item.MissionRef, item.Purpose.Objective, item.Principal.Subject, item.Agent.ClientID, item.Agent.InstanceID) {
			filtered = append(filtered, item)
		}
	}
	slices.SortFunc(filtered, func(a, b Mission) int { return strings.Compare(b.MissionRef, a.MissionRef) })
	return paginate(filtered, query)
}

func (s *Service) ListProposals(query ListQuery) (CollectionPage[MissionProposal], error) {
	items, err := s.missions.ListProposals()
	if err != nil {
		return CollectionPage[MissionProposal]{}, err
	}
	filtered := make([]MissionProposal, 0, len(items))
	for _, item := range items {
		if tenantMatches(query.TenantID, item.TenantID) &&
			valueMatches(query.State, string(item.Status)) &&
			textMatches(query.Query, item.ProposalID, item.Intent.Objective, item.Principal.Subject, item.Agent.ClientID) {
			filtered = append(filtered, item)
		}
	}
	slices.Reverse(filtered)
	return paginate(filtered, query)
}

func (s *Service) GetProposal(id string) (MissionProposal, error) {
	return s.missions.GetProposal(id)
}

func (s *Service) ListExpansions(query ListQuery) (CollectionPage[ExpansionRequest], error) {
	items, err := s.governance.ListExpansionRequests()
	if err != nil {
		return CollectionPage[ExpansionRequest]{}, err
	}
	filtered := make([]ExpansionRequest, 0, len(items))
	for _, item := range items {
		if tenantMatches(query.TenantID, item.TenantID) &&
			valueMatches(query.Status, item.Status) &&
			textMatches(query.Query, item.ExpansionID, item.MissionRef, item.Justification, item.Action.Operation, item.Action.Resource.Type, item.Action.Resource.ID) {
			filtered = append(filtered, item)
		}
	}
	slices.Reverse(filtered)
	return paginate(filtered, query)
}

func (s *Service) ListAgents(query ListQuery) (CollectionPage[AgentIdentity], error) {
	items, err := s.identities.ListAgentIdentities()
	if err != nil {
		return CollectionPage[AgentIdentity]{}, err
	}
	filtered := make([]AgentIdentity, 0, len(items))
	for _, item := range items {
		if tenantMatches(query.TenantID, item.TenantID) &&
			valueMatches(query.Status, item.Status) &&
			textMatches(query.Query, item.AgentID, item.Agent.Provider, item.Agent.ClientID, item.Agent.InstanceID, item.KeyThumbprint) {
			filtered = append(filtered, item)
		}
	}
	slices.Reverse(filtered)
	return paginate(filtered, query)
}

func (s *Service) ListToolContracts(query ListQuery) (CollectionPage[ToolContract], error) {
	items, err := s.governance.ListToolContracts()
	if err != nil {
		return CollectionPage[ToolContract]{}, err
	}
	filtered := make([]ToolContract, 0, len(items))
	for _, item := range items {
		if textMatches(query.Query, item.ToolName, item.ResourceType, item.ResourceID, item.Operation) {
			filtered = append(filtered, item)
		}
	}
	slices.Reverse(filtered)
	return paginate(filtered, query)
}

func (s *Service) ListProjections(query ListQuery) (CollectionPage[Projection], error) {
	items, err := s.projections.ListProjections()
	if err != nil {
		return CollectionPage[Projection]{}, err
	}
	filtered := make([]Projection, 0, len(items))
	for _, item := range items {
		if tenantMatches(query.TenantID, item.TenantID) &&
			valueMatches(query.Status, item.Status) &&
			valueMatches(query.Type, item.Type) &&
			textMatches(query.Query, item.ProjectionID, item.MissionRef, item.Actor.ClientID, item.Actor.AgentInstanceID) {
			filtered = append(filtered, item)
		}
	}
	slices.Reverse(filtered)
	return paginate(filtered, query)
}

func (s *Service) GetContainmentRule(id string) (ContainmentRule, error) {
	return s.containments.GetContainmentRule(id)
}

func (s *Service) ListEvents(query ListQuery) (CollectionPage[Event], error) {
	items := s.events.Events()
	filtered := make([]Event, 0, len(items))
	for _, item := range items {
		if tenantMatches(query.TenantID, item.TenantID) &&
			valueMatches(query.Type, item.Type) &&
			textMatches(query.Query, item.EventID, item.MissionRef, item.Type, item.CorrelationID, item.CausationID) {
			filtered = append(filtered, item)
		}
	}
	slices.Reverse(filtered)
	page, err := paginate(filtered, query)
	if err != nil {
		return CollectionPage[Event]{}, err
	}
	return page, nil
}

func paginate[T any](items []T, query ListQuery) (CollectionPage[T], error) {
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultCollectionLimit
	}
	if limit > MaxCollectionLimit {
		limit = MaxCollectionLimit
	}
	start := 0
	if query.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil {
			return CollectionPage[T]{}, fmt.Errorf("invalid cursor")
		}
		start, err = strconv.Atoi(string(raw))
		if err != nil || start < 0 || start > len(items) {
			return CollectionPage[T]{}, fmt.Errorf("invalid cursor")
		}
	}
	end := min(start+limit, len(items))
	pageItems := make([]T, end-start)
	copy(pageItems, items[start:end])
	page := CollectionPage[T]{Items: pageItems, Total: len(items)}
	if end < len(items) {
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	return page, nil
}

func tenantMatches(filter string, value string) bool {
	return filter == "" || strings.EqualFold(strings.TrimSpace(filter), value)
}

func valueMatches(filter string, value string) bool {
	return filter == "" || strings.EqualFold(strings.TrimSpace(filter), value)
}

func textMatches(query string, values ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

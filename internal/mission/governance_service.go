package mission

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) CreateExpansionRequest(ref string, req CreateExpansionRequest) (ExpansionRequest, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return ExpansionRequest{}, err
	}
	return s.createExpansionRequestForMission(mission, req, s.clock.Now())
}

func (s *Service) GetExpansionRequest(id string) (ExpansionRequest, error) {
	if strings.TrimSpace(id) == "" {
		return ExpansionRequest{}, fmt.Errorf("expansion_id is required")
	}
	return s.governance.GetExpansionRequest(id)
}

func (s *Service) ApproveExpansionRequest(id string, req ExpansionDecisionRequest) (ExpansionDecisionResponse, error) {
	return s.ApproveExpansionRequestContext(context.Background(), id, req)
}

func (s *Service) ApproveExpansionRequestContext(ctx context.Context, id string, req ExpansionDecisionRequest) (ExpansionDecisionResponse, error) {
	return s.decideExpansionRequest(ctx, id, req, ExpansionStatusApproved, true)
}

func (s *Service) DenyExpansionRequest(id string, req ExpansionDecisionRequest) (ExpansionDecisionResponse, error) {
	return s.DenyExpansionRequestContext(context.Background(), id, req)
}

func (s *Service) DenyExpansionRequestContext(ctx context.Context, id string, req ExpansionDecisionRequest) (ExpansionDecisionResponse, error) {
	return s.decideExpansionRequest(ctx, id, req, ExpansionStatusDenied, false)
}

func (s *Service) VerifyDecisionArtifactEvidence(req VerifyDecisionArtifactRequest) VerifyDecisionArtifactResponse {
	if strings.TrimSpace(req.DecisionArtifact) == "" {
		return VerifyDecisionArtifactResponse{Valid: false, Error: "decision_artifact is required"}
	}
	payload, err := VerifyDecisionArtifact(req.DecisionArtifact, s.artifactKey)
	if err != nil {
		return VerifyDecisionArtifactResponse{Valid: false, Error: err.Error()}
	}
	resp := VerifyDecisionArtifactResponse{Valid: true, Payload: payload}
	if payload.EvidenceID == "" {
		return resp
	}
	evidence, err := s.governance.GetEvaluationEvidence(payload.EvidenceID)
	if errors.Is(err, ErrNotFound) {
		return resp
	}
	if err != nil {
		return VerifyDecisionArtifactResponse{Valid: false, Payload: payload, Error: err.Error()}
	}
	resp.Evidence = &evidence
	return resp
}

func (s *Service) RegisterToolContract(contract ToolContract) (ToolContract, error) {
	contract.ToolName = strings.TrimSpace(contract.ToolName)
	contract.ResourceType = strings.TrimSpace(contract.ResourceType)
	contract.ResourceID = strings.TrimSpace(contract.ResourceID)
	contract.ResourceIDParam = strings.TrimSpace(contract.ResourceIDParam)
	contract.Operation = strings.TrimSpace(contract.Operation)
	contract.OperationParam = strings.TrimSpace(contract.OperationParam)
	contract.ActionType = strings.TrimSpace(contract.ActionType)
	if contract.ToolName == "" {
		return ToolContract{}, fmt.Errorf("tool_name is required")
	}
	if contract.ResourceType == "" {
		return ToolContract{}, fmt.Errorf("resource_type is required")
	}
	if contract.ResourceID == "" && contract.ResourceIDParam == "" {
		return ToolContract{}, fmt.Errorf("resource_id or resource_id_param is required")
	}
	if contract.Operation == "" && contract.OperationParam == "" {
		return ToolContract{}, fmt.Errorf("operation or operation_param is required")
	}
	if contract.ActionType == "" {
		contract.ActionType = "tool_call"
	}
	if contract.CreatedAt.IsZero() {
		contract.CreatedAt = s.clock.Now()
	}
	if err := s.governance.SaveToolContract(contract); err != nil {
		return ToolContract{}, err
	}
	return contract, nil
}

func (s *Service) GetToolContract(toolName string) (ToolContract, error) {
	if strings.TrimSpace(toolName) == "" {
		return ToolContract{}, fmt.Errorf("tool_name is required")
	}
	return s.governance.GetToolContract(toolName)
}

func (s *Service) AuthorizeToolCall(req AuthorizeToolCallRequest) (AuthorizeToolCallResponse, error) {
	contract, err := s.governance.GetToolContract(req.ToolName)
	if err != nil {
		return AuthorizeToolCallResponse{}, err
	}
	if missing := missingContextKeys(req.Context, contract.RequiredContext); len(missing) > 0 {
		return AuthorizeToolCallResponse{
			ToolName:    req.ToolName,
			Decision:    DecisionDeny,
			MissionRef:  req.MissionRef,
			ReasonCodes: []string{"REQUIRED_CONTEXT_MISSING"},
			HumanReason: "The tool call is missing required policy context.",
			Constraints: map[string]any{"missing_context": missing},
		}, nil
	}

	resourceID := contract.ResourceID
	if resourceID == "" {
		value, ok := stringFromMap(req.Arguments, contract.ResourceIDParam)
		if !ok {
			return toolArgumentDeny(req, contract.ResourceIDParam), nil
		}
		resourceID = value
	}
	operation := contract.Operation
	if operation == "" {
		value, ok := stringFromMap(req.Arguments, contract.OperationParam)
		if !ok {
			return toolArgumentDeny(req, contract.OperationParam), nil
		}
		operation = value
	}

	decision, err := s.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              req.Actor,
		Action: Action{
			Type:      contract.ActionType,
			Name:      contract.ToolName,
			Resource:  ActionResource{Type: contract.ResourceType, ID: resourceID},
			Operation: operation,
		},
		Context: req.Context,
	})
	if err != nil {
		return AuthorizeToolCallResponse{}, err
	}
	return AuthorizeToolCallResponse{
		ToolName:         req.ToolName,
		Decision:         decision.Decision,
		MissionRef:       decision.MissionRef,
		MissionVersion:   decision.MissionVersion,
		ReasonCodes:      decision.ReasonCodes,
		HumanReason:      decision.HumanReason,
		DecisionArtifact: decision.DecisionArtifact,
		Constraints:      decision.Constraints,
		Escalation:       decision.Escalation,
	}, nil
}

func (s *Service) createExpansionRequestForMission(mission Mission, req CreateExpansionRequest, now time.Time) (ExpansionRequest, error) {
	if mission.State != StateActive {
		return ExpansionRequest{}, fmt.Errorf("mission is not active")
	}
	if !actorMatches(mission, req.Requester) {
		return ExpansionRequest{}, fmt.Errorf("requester is not authorized for this mission")
	}
	if req.MissionVersionSeen > 0 && req.MissionVersionSeen < mission.Version {
		return ExpansionRequest{}, fmt.Errorf("mission version is stale")
	}
	if rule, ok, err := s.matchingActiveContainmentForEvaluation(mission, req.Requester, req.Action, now); err != nil {
		return ExpansionRequest{}, err
	} else if ok {
		return ExpansionRequest{}, fmt.Errorf("expansion blocked by containment rule %s", rule.RuleID)
	}
	requestedAuthority := req.RequestedAuthority
	if len(requestedAuthority.Resources) == 0 {
		requestedAuthority = authorityForAction(req.Action)
	}
	if len(requestedAuthority.Resources) == 0 {
		return ExpansionRequest{}, fmt.Errorf("requested_authority.resources is required")
	}
	expansion := ExpansionRequest{
		ExpansionID:        newID("mex"),
		MissionRef:         mission.MissionRef,
		MissionVersionSeen: effectiveMissionVersionSeen(req.MissionVersionSeen, mission.Version),
		TenantID:           mission.TenantID,
		Requester:          req.Requester,
		Action:             req.Action,
		Context:            req.Context,
		RequestedAuthority: requestedAuthority,
		Justification:      strings.TrimSpace(req.Justification),
		Status:             ExpansionStatusPending,
		CreatedAt:          now,
	}
	if err := s.governance.SaveExpansionRequest(expansion); err != nil {
		return ExpansionRequest{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.expansion_requested",
		Actor:         map[string]any{"agent_instance_id": req.Requester.AgentInstanceID, "client_id": req.Requester.ClientID, "key_thumbprint": req.Requester.KeyThumbprint},
		Payload:       map[string]any{"expansion_id": expansion.ExpansionID, "operation": req.Action.Operation, "resource_type": req.Action.Resource.Type, "resource_id": req.Action.Resource.ID},
		VersionBefore: mission.Version,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	})
	return expansion, nil
}

func (s *Service) decideExpansionRequest(ctx context.Context, id string, req ExpansionDecisionRequest, status string, enforceApprovalRules bool) (ExpansionDecisionResponse, error) {
	if req.Approver.Subject == "" {
		return ExpansionDecisionResponse{}, fmt.Errorf("approver.subject is required")
	}
	expansion, err := s.governance.GetExpansionRequest(id)
	if err != nil {
		return ExpansionDecisionResponse{}, err
	}
	if expansion.Status != ExpansionStatusPending {
		return ExpansionDecisionResponse{}, fmt.Errorf("expansion request is already %s", expansion.Status)
	}
	if status == ExpansionStatusApproved && enforceApprovalRules {
		if rule, ok, err := s.matchingApprovalRule(expansion); err != nil {
			return ExpansionDecisionResponse{}, err
		} else if ok {
			return ExpansionDecisionResponse{}, fmt.Errorf("approval rule %s requires %d approval submissions", rule.RuleID, rule.RequiredApprovals)
		}
	}
	mission, err := s.missions.GetMission(expansion.MissionRef)
	if err != nil {
		return ExpansionDecisionResponse{}, err
	}
	now := s.clock.Now()
	versionBefore := mission.Version
	if status == ExpansionStatusApproved {
		if mission.State != StateActive {
			return ExpansionDecisionResponse{}, fmt.Errorf("mission is not active")
		}
		if expansion.MissionVersionSeen > 0 && expansion.MissionVersionSeen != mission.Version {
			return ExpansionDecisionResponse{}, fmt.Errorf("mission version changed since expansion request")
		}
		if authorityConflictsWithForbiddenActions(mission.AuthorityRegion, expansion.RequestedAuthority) {
			return ExpansionDecisionResponse{}, fmt.Errorf("requested authority conflicts with forbidden actions")
		}
		if err := s.ensureExpansionApprovalNotContained(ctx, mission, expansion, now); err != nil {
			return ExpansionDecisionResponse{}, err
		}
		mission.AuthorityRegion = mergeAuthority(mission.AuthorityRegion, expansion.RequestedAuthority)
		mission.Version++
	}

	evidence := req.ApprovalEvidence
	evidence.Approver = req.Approver
	if evidence.ApprovedAt.IsZero() {
		evidence.ApprovedAt = now
	}
	expansion.Status = status
	expansion.DecidedAt = now
	expansion.Approver = req.Approver
	expansion.DecisionReason = strings.TrimSpace(req.Reason)
	expansion.ApprovalEvidence = evidence
	event := Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.expansion_" + status,
		Actor:         map[string]any{"subject": req.Approver.Subject, "issuer": req.Approver.Issuer},
		Payload:       map[string]any{"expansion_id": expansion.ExpansionID, "reason": req.Reason},
		VersionBefore: versionBefore,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	}
	commit := ExpansionDecisionCommit{
		Expansion:               expansion,
		ExpectedExpansionStatus: ExpansionStatusPending,
		Event:                   event,
	}
	if status == ExpansionStatusApproved {
		commit.Mission = &mission
		commit.ExpectedMissionVersion = versionBefore
	}
	if err := s.expansionDecisions.CommitExpansionDecision(ctx, commit); err != nil {
		return ExpansionDecisionResponse{}, err
	}
	return ExpansionDecisionResponse{
		ExpansionID:    expansion.ExpansionID,
		Status:         expansion.Status,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
	}, nil
}

func (s *Service) ensureExpansionApprovalNotContained(ctx context.Context, mission Mission, expansion ExpansionRequest, now time.Time) error {
	rule, contained, err := s.authorityGuard.Check(ctx, AuthorityOperation{
		Mission: &mission,
		Actor:   expansion.Requester,
		Action:  expansion.Action,
		At:      now,
	})
	if err != nil {
		return err
	}
	if contained {
		return fmt.Errorf("expansion approval blocked by containment rule %s", rule.RuleID)
	}
	return nil
}

func authorityForAction(action Action) AuthorityRegion {
	operation := strings.TrimSpace(action.Operation)
	if operation == "" {
		operation = strings.TrimSpace(action.Name)
	}
	if action.Resource.Type == "" || action.Resource.ID == "" || operation == "" {
		return AuthorityRegion{}
	}
	return AuthorityRegion{Resources: []ResourceGrant{{
		Type:    action.Resource.Type,
		ID:      action.Resource.ID,
		Actions: []string{operation},
	}}}
}

func effectiveMissionVersionSeen(seen int, current int) int {
	if seen > 0 {
		return seen
	}
	return current
}

func mergeAuthority(region AuthorityRegion, requested AuthorityRegion) AuthorityRegion {
	merged := AuthorityRegion{
		Resources:        append([]ResourceGrant(nil), region.Resources...),
		ForbiddenActions: append([]string(nil), region.ForbiddenActions...),
	}
	for _, grant := range requested.Resources {
		if grant.Type == "" || grant.ID == "" || len(grant.Actions) == 0 {
			continue
		}
		found := false
		for i := range merged.Resources {
			if merged.Resources[i].Type == grant.Type && merged.Resources[i].ID == grant.ID {
				for _, action := range grant.Actions {
					if !contains(merged.Resources[i].Actions, action) {
						merged.Resources[i].Actions = append(merged.Resources[i].Actions, action)
					}
				}
				if merged.Resources[i].Constraints == nil && grant.Constraints != nil {
					merged.Resources[i].Constraints = grant.Constraints
				}
				found = true
				break
			}
		}
		if !found {
			merged.Resources = append(merged.Resources, grant)
		}
	}
	return merged
}

func authorityConflictsWithForbiddenActions(region AuthorityRegion, requested AuthorityRegion) bool {
	for _, grant := range requested.Resources {
		for _, action := range grant.Actions {
			if contains(region.ForbiddenActions, action) {
				return true
			}
		}
	}
	return false
}

func missingContextKeys(context map[string]any, required []string) []string {
	missing := make([]string, 0)
	for _, key := range required {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := lookupValue(context, key); !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func stringFromMap(values map[string]any, key string) (string, bool) {
	value, ok := lookupValue(values, key)
	if !ok {
		return "", false
	}
	str := strings.TrimSpace(fmt.Sprint(value))
	return str, str != ""
}

func toolArgumentDeny(req AuthorizeToolCallRequest, missing string) AuthorizeToolCallResponse {
	return AuthorizeToolCallResponse{
		ToolName:    req.ToolName,
		Decision:    DecisionDeny,
		MissionRef:  req.MissionRef,
		ReasonCodes: []string{"TOOL_CONTRACT_ARGUMENT_MISSING"},
		HumanReason: "The tool call is missing an argument required by its contract.",
		Constraints: map[string]any{"missing_argument": missing},
	}
}

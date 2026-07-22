package mission

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type Service struct {
	identities          IdentityStore
	missions            MissionStore
	governance          GovernanceStore
	projections         ProjectionStore
	approvals           ApprovalStore
	negotiations        NegotiationStore
	containments        ContainmentStore
	policies            PolicyStore
	expansionDecisions  ExpansionDecisionStore
	proposalApprovals   ProposalApprovalStore
	events              EventStore
	github              GitHubStore
	okta                OktaStore
	entra               EntraStore
	slack               SlackStore
	servicenow          ServiceNowStore
	atlassian           AtlassianStore
	salesforce          SalesforceStore
	clock               Clock
	artifactKey         []byte
	githubWebhookSecret []byte
	authorityGuard      AuthorityGuard
	governanceReads     GovernanceReadStore
}

type ServiceDependencies struct {
	Identities          IdentityStore
	Missions            MissionStore
	Governance          GovernanceStore
	Projections         ProjectionStore
	Approvals           ApprovalStore
	Negotiations        NegotiationStore
	Containments        ContainmentStore
	Policies            PolicyStore
	ExpansionDecisions  ExpansionDecisionStore
	ProposalApprovals   ProposalApprovalStore
	Events              EventStore
	GitHub              GitHubStore
	Okta                OktaStore
	Entra               EntraStore
	Slack               SlackStore
	ServiceNow          ServiceNowStore
	Atlassian           AtlassianStore
	Salesforce          SalesforceStore
	GovernanceReads     GovernanceReadStore
	AuthorityGuard      AuthorityGuard
	ArtifactKey         []byte
	GitHubWebhookSecret []byte
}

type containmentGuardStores struct {
	MissionStore
	GovernanceReadStore
}

func NewService(store Store, clock Clock) *Service {
	return NewServiceWithArtifactKey(store, clock, nil)
}

func NewServiceWithArtifactKey(store Store, clock Clock, artifactKey []byte) *Service {
	return NewServiceWithDependencies(ServiceDependencies{
		Identities:         store,
		Missions:           store,
		Governance:         store,
		Projections:        store,
		Approvals:          store,
		Negotiations:       store,
		Containments:       store,
		Policies:           store,
		ExpansionDecisions: store,
		ProposalApprovals:  store,
		Events:             store,
		GitHub:             store,
		Okta:               store,
		Entra:              store,
		Slack:              store,
		Atlassian:          store,
		Salesforce:         store,
		GovernanceReads:    store,
		ArtifactKey:        artifactKey,
	}, clock)
}

func NewServiceWithDependencies(dependencies ServiceDependencies, clock Clock) *Service {
	if clock == nil {
		clock = SystemClock{}
	}
	if dependencies.AuthorityGuard == nil {
		dependencies.AuthorityGuard = NewContainmentGuard(containmentGuardStores{
			MissionStore:        dependencies.Missions,
			GovernanceReadStore: dependencies.GovernanceReads,
		})
	}
	if dependencies.ProposalApprovals == nil {
		if approvals, ok := dependencies.Missions.(ProposalApprovalStore); ok {
			dependencies.ProposalApprovals = approvals
		}
	}
	if dependencies.Policies == nil {
		if policies, ok := dependencies.Governance.(PolicyStore); ok {
			dependencies.Policies = policies
		}
	}
	if len(dependencies.ArtifactKey) == 0 {
		dependencies.ArtifactKey = decisionArtifactKeyFromEnv()
	}
	if len(dependencies.GitHubWebhookSecret) == 0 {
		dependencies.GitHubWebhookSecret = GitHubWebhookSecretFromEnv()
	}
	return &Service{
		identities:          dependencies.Identities,
		missions:            dependencies.Missions,
		governance:          dependencies.Governance,
		projections:         dependencies.Projections,
		approvals:           dependencies.Approvals,
		negotiations:        dependencies.Negotiations,
		containments:        dependencies.Containments,
		policies:            dependencies.Policies,
		expansionDecisions:  dependencies.ExpansionDecisions,
		proposalApprovals:   dependencies.ProposalApprovals,
		events:              dependencies.Events,
		github:              dependencies.GitHub,
		okta:                dependencies.Okta,
		entra:               dependencies.Entra,
		slack:               dependencies.Slack,
		servicenow:          dependencies.ServiceNow,
		atlassian:           dependencies.Atlassian,
		salesforce:          dependencies.Salesforce,
		clock:               clock,
		artifactKey:         dependencies.ArtifactKey,
		githubWebhookSecret: dependencies.GitHubWebhookSecret,
		authorityGuard:      dependencies.AuthorityGuard,
		governanceReads:     dependencies.GovernanceReads,
	}
}

func (s *Service) CreateProposal(req CreateProposalRequest) (CreateProposalResponse, error) {
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	if req.Principal.Subject == "" {
		return CreateProposalResponse{}, fmt.Errorf("principal.subject is required")
	}
	if req.Agent.InstanceID == "" || req.Agent.ClientID == "" {
		return CreateProposalResponse{}, fmt.Errorf("agent.client_id and agent.instance_id are required")
	}
	if req.Intent.Objective == "" {
		return CreateProposalResponse{}, fmt.Errorf("intent.objective is required")
	}
	if len(req.AuthorityRegion.Resources) == 0 {
		return CreateProposalResponse{}, fmt.Errorf("authority_region.resources is required")
	}

	now := s.clock.Now()
	if req.Lifecycle.CreatedAt.IsZero() {
		req.Lifecycle.CreatedAt = now
	}
	if req.Lifecycle.NotBefore.IsZero() {
		req.Lifecycle.NotBefore = now
	}
	if req.Delegation.MaxDepth == 0 {
		req.Delegation.MaxDepth = 1
	}
	if req.Delegation.Attenuation == "" {
		req.Delegation.Attenuation = "strict_subset"
	}
	if req.Risk.DefaultMode == "" {
		req.Risk.DefaultMode = "signal_based"
	}

	proposal := MissionProposal{
		ProposalID:      newID("mprp"),
		Status:          StatePendingApproval,
		TenantID:        req.TenantID,
		Principal:       req.Principal,
		Agent:           req.Agent,
		Intent:          req.Intent,
		AuthorityRegion: req.AuthorityRegion,
		Conditions:      req.Conditions,
		Lifecycle:       req.Lifecycle,
		Delegation:      req.Delegation,
		Risk:            req.Risk,
		CreatedAt:       now,
	}
	if err := s.missions.SaveProposal(proposal); err != nil {
		return CreateProposalResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
		TenantID:   proposal.TenantID,
		Type:       "mission_proposal.created",
		Payload:    map[string]any{"proposal_id": proposal.ProposalID},
		OccurredAt: now,
	})

	return CreateProposalResponse{
		ProposalID:         proposal.ProposalID,
		Status:             proposal.Status,
		ProposedMissionRef: "preview_" + proposal.ProposalID,
		ApprovalURL:        "/v1/mission-proposals/" + proposal.ProposalID + "/approve",
		DisplaySummary:     proposal.Intent.Objective,
	}, nil
}

func (s *Service) ApproveProposal(id string, req ApproveProposalRequest) (ApproveProposalResponse, error) {
	proposal, err := s.missions.GetProposal(id)
	if err != nil {
		return ApproveProposalResponse{}, err
	}
	now := s.clock.Now()
	evidence := req.ApprovalEvidence
	evidence.Approver = req.Approver
	if evidence.ApprovedAt.IsZero() {
		evidence.ApprovedAt = now
	}

	mission := Mission{
		MissionID:       newID("mis"),
		MissionRef:      newID("mref"),
		TenantID:        proposal.TenantID,
		State:           StateActive,
		Version:         1,
		Principal:       proposal.Principal,
		Agent:           proposal.Agent,
		Purpose:         proposal.Intent,
		AuthorityRegion: proposal.AuthorityRegion,
		Conditions:      proposal.Conditions,
		Lifecycle:       proposal.Lifecycle,
		Delegation:      proposal.Delegation,
		Risk:            proposal.Risk,
		Approval:        evidence,
	}
	if mission.Lifecycle.CreatedAt.IsZero() {
		mission.Lifecycle.CreatedAt = now
	}
	if mission.Lifecycle.NotBefore.IsZero() {
		mission.Lifecycle.NotBefore = now
	}
	event := Event{
		EventID:      newID("mev"),
		MissionRef:   mission.MissionRef,
		TenantID:     mission.TenantID,
		Type:         "mission.activated",
		Actor:        map[string]any{"subject": req.Approver.Subject, "issuer": req.Approver.Issuer},
		Payload:      map[string]any{"proposal_id": id},
		VersionAfter: mission.Version,
		OccurredAt:   now,
	}
	if s.proposalApprovals != nil {
		if err := s.proposalApprovals.CommitProposalApproval(context.Background(), ProposalApprovalCommit{
			ProposalID: id,
			Mission:    mission,
			Event:      event,
		}); err != nil {
			return ApproveProposalResponse{}, err
		}
	} else {
		if err := s.missions.SaveMission(mission); err != nil {
			return ApproveProposalResponse{}, err
		}
		if err := s.missions.DeleteProposal(id); err != nil {
			return ApproveProposalResponse{}, err
		}
		if err := s.events.AppendEvent(event); err != nil {
			return ApproveProposalResponse{}, err
		}
	}

	return ApproveProposalResponse{
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		State:          mission.State,
	}, nil
}

func (s *Service) Evaluate(ref string, req EvaluateRequest) (EvaluateResponse, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return EvaluateResponse{}, err
	}
	now := s.clock.Now()
	if expired(mission, now) {
		updated, err := s.transition(mission, StateExpired, "mission_expired", nil, "")
		if err != nil {
			return EvaluateResponse{}, err
		}
		return s.finalizeEvaluation(updated, req, deny(updated, "MISSION_EXPIRED", "The mission has expired."), now)
	}
	if mission.State != StateActive {
		return s.finalizeEvaluation(mission, req, deny(mission, "MISSION_NOT_ACTIVE", "The mission is not active."), now)
	}
	if !actorMatches(mission, req.Actor) {
		return s.finalizeEvaluation(mission, req, deny(mission, "ACTOR_NOT_AUTHORIZED_FOR_MISSION", "The actor is not authorized for this mission."), now)
	}
	if rule, ok, err := s.matchingActiveContainmentForEvaluation(mission, req.Actor, req.Action, now); err != nil {
		return EvaluateResponse{}, err
	} else if ok {
		return s.finalizeEvaluation(mission, req, EvaluateResponse{
			Decision:       DecisionDeny,
			MissionRef:     mission.MissionRef,
			MissionVersion: mission.Version,
			ReasonCodes:    []string{"CONTAINMENT_ACTIVE"},
			HumanReason:    "The requested action is blocked by an active containment rule.",
			Constraints:    map[string]any{"containment_rule_id": rule.RuleID},
		}, now)
	}
	if req.MissionVersionSeen > 0 && req.MissionVersionSeen < mission.Version {
		return s.finalizeEvaluation(mission, req, EvaluateResponse{
			Decision:       DecisionRequireRefresh,
			MissionRef:     mission.MissionRef,
			MissionVersion: mission.Version,
			ReasonCodes:    []string{"MISSION_VERSION_STALE"},
			HumanReason:    "The caller has stale mission state.",
		}, now)
	}
	if !actionInScopeForContext(mission.AuthorityRegion, req.Action, req.Context) {
		if irreversible(req.Context) || highRisk(req.Context) {
			expansion, err := s.createExpansionRequestForMission(mission, CreateExpansionRequest{
				MissionVersionSeen: effectiveMissionVersionSeen(req.MissionVersionSeen, mission.Version),
				Requester:          req.Actor,
				Action:             req.Action,
				Context:            req.Context,
				RequestedAuthority: authorityForAction(req.Action),
				Justification:      "Risky action outside approved mission scope requires human approval.",
			}, now)
			if err != nil {
				return EvaluateResponse{}, err
			}
			return s.finalizeEvaluationWithEvidence(mission, req, EvaluateResponse{
				Decision:       DecisionRequireApproval,
				MissionRef:     mission.MissionRef,
				MissionVersion: mission.Version,
				ReasonCodes:    []string{"ACTION_OUTSIDE_MISSION_SCOPE", "APPROVAL_REQUIRED"},
				HumanReason:    "The requested action is outside the approved mission scope.",
				Constraints:    map[string]any{"expansion_request_id": expansion.ExpansionID},
				Escalation:     &Escalation{Type: "mission_expansion", URL: "/v1/expansion-requests/" + expansion.ExpansionID},
			}, now, nil, expansion.ExpansionID)
		}
		return s.finalizeEvaluation(mission, req, EvaluateResponse{
			Decision:       DecisionRequireExpansion,
			MissionRef:     mission.MissionRef,
			MissionVersion: mission.Version,
			ReasonCodes:    []string{"ACTION_OUTSIDE_MISSION_SCOPE"},
			HumanReason:    "The requested action needs mission expansion.",
		}, now)
	}
	conditionsOK, failedCondition, conditionResults, err := evaluateConditionsWithEvidence(mission.Conditions, req.Context)
	if err != nil {
		return EvaluateResponse{}, err
	}
	if !conditionsOK {
		if shouldSuspend(mission.Conditions, failedCondition) {
			updated, err := s.transition(mission, StateSuspended, "condition_failed", map[string]any{"condition_id": failedCondition}, "")
			if err != nil {
				return EvaluateResponse{}, err
			}
			return s.finalizeEvaluationWithEvidence(updated, req, EvaluateResponse{
				Decision:       DecisionSuspend,
				MissionRef:     updated.MissionRef,
				MissionVersion: updated.Version,
				ReasonCodes:    []string{"MISSION_CONDITION_FAILED", failedCondition},
				HumanReason:    "A mission condition failed and the mission has been suspended.",
			}, now, conditionResults, "")
		}
		return s.finalizeEvaluationWithEvidence(mission, req, deny(mission, "MISSION_CONDITION_FAILED", "A mission condition failed."), now, conditionResults, "")
	}

	return s.finalizeEvaluationWithEvidence(mission, req, EvaluateResponse{
		Decision:       DecisionAllow,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		ReasonCodes:    []string{"IN_SCOPE", "MISSION_ACTIVE", "CONDITIONS_TRUE"},
		HumanReason:    "The action is authorized by the active mission.",
	}, now, conditionResults, "")
}

func (s *Service) Resume(ref string, req ResumeRequest) (EvaluateResponse, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return EvaluateResponse{}, err
	}
	now := s.clock.Now()
	if expired(mission, now) {
		updated, _ := s.transition(mission, StateExpired, "mission_expired", nil, "")
		return deny(updated, "MISSION_EXPIRED", "The mission has expired."), nil
	}
	if mission.State != StateActive {
		return deny(mission, "MISSION_NOT_ACTIVE", "The mission is not active."), nil
	}
	if !actorMatches(mission, req.Actor) {
		return deny(mission, "ACTOR_NOT_AUTHORIZED_FOR_MISSION", "The actor is not authorized for this mission."), nil
	}
	if rule, ok, err := s.matchingActiveContainmentForEvaluation(mission, req.Actor, Action{Type: "mission", Name: "resume", Resource: ActionResource{Type: "mission", ID: mission.MissionRef}, Operation: "resume"}, now); err != nil {
		return EvaluateResponse{}, err
	} else if ok {
		return EvaluateResponse{
			Decision:       DecisionDeny,
			MissionRef:     mission.MissionRef,
			MissionVersion: mission.Version,
			ReasonCodes:    []string{"CONTAINMENT_ACTIVE"},
			HumanReason:    "Mission resume is blocked by an active containment rule.",
			Constraints:    map[string]any{"containment_rule_id": rule.RuleID},
		}, nil
	}
	if req.MissionVersionSeen > 0 && req.MissionVersionSeen < mission.Version {
		return EvaluateResponse{
			Decision:       DecisionRequireRefresh,
			MissionRef:     mission.MissionRef,
			MissionVersion: mission.Version,
			ReasonCodes:    []string{"MISSION_VERSION_STALE"},
		}, nil
	}
	return EvaluateResponse{
		Decision:       DecisionAllow,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		ReasonCodes:    []string{"MISSION_ACTIVE", "ACTOR_AUTHORIZED"},
		HumanReason:    "The agent may resume this mission.",
	}, nil
}

func (s *Service) Delegate(ref string, req DelegationRequest) (DelegationResponse, error) {
	parent, err := s.missions.GetMission(ref)
	if err != nil {
		return DelegationResponse{}, err
	}
	if parent.State != StateActive {
		return DelegationResponse{}, fmt.Errorf("parent mission is not active")
	}
	if !actorMatches(parent, req.DelegatingActor) {
		return DelegationResponse{}, fmt.Errorf("delegating actor is not authorized for parent mission")
	}
	if rule, ok, err := s.matchingActiveContainmentForEvaluation(parent, req.DelegatingActor, Action{Type: "mission", Name: "delegate", Resource: ActionResource{Type: "mission", ID: parent.MissionRef}, Operation: "delegate"}, s.clock.Now()); err != nil {
		return DelegationResponse{}, err
	} else if ok {
		return DelegationResponse{}, fmt.Errorf("delegation blocked by containment rule %s", rule.RuleID)
	}
	if !parent.Delegation.Permitted {
		return DelegationResponse{}, fmt.Errorf("delegation is not permitted")
	}
	if parent.Delegation.MaxDepth > 0 && parent.Delegation.CurrentDepth >= parent.Delegation.MaxDepth {
		return DelegationResponse{}, fmt.Errorf("delegation depth exceeded")
	}
	if !authoritySubset(parent.AuthorityRegion, req.RequestedAuthority) {
		return DelegationResponse{}, fmt.Errorf("requested authority is not a strict subset of parent authority")
	}
	expiresAt := req.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = parent.Lifecycle.ExpiresAt
	}
	if !parent.Lifecycle.ExpiresAt.IsZero() && expiresAt.After(parent.Lifecycle.ExpiresAt) {
		return DelegationResponse{}, fmt.Errorf("child mission expiry exceeds parent mission expiry")
	}
	now := s.clock.Now()
	childDelegation := req.Delegation
	childDelegation.CurrentDepth = parent.Delegation.CurrentDepth + 1
	childDelegation.ParentMissionRef = parent.MissionRef
	if childDelegation.Attenuation == "" {
		childDelegation.Attenuation = "strict_subset"
	}
	child := Mission{
		MissionID:       newID("mis"),
		MissionRef:      newID("mref"),
		TenantID:        parent.TenantID,
		State:           StateActive,
		Version:         1,
		Principal:       parent.Principal,
		Agent:           req.TargetAgent,
		Purpose:         parent.Purpose,
		AuthorityRegion: req.RequestedAuthority,
		Conditions:      append(parent.Conditions, req.Conditions...),
		Lifecycle: Lifecycle{
			CreatedAt:      now,
			NotBefore:      now,
			ExpiresAt:      expiresAt,
			TerminalEvents: parent.Lifecycle.TerminalEvents,
			OnExpiry:       parent.Lifecycle.OnExpiry,
		},
		Delegation: childDelegation,
		Risk:       parent.Risk,
		Approval:   parent.Approval,
	}
	if err := s.missions.SaveMission(child); err != nil {
		return DelegationResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:      newID("mev"),
		MissionRef:   child.MissionRef,
		TenantID:     child.TenantID,
		Type:         "mission.delegated",
		Actor:        map[string]any{"parent_mission_ref": parent.MissionRef, "agent_instance_id": req.DelegatingActor.AgentInstanceID},
		Payload:      map[string]any{"child_mission_ref": child.MissionRef},
		VersionAfter: child.Version,
		OccurredAt:   now,
	})
	return DelegationResponse{
		ChildMissionRef:  child.MissionRef,
		ParentMissionRef: parent.MissionRef,
		State:            child.State,
		Attenuation:      child.Delegation.Attenuation,
	}, nil
}

func (s *Service) Revoke(ref string, req StateChangeRequest) (Mission, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return Mission{}, err
	}
	return s.transitionCascade(mission, StateRevoked, "mission_revoked", req)
}

func (s *Service) Complete(ref string, req StateChangeRequest) (Mission, error) {
	mission, err := s.missions.GetMission(ref)
	if err != nil {
		return Mission{}, err
	}
	return s.transitionCascade(mission, StateCompleted, "mission_completed", req)
}

func (s *Service) Introspect(ref string) (Mission, error) {
	return s.missions.GetMission(ref)
}

func (s *Service) Events() []Event {
	return s.events.Events()
}

func (s *Service) finalizeEvaluation(mission Mission, req EvaluateRequest, resp EvaluateResponse, now time.Time) (EvaluateResponse, error) {
	return s.finalizeEvaluationWithEvidence(mission, req, resp, now, nil, "")
}

func (s *Service) finalizeEvaluationWithEvidence(mission Mission, req EvaluateRequest, resp EvaluateResponse, now time.Time, conditionResults []ConditionEvaluation, expansionID string) (EvaluateResponse, error) {
	resp, policyEvaluation, err := s.activePolicyEvaluation(mission, req, resp)
	if err != nil {
		return EvaluateResponse{}, err
	}
	if resp.MissionRef == "" {
		resp.MissionRef = mission.MissionRef
	}
	if resp.MissionVersion == 0 {
		resp.MissionVersion = mission.Version
	}
	evidenceID := newID("mevd")
	contextHash := HashDecisionContext(req.Context)
	policyVersion := policyEvaluation.PolicyVersion
	if policyVersion == "" {
		policyVersion = DefaultPolicyVersionID
	}
	payload := DecisionArtifactPayload{
		ArtifactID:         newID("mdar"),
		MissionRef:         resp.MissionRef,
		MissionVersion:     resp.MissionVersion,
		PolicyVersion:      policyVersion,
		PolicyBundleID:     policyEvaluation.BundleID,
		PolicyBundleHash:   policyEvaluation.BundleHash,
		PolicyRuleIDs:      appliedPolicyRuleIDs(policyEvaluation.RuleResults),
		EvidenceID:         evidenceID,
		ExpansionRequestID: expansionID,
		Decision:           resp.Decision,
		ReasonCodes:        resp.ReasonCodes,
		Actor:              req.Actor,
		Action:             req.Action,
		ContextHash:        contextHash,
		IssuedAt:           now,
	}
	artifact, err := SignDecisionArtifact(payload, s.artifactKey)
	if err != nil {
		return EvaluateResponse{}, err
	}
	resp.DecisionArtifact = artifact
	if err := s.governance.SaveEvaluationEvidence(EvaluationEvidence{
		EvidenceID:       evidenceID,
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		TenantID:         mission.TenantID,
		PolicyVersion:    policyVersion,
		PolicyBundleID:   policyEvaluation.BundleID,
		PolicyBundleHash: policyEvaluation.BundleHash,
		Actor:            req.Actor,
		Action:           req.Action,
		ContextHash:      contextHash,
		Decision:         resp.Decision,
		ReasonCodes:      resp.ReasonCodes,
		ConditionResults: conditionResults,
		PolicyResults:    policyEvaluation.RuleResults,
		Artifact:         artifact,
		CreatedAt:        now,
	}); err != nil {
		return EvaluateResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.evaluated",
		Actor:         map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID, "key_thumbprint": req.Actor.KeyThumbprint},
		Payload:       map[string]any{"decision": resp.Decision, "decision_artifact": artifact, "evidence_id": evidenceID, "policy_version": policyVersion, "policy_bundle_id": policyEvaluation.BundleID, "policy_bundle_hash": policyEvaluation.BundleHash, "policy_rule_ids": appliedPolicyRuleIDs(policyEvaluation.RuleResults), "expansion_request_id": expansionID, "operation": req.Action.Operation, "resource_type": req.Action.Resource.Type, "resource_id": req.Action.Resource.ID, "reason_codes": resp.ReasonCodes},
		VersionBefore: mission.Version,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	})
	return resp, nil
}

func appliedPolicyRuleIDs(results []PolicyRuleResult) []string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		if result.Applied {
			ids = append(ids, result.RuleID)
		}
	}
	return ids
}

func (s *Service) transitionCascade(mission Mission, state State, reason string, req StateChangeRequest) (Mission, error) {
	updated, err := s.transition(mission, state, reason, map[string]any{"reason": req.Reason}, req.CausationID)
	if err != nil {
		return Mission{}, err
	}
	if mission.Delegation.CascadeRevocation || state == StateCompleted || state == StateRevoked {
		children, err := s.missions.ChildrenOf(mission.MissionRef)
		if err != nil {
			return Mission{}, err
		}
		childState := StateRevoked
		if state == StateCompleted {
			childState = StateRevoked
		}
		for _, child := range children {
			if child.State == StateActive || child.State == StateSuspended {
				_, _ = s.transitionCascade(child, childState, "parent_"+reason, StateChangeRequest{
					Actor:       map[string]any{"type": "system", "subject": "mission-authority-service"},
					Reason:      "parent mission transitioned to " + string(state),
					CausationID: req.CausationID,
				})
			}
		}
	}
	return updated, nil
}

func (s *Service) transition(mission Mission, state State, reason string, payload map[string]any, causationID string) (Mission, error) {
	if terminal(mission.State) {
		return mission, nil
	}
	now := s.clock.Now()
	before := mission.Version
	mission.State = state
	mission.Version++
	if err := s.missions.UpdateMission(mission); err != nil {
		return Mission{}, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["reason"] = reason
	_ = s.events.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission." + string(state),
		Payload:       payload,
		VersionBefore: before,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
		CausationID:   causationID,
	})
	return mission, nil
}

func actorMatches(mission Mission, actor Actor) bool {
	if actor.AgentInstanceID == "" || actor.ClientID == "" {
		return false
	}
	if mission.Agent.InstanceID != actor.AgentInstanceID || mission.Agent.ClientID != actor.ClientID {
		return false
	}
	if mission.Agent.KeyThumbprint != "" {
		return mission.Agent.KeyThumbprint == actor.KeyThumbprint
	}
	return true
}

func expired(mission Mission, now time.Time) bool {
	if mission.Lifecycle.NotBefore.After(now) {
		return true
	}
	return !mission.Lifecycle.ExpiresAt.IsZero() && now.After(mission.Lifecycle.ExpiresAt)
}

func terminal(state State) bool {
	return state == StateCompleted || state == StateRevoked || state == StateExpired || state == StateRejected
}

func shouldSuspend(conditions []Condition, id string) bool {
	for _, condition := range conditions {
		if condition.ID == id {
			return condition.OnFailure == "suspend" || condition.OnFailure == "suspend_and_notify" || condition.OnFailure == "halt_and_escalate"
		}
	}
	return false
}

func irreversible(context map[string]any) bool {
	value, ok := context["reversible"]
	return ok && fmt.Sprint(value) == "false"
}

func highRisk(context map[string]any) bool {
	value, ok := context["risk"]
	return ok && (fmt.Sprint(value) == "high" || fmt.Sprint(value) == "critical")
}

func deny(mission Mission, code string, reason string) EvaluateResponse {
	return EvaluateResponse{
		Decision:       DecisionDeny,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		ReasonCodes:    []string{code},
		HumanReason:    reason,
	}
}

func newID(prefix string) string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(errors.Join(fmt.Errorf("generate id"), err))
	}
	return prefix + "_" + hex.EncodeToString(buf[:])
}

// OutboxPublisher handles publishing events from the outbox table.
type OutboxPublisher struct {
	store    OutboxStore
	interval time.Duration
	logger   *slog.Logger
}

// NewOutboxPublisher creates a new outbox publisher.
func NewOutboxPublisher(store OutboxStore, interval time.Duration) *OutboxPublisher {
	return &OutboxPublisher{
		store:    store,
		interval: interval,
		logger:   slog.Default().With("component", "outbox-publisher"),
	}
}

// Start begins the polling loop to publish outbox events.
func (p *OutboxPublisher) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.logger.Info("outbox publisher started", "interval_ms", p.interval.Milliseconds())

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox publisher stopped")
			return ctx.Err()
		case <-ticker.C:
			events, err := p.store.PublishOutboxEvents()
			if err != nil {
				p.logger.Error("failed to publish outbox events", "error", err)
				continue
			}

			for _, event := range events {
				p.logger.Info("published outbox event",
					"id", event.ID,
					"type", event.Type,
					"mission_ref", event.MissionRef,
				)
			}
		}
	}
}

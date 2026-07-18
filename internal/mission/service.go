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
	store Store
	clock Clock
}

func NewService(store Store, clock Clock) *Service {
	return &Service{store: store, clock: clock}
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
	if err := s.store.SaveProposal(proposal); err != nil {
		return CreateProposalResponse{}, err
	}
	_ = s.store.AppendEvent(Event{
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
	proposal, err := s.store.GetProposal(id)
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
	if err := s.store.SaveMission(mission); err != nil {
		return ApproveProposalResponse{}, err
	}
	_ = s.store.DeleteProposal(id)
	_ = s.store.AppendEvent(Event{
		EventID:      newID("mev"),
		MissionRef:   mission.MissionRef,
		TenantID:     mission.TenantID,
		Type:         "mission.activated",
		Actor:        map[string]any{"subject": req.Approver.Subject, "issuer": req.Approver.Issuer},
		Payload:      map[string]any{"proposal_id": id},
		VersionAfter: mission.Version,
		OccurredAt:   now,
	})

	return ApproveProposalResponse{
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		State:          mission.State,
	}, nil
}

func (s *Service) Evaluate(ref string, req EvaluateRequest) (EvaluateResponse, error) {
	mission, err := s.store.GetMission(ref)
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
	if req.MissionVersionSeen > 0 && req.MissionVersionSeen < mission.Version {
		return EvaluateResponse{
			Decision:       DecisionRequireRefresh,
			MissionRef:     mission.MissionRef,
			MissionVersion: mission.Version,
			ReasonCodes:    []string{"MISSION_VERSION_STALE"},
			HumanReason:    "The caller has stale mission state.",
		}, nil
	}
	if !actionInScope(mission.AuthorityRegion, req.Action) {
		if irreversible(req.Context) || highRisk(req.Context) {
			return EvaluateResponse{
				Decision:       DecisionRequireApproval,
				MissionRef:     mission.MissionRef,
				MissionVersion: mission.Version,
				ReasonCodes:    []string{"ACTION_OUTSIDE_MISSION_SCOPE", "APPROVAL_REQUIRED"},
				HumanReason:    "The requested action is outside the approved mission scope.",
				Escalation:     &Escalation{Type: "mission_expansion", URL: "/v1/missions/" + mission.MissionRef + "/expansion-requests"},
			}, nil
		}
		return EvaluateResponse{
			Decision:       DecisionRequireExpansion,
			MissionRef:     mission.MissionRef,
			MissionVersion: mission.Version,
			ReasonCodes:    []string{"ACTION_OUTSIDE_MISSION_SCOPE"},
			HumanReason:    "The requested action needs mission expansion.",
		}, nil
	}
	conditionsOK, failedCondition, err := evaluateConditions(mission.Conditions, req.Context)
	if err != nil {
		return EvaluateResponse{}, err
	}
	if !conditionsOK {
		if shouldSuspend(mission.Conditions, failedCondition) {
			updated, _ := s.transition(mission, StateSuspended, "condition_failed", map[string]any{"condition_id": failedCondition}, "")
			return EvaluateResponse{
				Decision:       DecisionSuspend,
				MissionRef:     updated.MissionRef,
				MissionVersion: updated.Version,
				ReasonCodes:    []string{"MISSION_CONDITION_FAILED", failedCondition},
				HumanReason:    "A mission condition failed and the mission has been suspended.",
			}, nil
		}
		return deny(mission, "MISSION_CONDITION_FAILED", "A mission condition failed."), nil
	}

	artifact := newID("mdar")
	_ = s.store.AppendEvent(Event{
		EventID:       newID("mev"),
		MissionRef:    mission.MissionRef,
		TenantID:      mission.TenantID,
		Type:          "mission.evaluated",
		Actor:         map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID},
		Payload:       map[string]any{"decision": DecisionAllow, "decision_artifact": artifact, "operation": req.Action.Operation},
		VersionBefore: mission.Version,
		VersionAfter:  mission.Version,
		OccurredAt:    now,
	})
	return EvaluateResponse{
		Decision:         DecisionAllow,
		MissionRef:       mission.MissionRef,
		MissionVersion:   mission.Version,
		ReasonCodes:      []string{"IN_SCOPE", "MISSION_ACTIVE", "CONDITIONS_TRUE"},
		HumanReason:      "The action is authorized by the active mission.",
		DecisionArtifact: artifact,
	}, nil
}

func (s *Service) Resume(ref string, req ResumeRequest) (EvaluateResponse, error) {
	mission, err := s.store.GetMission(ref)
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
	parent, err := s.store.GetMission(ref)
	if err != nil {
		return DelegationResponse{}, err
	}
	if parent.State != StateActive {
		return DelegationResponse{}, fmt.Errorf("parent mission is not active")
	}
	if !actorMatches(parent, req.DelegatingActor) {
		return DelegationResponse{}, fmt.Errorf("delegating actor is not authorized for parent mission")
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
	if err := s.store.SaveMission(child); err != nil {
		return DelegationResponse{}, err
	}
	_ = s.store.AppendEvent(Event{
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
	mission, err := s.store.GetMission(ref)
	if err != nil {
		return Mission{}, err
	}
	return s.transitionCascade(mission, StateRevoked, "mission_revoked", req)
}

func (s *Service) Complete(ref string, req StateChangeRequest) (Mission, error) {
	mission, err := s.store.GetMission(ref)
	if err != nil {
		return Mission{}, err
	}
	return s.transitionCascade(mission, StateCompleted, "mission_completed", req)
}

func (s *Service) Introspect(ref string) (Mission, error) {
	return s.store.GetMission(ref)
}

func (s *Service) Events() []Event {
	return s.store.Events()
}

func (s *Service) transitionCascade(mission Mission, state State, reason string, req StateChangeRequest) (Mission, error) {
	updated, err := s.transition(mission, state, reason, map[string]any{"reason": req.Reason}, req.CausationID)
	if err != nil {
		return Mission{}, err
	}
	if mission.Delegation.CascadeRevocation || state == StateCompleted || state == StateRevoked {
		children, err := s.store.ChildrenOf(mission.MissionRef)
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
	if err := s.store.UpdateMission(mission); err != nil {
		return Mission{}, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["reason"] = reason
	_ = s.store.AppendEvent(Event{
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
	if mission.Agent.KeyThumbprint != "" && actor.KeyThumbprint != "" {
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
	store   Store
	interval time.Duration
	logger  *slog.Logger
}

// NewOutboxPublisher creates a new outbox publisher.
func NewOutboxPublisher(store Store, interval time.Duration) *OutboxPublisher {
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

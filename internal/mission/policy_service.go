package mission

import (
	"errors"
	"fmt"
)

func (s *Service) CreatePolicyBundle(req CreatePolicyBundleRequest, createdBy Principal) (PolicyBundle, error) {
	if s.policies == nil {
		return PolicyBundle{}, fmt.Errorf("policy store is not configured")
	}
	now := s.clock.Now()
	bundle, err := normalizePolicyBundle(req, createdBy, now)
	if err != nil {
		return PolicyBundle{}, err
	}
	if err := s.policies.SavePolicyBundle(bundle); err != nil {
		return PolicyBundle{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
		TenantID:   bundle.TenantID,
		Type:       "policy_bundle.created",
		Actor:      map[string]any{"subject": createdBy.Subject, "issuer": createdBy.Issuer},
		Payload:    map[string]any{"bundle_id": bundle.BundleID, "policy_version": bundle.Version, "bundle_hash": bundle.BundleHash},
		OccurredAt: now,
	})
	return bundle, nil
}

func (s *Service) GetPolicyBundle(id string) (PolicyBundle, error) {
	if s.policies == nil {
		return PolicyBundle{}, fmt.Errorf("policy store is not configured")
	}
	return s.policies.GetPolicyBundle(id)
}

func (s *Service) ListPolicyBundles() ([]PolicyBundle, error) {
	if s.policies == nil {
		return nil, fmt.Errorf("policy store is not configured")
	}
	return s.policies.ListPolicyBundles()
}

func (s *Service) ActivatePolicyBundle(id string, req ActivatePolicyBundleRequest, activatedBy Principal) (PolicyBundle, error) {
	if s.policies == nil {
		return PolicyBundle{}, fmt.Errorf("policy store is not configured")
	}
	bundle, err := s.policies.GetPolicyBundle(id)
	if err != nil {
		return PolicyBundle{}, err
	}
	now := s.clock.Now()
	bundle.Status = PolicyBundleStatusActive
	bundle.ActivatedBy = activatedBy
	bundle.ActivatedAt = now
	if bundle.BundleHash == "" {
		hash, err := hashPolicyBundle(bundle)
		if err != nil {
			return PolicyBundle{}, err
		}
		bundle.BundleHash = hash
	}
	bundle.Signature = signPolicyBundleHash(bundle.BundleHash, s.artifactKey)
	if err := s.policies.ActivatePolicyBundle(bundle); err != nil {
		return PolicyBundle{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:  newID("mev"),
		TenantID: bundle.TenantID,
		Type:     "policy_bundle.activated",
		Actor:    map[string]any{"subject": activatedBy.Subject, "issuer": activatedBy.Issuer},
		Payload: map[string]any{
			"bundle_id":      bundle.BundleID,
			"policy_version": bundle.Version,
			"bundle_hash":    bundle.BundleHash,
			"signature":      bundle.Signature,
			"reason":         req.Reason,
		},
		OccurredAt: now,
	})
	return bundle, nil
}

func (s *Service) SimulatePolicyBundle(id string, req SimulatePolicyBundleRequest) (SimulatePolicyBundleResponse, error) {
	if s.policies == nil {
		return SimulatePolicyBundleResponse{}, fmt.Errorf("policy store is not configured")
	}
	if req.MissionRef == "" {
		return SimulatePolicyBundleResponse{}, fmt.Errorf("mission_ref is required")
	}
	bundle, err := s.policies.GetPolicyBundle(id)
	if err != nil {
		return SimulatePolicyBundleResponse{}, err
	}
	mission, err := s.missions.GetMission(req.MissionRef)
	if err != nil {
		return SimulatePolicyBundleResponse{}, err
	}
	if bundle.TenantID != "" && bundle.TenantID != mission.TenantID {
		return SimulatePolicyBundleResponse{}, fmt.Errorf("policy bundle tenant %q does not match mission tenant %q", bundle.TenantID, mission.TenantID)
	}
	baseDecision := req.BaseDecision
	if baseDecision == "" {
		baseDecision = DecisionAllow
	}
	base := EvaluateResponse{
		Decision:       baseDecision,
		MissionRef:     mission.MissionRef,
		MissionVersion: mission.Version,
		ReasonCodes:    []string{"SIMULATED_BASE_DECISION"},
		HumanReason:    "Policy simulation baseline.",
	}
	evaluated, policyEvaluation, err := applyPolicyBundle(bundle, mission, req.Evaluation, base)
	if err != nil {
		return SimulatePolicyBundleResponse{}, err
	}
	return SimulatePolicyBundleResponse{
		BundleID:         bundle.BundleID,
		TenantID:         bundle.TenantID,
		PolicyVersion:    bundle.Version,
		BundleHash:       bundle.BundleHash,
		OriginalDecision: base.Decision,
		Decision:         evaluated.Decision,
		ReasonCodes:      evaluated.ReasonCodes,
		HumanReason:      evaluated.HumanReason,
		Constraints:      evaluated.Constraints,
		PolicyEvaluation: policyEvaluation,
		RuleResults:      policyEvaluation.RuleResults,
	}, nil
}

func (s *Service) activePolicyEvaluation(mission Mission, req EvaluateRequest, resp EvaluateResponse) (EvaluateResponse, PolicyEvaluation, error) {
	evaluation := PolicyEvaluation{PolicyVersion: DefaultPolicyVersionID}
	if s.policies == nil {
		return resp, evaluation, nil
	}
	bundle, err := s.policies.GetActivePolicyBundle(mission.TenantID)
	if errors.Is(err, ErrNotFound) {
		return resp, evaluation, nil
	}
	if err != nil {
		return EvaluateResponse{}, evaluation, err
	}
	return applyPolicyBundle(bundle, mission, req, resp)
}

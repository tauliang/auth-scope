package mission

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

type PolicyBundleStatus string

const (
	PolicyBundleStatusDraft    PolicyBundleStatus = "draft"
	PolicyBundleStatusActive   PolicyBundleStatus = "active"
	PolicyBundleStatusArchived PolicyBundleStatus = "archived"

	PolicyCombiningFirstApplicable = "first_applicable"

	PolicyEffectAllow                = "allow"
	PolicyEffectDeny                 = "deny"
	PolicyEffectRequireApproval      = "require_approval"
	PolicyEffectRequireExpansion     = "require_expansion"
	PolicyEffectAllowWithConstraints = "allow_with_constraints"
)

type PolicyBundle struct {
	BundleID           string             `json:"bundle_id"`
	TenantID           string             `json:"tenant_id,omitempty"`
	Version            string             `json:"version"`
	Name               string             `json:"name,omitempty"`
	Description        string             `json:"description,omitempty"`
	Status             PolicyBundleStatus `json:"status"`
	CombiningAlgorithm string             `json:"combining_algorithm,omitempty"`
	Rules              []PolicyRule       `json:"rules"`
	BundleHash         string             `json:"bundle_hash,omitempty"`
	Signature          string             `json:"signature,omitempty"`
	CreatedBy          Principal          `json:"created_by,omitempty"`
	ActivatedBy        Principal          `json:"activated_by,omitempty"`
	CreatedAt          time.Time          `json:"created_at,omitempty"`
	ActivatedAt        time.Time          `json:"activated_at,omitempty"`
}

type PolicyRule struct {
	RuleID      string          `json:"rule_id"`
	Description string          `json:"description,omitempty"`
	Priority    int             `json:"priority,omitempty"`
	Disabled    bool            `json:"disabled,omitempty"`
	Effect      string          `json:"effect"`
	Match       PolicyRuleMatch `json:"match,omitempty"`
	Conditions  []Condition     `json:"conditions,omitempty"`
	ReasonCodes []string        `json:"reason_codes,omitempty"`
	HumanReason string          `json:"human_reason,omitempty"`
	Constraints map[string]any  `json:"constraints,omitempty"`
}

type PolicyRuleMatch struct {
	ActionTypes       []string   `json:"action_types,omitempty"`
	Operations        []string   `json:"operations,omitempty"`
	ResourceTypes     []string   `json:"resource_types,omitempty"`
	ResourceIDs       []string   `json:"resource_ids,omitempty"`
	AgentClientIDs    []string   `json:"agent_client_ids,omitempty"`
	PrincipalSubjects []string   `json:"principal_subjects,omitempty"`
	BaseDecisions     []Decision `json:"base_decisions,omitempty"`
}

type PolicyRuleResult struct {
	RuleID           string                `json:"rule_id"`
	Effect           string                `json:"effect,omitempty"`
	Matched          bool                  `json:"matched"`
	Applied          bool                  `json:"applied,omitempty"`
	ReasonCodes      []string              `json:"reason_codes,omitempty"`
	ConditionResults []ConditionEvaluation `json:"condition_results,omitempty"`
	Error            string                `json:"error,omitempty"`
}

type PolicyEvaluation struct {
	BundleID      string             `json:"bundle_id,omitempty"`
	TenantID      string             `json:"tenant_id,omitempty"`
	PolicyVersion string             `json:"policy_version"`
	BundleHash    string             `json:"bundle_hash,omitempty"`
	Status        PolicyBundleStatus `json:"status,omitempty"`
	RuleResults   []PolicyRuleResult `json:"rule_results,omitempty"`
}

type CreatePolicyBundleRequest struct {
	TenantID           string       `json:"tenant_id,omitempty"`
	Version            string       `json:"version"`
	Name               string       `json:"name,omitempty"`
	Description        string       `json:"description,omitempty"`
	CombiningAlgorithm string       `json:"combining_algorithm,omitempty"`
	Rules              []PolicyRule `json:"rules"`
}

type ActivatePolicyBundleRequest struct {
	Reason string `json:"reason,omitempty"`
}

type SimulatePolicyBundleRequest struct {
	MissionRef   string          `json:"mission_ref"`
	Evaluation   EvaluateRequest `json:"evaluation"`
	BaseDecision Decision        `json:"base_decision,omitempty"`
}

type SimulatePolicyBundleResponse struct {
	BundleID         string             `json:"bundle_id"`
	TenantID         string             `json:"tenant_id,omitempty"`
	PolicyVersion    string             `json:"policy_version"`
	BundleHash       string             `json:"bundle_hash,omitempty"`
	OriginalDecision Decision           `json:"original_decision"`
	Decision         Decision           `json:"decision"`
	ReasonCodes      []string           `json:"reason_codes,omitempty"`
	HumanReason      string             `json:"human_reason,omitempty"`
	Constraints      map[string]any     `json:"constraints,omitempty"`
	PolicyEvaluation PolicyEvaluation   `json:"policy_evaluation"`
	RuleResults      []PolicyRuleResult `json:"rule_results,omitempty"`
}

type policyBundleDigest struct {
	TenantID           string       `json:"tenant_id,omitempty"`
	Version            string       `json:"version"`
	Name               string       `json:"name,omitempty"`
	Description        string       `json:"description,omitempty"`
	CombiningAlgorithm string       `json:"combining_algorithm,omitempty"`
	Rules              []PolicyRule `json:"rules"`
}

func hashPolicyBundle(bundle PolicyBundle) (string, error) {
	payload := policyBundleDigest{
		TenantID:           strings.TrimSpace(bundle.TenantID),
		Version:            strings.TrimSpace(bundle.Version),
		Name:               strings.TrimSpace(bundle.Name),
		Description:        strings.TrimSpace(bundle.Description),
		CombiningAlgorithm: strings.TrimSpace(bundle.CombiningAlgorithm),
		Rules:              bundle.Rules,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal policy bundle digest: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func signPolicyBundleHash(hash string, key []byte) string {
	if len(key) == 0 || strings.TrimSpace(hash) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("auth-scope-policy-bundle\n" + hash))
	return "hs256:" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func normalizePolicyBundle(req CreatePolicyBundleRequest, createdBy Principal, now time.Time) (PolicyBundle, error) {
	version := strings.TrimSpace(req.Version)
	if version == "" {
		return PolicyBundle{}, fmt.Errorf("version is required")
	}
	if len(req.Rules) == 0 {
		return PolicyBundle{}, fmt.Errorf("rules are required")
	}
	algorithm := strings.TrimSpace(req.CombiningAlgorithm)
	if algorithm == "" {
		algorithm = PolicyCombiningFirstApplicable
	}
	if algorithm != PolicyCombiningFirstApplicable {
		return PolicyBundle{}, fmt.Errorf("unsupported policy combining algorithm %q", algorithm)
	}

	rules := append([]PolicyRule(nil), req.Rules...)
	for i := range rules {
		rules[i].RuleID = strings.TrimSpace(rules[i].RuleID)
		if rules[i].RuleID == "" {
			rules[i].RuleID = fmt.Sprintf("rule-%03d", i+1)
		}
		rules[i].Effect = strings.TrimSpace(rules[i].Effect)
		if rules[i].Effect == "" {
			return PolicyBundle{}, fmt.Errorf("rules[%d].effect is required", i)
		}
		if !supportedPolicyEffect(rules[i].Effect) {
			return PolicyBundle{}, fmt.Errorf("rules[%d].effect %q is unsupported", i, rules[i].Effect)
		}
	}
	slices.SortFunc(rules, func(a, b PolicyRule) int {
		if a.Priority == b.Priority {
			return strings.Compare(a.RuleID, b.RuleID)
		}
		if a.Priority < b.Priority {
			return -1
		}
		return 1
	})

	bundle := PolicyBundle{
		BundleID:           newID("mpol"),
		TenantID:           strings.TrimSpace(req.TenantID),
		Version:            version,
		Name:               strings.TrimSpace(req.Name),
		Description:        strings.TrimSpace(req.Description),
		Status:             PolicyBundleStatusDraft,
		CombiningAlgorithm: algorithm,
		Rules:              rules,
		CreatedBy:          createdBy,
		CreatedAt:          now,
	}
	hash, err := hashPolicyBundle(bundle)
	if err != nil {
		return PolicyBundle{}, err
	}
	bundle.BundleHash = hash
	return bundle, nil
}

func supportedPolicyEffect(effect string) bool {
	switch effect {
	case PolicyEffectAllow, PolicyEffectDeny, PolicyEffectRequireApproval, PolicyEffectRequireExpansion, PolicyEffectAllowWithConstraints:
		return true
	default:
		return false
	}
}

func applyPolicyBundle(bundle PolicyBundle, mission Mission, req EvaluateRequest, base EvaluateResponse) (EvaluateResponse, PolicyEvaluation, error) {
	evaluation := PolicyEvaluation{
		BundleID:      bundle.BundleID,
		TenantID:      bundle.TenantID,
		PolicyVersion: bundle.Version,
		BundleHash:    bundle.BundleHash,
		Status:        bundle.Status,
	}
	if !policyMayRestrictDecision(base.Decision) {
		return base, evaluation, nil
	}
	context := policyEvaluationContext(mission, req, base)
	rules := append([]PolicyRule(nil), bundle.Rules...)
	slices.SortFunc(rules, func(a, b PolicyRule) int {
		if a.Priority == b.Priority {
			return strings.Compare(a.RuleID, b.RuleID)
		}
		if a.Priority < b.Priority {
			return -1
		}
		return 1
	})

	resp := base
	for _, rule := range rules {
		result := PolicyRuleResult{RuleID: rule.RuleID, Effect: rule.Effect}
		if rule.Disabled {
			evaluation.RuleResults = append(evaluation.RuleResults, result)
			continue
		}
		if !policyRuleMatch(rule.Match, mission, req, base.Decision) {
			evaluation.RuleResults = append(evaluation.RuleResults, result)
			continue
		}
		conditionsOK, _, conditionResults, err := evaluateConditionsWithEvidence(rule.Conditions, context)
		result.ConditionResults = conditionResults
		if err != nil {
			result.Error = err.Error()
			evaluation.RuleResults = append(evaluation.RuleResults, result)
			return EvaluateResponse{}, evaluation, err
		}
		if !conditionsOK {
			evaluation.RuleResults = append(evaluation.RuleResults, result)
			continue
		}
		result.Matched = true
		result.Applied = policyEffectCanApply(rule.Effect, base.Decision)
		result.ReasonCodes = policyReasonCodes(rule)
		evaluation.RuleResults = append(evaluation.RuleResults, result)
		if result.Applied {
			resp = applyPolicyRuleDecision(bundle, rule, base)
			return resp, evaluation, nil
		}
	}
	return resp, evaluation, nil
}

func policyMayRestrictDecision(decision Decision) bool {
	return decision == DecisionAllow || decision == DecisionAllowWithConstraint || decision == DecisionRequireExpansion || decision == DecisionRequireApproval
}

func policyEffectCanApply(effect string, base Decision) bool {
	switch effect {
	case PolicyEffectDeny, PolicyEffectRequireApproval:
		return true
	case PolicyEffectRequireExpansion:
		return base == DecisionAllow || base == DecisionAllowWithConstraint
	case PolicyEffectAllowWithConstraints:
		return base == DecisionAllow || base == DecisionAllowWithConstraint
	case PolicyEffectAllow:
		return base == DecisionAllow || base == DecisionAllowWithConstraint
	default:
		return false
	}
}

func applyPolicyRuleDecision(bundle PolicyBundle, rule PolicyRule, base EvaluateResponse) EvaluateResponse {
	resp := base
	resp.Decision = Decision(rule.Effect)
	if rule.Effect == PolicyEffectAllow {
		resp.Decision = base.Decision
	}
	if len(rule.ReasonCodes) > 0 {
		resp.ReasonCodes = append(resp.ReasonCodes, rule.ReasonCodes...)
	} else {
		resp.ReasonCodes = append(resp.ReasonCodes, "POLICY_RULE_APPLIED", rule.RuleID)
	}
	if rule.HumanReason != "" {
		resp.HumanReason = rule.HumanReason
	}
	if len(rule.Constraints) > 0 {
		resp.Constraints = mergeConstraints(resp.Constraints, rule.Constraints)
	}
	if resp.Constraints == nil {
		resp.Constraints = map[string]any{}
	}
	resp.Constraints["policy_bundle_id"] = bundle.BundleID
	resp.Constraints["policy_version"] = bundle.Version
	resp.Constraints["policy_rule_id"] = rule.RuleID
	if resp.Decision == DecisionRequireApproval && resp.Escalation == nil {
		resp.Escalation = &Escalation{Type: "policy_approval", URL: "/v1/policy-bundles/" + bundle.BundleID}
	}
	return resp
}

func policyReasonCodes(rule PolicyRule) []string {
	if len(rule.ReasonCodes) > 0 {
		return append([]string(nil), rule.ReasonCodes...)
	}
	return []string{"POLICY_RULE_APPLIED", rule.RuleID}
}

func mergeConstraints(base map[string]any, extra map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func policyRuleMatch(match PolicyRuleMatch, mission Mission, req EvaluateRequest, base Decision) bool {
	if len(match.BaseDecisions) > 0 && !slices.Contains(match.BaseDecisions, base) {
		return false
	}
	if len(match.ActionTypes) > 0 && !contains(match.ActionTypes, req.Action.Type) {
		return false
	}
	if len(match.Operations) > 0 && !contains(match.Operations, req.Action.Operation) && !contains(match.Operations, req.Action.Name) {
		return false
	}
	if len(match.ResourceTypes) > 0 && !contains(match.ResourceTypes, req.Action.Resource.Type) {
		return false
	}
	if len(match.ResourceIDs) > 0 && !policyResourceIDMatches(match.ResourceIDs, req.Action.Resource.ID) {
		return false
	}
	if len(match.AgentClientIDs) > 0 && !contains(match.AgentClientIDs, req.Actor.ClientID) && !contains(match.AgentClientIDs, mission.Agent.ClientID) {
		return false
	}
	if len(match.PrincipalSubjects) > 0 && !contains(match.PrincipalSubjects, mission.Principal.Subject) {
		return false
	}
	return true
}

func policyResourceIDMatches(patterns []string, id string) bool {
	for _, pattern := range patterns {
		if resourceIDMatches(pattern, id) {
			return true
		}
	}
	return false
}

func policyEvaluationContext(mission Mission, req EvaluateRequest, base EvaluateResponse) map[string]any {
	return map[string]any{
		"mission": map[string]any{
			"ref":       mission.MissionRef,
			"tenant_id": mission.TenantID,
			"version":   mission.Version,
			"state":     string(mission.State),
			"risk":      mission.Risk,
		},
		"principal": map[string]any{
			"subject":        mission.Principal.Subject,
			"issuer":         mission.Principal.Issuer,
			"tenant_subject": mission.Principal.TenantSubject,
		},
		"agent": map[string]any{
			"provider":       mission.Agent.Provider,
			"client_id":      mission.Agent.ClientID,
			"instance_id":    mission.Agent.InstanceID,
			"key_thumbprint": mission.Agent.KeyThumbprint,
		},
		"actor": map[string]any{
			"client_id":         req.Actor.ClientID,
			"agent_instance_id": req.Actor.AgentInstanceID,
			"key_thumbprint":    req.Actor.KeyThumbprint,
		},
		"action": map[string]any{
			"type":      req.Action.Type,
			"name":      req.Action.Name,
			"operation": req.Action.Operation,
			"resource": map[string]any{
				"type": req.Action.Resource.Type,
				"id":   req.Action.Resource.ID,
			},
		},
		"context":       req.Context,
		"base_decision": string(base.Decision),
	}
}

package mission

import (
	"testing"
)

func TestActivePolicyBundleRestrictsAllowedEvaluationAndStoresEvidence(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	bundle := createPolicyBundle(t, service, CreatePolicyBundleRequest{
		TenantID: "demo",
		Version:  "mission-policy/v2",
		Name:     "Sensitive export controls",
		Rules: []PolicyRule{{
			RuleID:   "deny-high-risk-board-read",
			Priority: 10,
			Effect:   PolicyEffectDeny,
			Match: PolicyRuleMatch{
				Operations:    []string{"read"},
				ResourceTypes: []string{"drive_folder"},
				ResourceIDs:   []string{"board"},
				BaseDecisions: []Decision{DecisionAllow},
			},
			Conditions:  []Condition{{ID: "high-risk", Expression: "context.risk == 'high'"}},
			ReasonCodes: []string{"POLICY_HIGH_RISK_BLOCK"},
			HumanReason: "High-risk board-folder reads are blocked by enterprise policy.",
		}},
	})
	activated, err := service.ActivatePolicyBundle(bundle.BundleID, ActivatePolicyBundleRequest{Reason: "enterprise rollout"}, Principal{Subject: "security@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("ActivatePolicyBundle: %v", err)
	}
	if activated.Status != PolicyBundleStatusActive || activated.Signature == "" {
		t.Fatalf("activated bundle = %#v, want active with signature", activated)
	}

	resp, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "high"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if resp.Decision != DecisionDeny || !contains(resp.ReasonCodes, "POLICY_HIGH_RISK_BLOCK") {
		t.Fatalf("policy decision = %#v, want deny with policy reason", resp)
	}
	payload, err := VerifyDecisionArtifact(resp.DecisionArtifact, service.artifactKey)
	if err != nil {
		t.Fatalf("VerifyDecisionArtifact: %v", err)
	}
	if payload.PolicyVersion != bundle.Version || payload.PolicyBundleID != bundle.BundleID || len(payload.PolicyRuleIDs) != 1 || payload.PolicyRuleIDs[0] != "deny-high-risk-board-read" {
		t.Fatalf("artifact policy payload = %#v", payload)
	}
	evidence, err := service.governance.GetEvaluationEvidence(payload.EvidenceID)
	if err != nil {
		t.Fatalf("GetEvaluationEvidence: %v", err)
	}
	if evidence.PolicyBundleID != bundle.BundleID || len(evidence.PolicyResults) != 1 || !evidence.PolicyResults[0].Applied {
		t.Fatalf("evidence policy results = %#v", evidence)
	}
}

func TestPolicyBundleSimulationIsSideEffectFree(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	bundle := createPolicyBundle(t, service, CreatePolicyBundleRequest{
		TenantID: "demo",
		Version:  "mission-policy/sim",
		Rules: []PolicyRule{{
			RuleID:      "approval-for-delete",
			Effect:      PolicyEffectRequireApproval,
			Match:       PolicyRuleMatch{Operations: []string{"delete"}, BaseDecisions: []Decision{DecisionAllow}},
			ReasonCodes: []string{"POLICY_DELETE_APPROVAL"},
		}},
	})

	beforeEvents := len(service.Events())
	resp, err := service.SimulatePolicyBundle(bundle.BundleID, SimulatePolicyBundleRequest{
		MissionRef:   mission.MissionRef,
		BaseDecision: DecisionAllow,
		Evaluation: EvaluateRequest{
			Actor:   Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action:  Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "delete"},
			Context: map[string]any{"finance.close.status": "open"},
		},
	})
	if err != nil {
		t.Fatalf("SimulatePolicyBundle: %v", err)
	}
	if resp.Decision != DecisionRequireApproval || len(resp.RuleResults) != 1 || !resp.RuleResults[0].Applied {
		t.Fatalf("simulation = %#v, want require approval with applied rule", resp)
	}
	if afterEvents := len(service.Events()); afterEvents != beforeEvents {
		t.Fatalf("simulation appended events: before=%d after=%d", beforeEvents, afterEvents)
	}
}

func TestPolicyAllowDoesNotWidenMissionScope(t *testing.T) {
	service := testService()
	mission := approveTestMission(t, service)
	bundle := createPolicyBundle(t, service, CreatePolicyBundleRequest{
		TenantID: "demo",
		Version:  "mission-policy/no-widening",
		Rules: []PolicyRule{{
			RuleID: "allow-external-send",
			Effect: PolicyEffectAllow,
			Match:  PolicyRuleMatch{Operations: []string{"send_external"}, BaseDecisions: []Decision{DecisionRequireExpansion}},
		}},
	})
	if _, err := service.ActivatePolicyBundle(bundle.BundleID, ActivatePolicyBundleRequest{}, Principal{Subject: "security@example.com", Issuer: "https://idp.example.com"}); err != nil {
		t.Fatalf("ActivatePolicyBundle: %v", err)
	}

	resp, err := service.Evaluate(mission.MissionRef, EvaluateRequest{
		MissionVersionSeen: mission.MissionVersion,
		Actor:              Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
		Action:             Action{Type: "tool_call", Resource: ActionResource{Type: "email", ID: "board"}, Operation: "send_external"},
		Context:            map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if resp.Decision != DecisionRequireExpansion {
		t.Fatalf("policy widened scope: got %#v", resp)
	}
}

func createPolicyBundle(t *testing.T, service *Service, req CreatePolicyBundleRequest) PolicyBundle {
	t.Helper()
	bundle, err := service.CreatePolicyBundle(req, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreatePolicyBundle: %v", err)
	}
	if bundle.BundleHash == "" || bundle.Status != PolicyBundleStatusDraft {
		t.Fatalf("created bundle = %#v, want draft with hash", bundle)
	}
	return bundle
}

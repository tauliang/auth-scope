package mission

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPolicyBundleHTTPFlow(t *testing.T) {
	router := testRouter()
	mission := createAPIMission(t, router)

	create := httptest.NewRecorder()
	router.ServeHTTP(create, jsonRequest(http.MethodPost, "/v1/policy-bundles", CreatePolicyBundleRequest{
		TenantID: "demo",
		Version:  "mission-policy/http",
		Rules: []PolicyRule{{
			RuleID:      "require-approval-for-write",
			Effect:      PolicyEffectRequireApproval,
			Match:       PolicyRuleMatch{Operations: []string{"write_draft"}, BaseDecisions: []Decision{DecisionAllow}},
			ReasonCodes: []string{"POLICY_WRITE_APPROVAL"},
		}},
	}))
	if create.Code != http.StatusCreated {
		t.Fatalf("create policy status = %d body=%s", create.Code, create.Body.String())
	}
	var bundle PolicyBundle
	decodeTestJSON(t, create.Body.Bytes(), &bundle)
	if bundle.BundleID == "" || bundle.BundleHash == "" || bundle.Status != PolicyBundleStatusDraft {
		t.Fatalf("bundle response = %#v", bundle)
	}

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/policy-bundles", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list policy status = %d body=%s", list.Code, list.Body.String())
	}
	var listed struct {
		PolicyBundles []PolicyBundle `json:"policy_bundles"`
	}
	decodeTestJSON(t, list.Body.Bytes(), &listed)
	if len(listed.PolicyBundles) != 1 || listed.PolicyBundles[0].BundleID != bundle.BundleID {
		t.Fatalf("listed policy bundles = %#v", listed)
	}

	activate := httptest.NewRecorder()
	router.ServeHTTP(activate, jsonRequest(http.MethodPost, "/v1/policy-bundles/"+bundle.BundleID+"/activate", ActivatePolicyBundleRequest{Reason: "http test"}))
	if activate.Code != http.StatusOK {
		t.Fatalf("activate policy status = %d body=%s", activate.Code, activate.Body.String())
	}
	var activated PolicyBundle
	decodeTestJSON(t, activate.Body.Bytes(), &activated)
	if activated.Status != PolicyBundleStatusActive || activated.Signature == "" {
		t.Fatalf("activated bundle = %#v", activated)
	}

	simulate := httptest.NewRecorder()
	router.ServeHTTP(simulate, jsonRequest(http.MethodPost, "/v1/policy-bundles/"+bundle.BundleID+"/simulate", SimulatePolicyBundleRequest{
		MissionRef:   mission.MissionRef,
		BaseDecision: DecisionAllow,
		Evaluation: EvaluateRequest{
			Actor:   Actor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action:  Action{Type: "tool_call", Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "write_draft"},
			Context: map[string]any{"finance.close.status": "open"},
		},
	}))
	if simulate.Code != http.StatusOK {
		t.Fatalf("simulate policy status = %d body=%s", simulate.Code, simulate.Body.String())
	}
	var simulated SimulatePolicyBundleResponse
	decodeTestJSON(t, simulate.Body.Bytes(), &simulated)
	if simulated.Decision != DecisionRequireApproval || len(simulated.RuleResults) != 1 || !simulated.RuleResults[0].Applied {
		t.Fatalf("simulated policy = %#v", simulated)
	}
}

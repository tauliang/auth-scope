package mission

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIOktaIntegrationLifecycle(t *testing.T) {
	service := testService()
	mission := approveOktaMission(t, service)
	router := withTestAdminAuthorization(NewHandler(service).Routes())

	createBinding := httptest.NewRecorder()
	router.ServeHTTP(createBinding, jsonRequest(http.MethodPost, "/v1/integrations/okta/app-bindings", CreateOktaAppBindingRequest{
		Issuer:         "https://acme.okta.com/oauth2/default",
		ClientID:       "0oaabc123client",
		AppID:          "0oaapp123",
		AppLabel:       "Auth Scope Console",
		MissionRef:     mission.MissionRef,
		RequiredGroups: []string{"Mission Operators"},
		AdminGroups:    []string{"Mission Admins"},
	}))
	if createBinding.Code != http.StatusCreated {
		t.Fatalf("create binding status = %d body=%s", createBinding.Code, createBinding.Body.String())
	}
	var binding OktaAppBinding
	decodeTestJSON(t, createBinding.Body.Bytes(), &binding)
	if binding.ClientID != "0oaabc123client" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected binding: %#v", binding)
	}

	listBindings := httptest.NewRecorder()
	router.ServeHTTP(listBindings, httptest.NewRequest(http.MethodGet, "/v1/integrations/okta/app-bindings", nil))
	if listBindings.Code != http.StatusOK {
		t.Fatalf("list bindings status = %d body=%s", listBindings.Code, listBindings.Body.String())
	}

	resolve := httptest.NewRecorder()
	router.ServeHTTP(resolve, jsonRequest(http.MethodPost, "/v1/integrations/okta/authority-context/resolve", ResolveOktaAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://acme.okta.com/oauth2/default",
			"cid":    "0oaabc123client",
			"sub":    "00u1agent",
			"groups": []any{"Mission Operators"},
			"scp":    []any{"openid", "groups"},
		},
		Evaluation: &OktaEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              OktaActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: OktaEvaluationAction{
				Type:      "tool_call",
				Resource:  OktaEvaluationActionResource{Type: "drive_folder", ID: "board"},
				Operation: "read",
			},
		},
	}))
	if resolve.Code != http.StatusOK {
		t.Fatalf("resolve status = %d body=%s", resolve.Code, resolve.Body.String())
	}
	var resolved OktaAuthorityContextResponse
	decodeTestJSON(t, resolve.Body.Bytes(), &resolved)
	if !resolved.Accepted || resolved.Evaluation == nil || resolved.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("unexpected resolved response: %#v", resolved)
	}

	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, jsonRequest(http.MethodPost, "/v1/integrations/okta/authority-context/resolve", ResolveOktaAuthorityContextRequest{
		Claims: map[string]any{
			"iss": "https://acme.okta.com/oauth2/default",
			"cid": "0oaabc123client",
			"sub": "00u1agent",
		},
	}))
	if denied.Code != http.StatusOK {
		t.Fatalf("denied status = %d body=%s", denied.Code, denied.Body.String())
	}
	var deniedResp OktaAuthorityContextResponse
	decodeTestJSON(t, denied.Body.Bytes(), &deniedResp)
	if deniedResp.Accepted || deniedResp.ReasonCodes[0] != "okta_required_group_missing" {
		t.Fatalf("unexpected denied response: %#v", deniedResp)
	}
}

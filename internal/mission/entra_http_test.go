package mission

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIEntraIntegrationLifecycle(t *testing.T) {
	service := testService()
	mission := approveEntraMission(t, service)
	router := withTestAdminAuthorization(NewHandler(service).Routes())

	createRegistration := httptest.NewRecorder()
	router.ServeHTTP(createRegistration, jsonRequest(http.MethodPost, "/v1/integrations/entra/app-registrations", CreateEntraAppRegistrationRequest{
		Issuer:         "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
		ClientID:       "00000000-0000-0000-0000-000000000000",
		AppID:          "app_entra_001",
		AppName:        "Auth Scope Console",
		MissionRef:     mission.MissionRef,
		RequiredGroups: []string{"Mission Operators"},
		AdminGroups:    []string{"Mission Admins"},
	}))
	if createRegistration.Code != http.StatusCreated {
		t.Fatalf("create registration status = %d body=%s", createRegistration.Code, createRegistration.Body.String())
	}
	var registration EntraAppRegistration
	decodeTestJSON(t, createRegistration.Body.Bytes(), &registration)
	if registration.ClientID != "00000000-0000-0000-0000-000000000000" || registration.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected registration: %#v", registration)
	}

	listRegistrations := httptest.NewRecorder()
	router.ServeHTTP(listRegistrations, httptest.NewRequest(http.MethodGet, "/v1/integrations/entra/app-registrations", nil))
	if listRegistrations.Code != http.StatusOK {
		t.Fatalf("list registrations status = %d body=%s", listRegistrations.Code, listRegistrations.Body.String())
	}

	resolve := httptest.NewRecorder()
	router.ServeHTTP(resolve, jsonRequest(http.MethodPost, "/v1/integrations/entra/authority-context/resolve", ResolveEntraAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"appid":  "00000000-0000-0000-0000-000000000000",
			"sub":    "user@example.onmicrosoft.com",
			"groups": []any{"Mission Operators"},
			"roles":  []any{"Reader"},
		},
		Evaluation: &EntraEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              EntraActor{AgentInstanceID: "inst_456", ClientID: "research-agent"},
			Action: EntraEvaluationAction{
				Type:      "tool_call",
				Resource:  EntraEvaluationActionResource{Type: "drive_folder", ID: "board"},
				Operation: "read",
			},
		},
	}))
	if resolve.Code != http.StatusOK {
		t.Fatalf("resolve status = %d body=%s", resolve.Code, resolve.Body.String())
	}
	var resolved EntraAuthorityContextResponse
	decodeTestJSON(t, resolve.Body.Bytes(), &resolved)
	if !resolved.Accepted || resolved.Evaluation == nil || resolved.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("unexpected resolved response: %#v", resolved)
	}

	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, jsonRequest(http.MethodPost, "/v1/integrations/entra/authority-context/resolve", ResolveEntraAuthorityContextRequest{
		Claims: map[string]any{
			"iss":   "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"appid": "00000000-0000-0000-0000-000000000000",
			"sub":   "user@example.onmicrosoft.com",
		},
	}))
	if denied.Code != http.StatusOK {
		t.Fatalf("denied status = %d body=%s", denied.Code, denied.Body.String())
	}
	var deniedResp EntraAuthorityContextResponse
	decodeTestJSON(t, denied.Body.Bytes(), &deniedResp)
	if deniedResp.Accepted || deniedResp.ReasonCodes[0] != "entra_required_group_missing" {
		t.Fatalf("unexpected denied response: %#v", deniedResp)
	}
}

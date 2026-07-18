package mission

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOperatorAPIRequiresAdminAndBindsSessionPrincipal(t *testing.T) {
	service := testService()
	principal := Principal{Subject: "operator@example.com", Issuer: "https://idp.example.com", TenantSubject: "operator@demo"}
	router := NewHandlerWithAdminAuthenticator(service, NewBearerAdminAuthenticator("operator-token", principal)).Routes()

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/admin/session", nil))
	if unauthorized.Code != http.StatusUnauthorized || unauthorized.Header().Get("x-request-id") == "" {
		t.Fatalf("unauthorized status/header = %d/%q", unauthorized.Code, unauthorized.Header().Get("x-request-id"))
	}
	var errorBody map[string]any
	decodeTestJSON(t, unauthorized.Body.Bytes(), &errorBody)
	if errorBody["code"] != "authentication_required" || errorBody["message"] == "" || errorBody["error"] == "" {
		t.Fatalf("error body = %#v", errorBody)
	}

	session := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/session", nil)
	req.Header.Set("Authorization", "Bearer operator-token")
	req.Header.Set("x-request-id", "frontend-request")
	router.ServeHTTP(session, req)
	if session.Code != http.StatusOK || session.Header().Get("x-request-id") != "frontend-request" {
		t.Fatalf("session status/header = %d/%q", session.Code, session.Header().Get("x-request-id"))
	}
	var response AdminSessionResponse
	decodeTestJSON(t, session.Body.Bytes(), &response)
	if response.Principal != principal || !response.Capabilities["approve"] || response.APIVersion != "v1" {
		t.Fatalf("session = %#v", response)
	}
}

func TestOperatorAPICollectionsSummaryAndValidation(t *testing.T) {
	router := testRouter()
	createAPIMission(t, router)

	tests := []struct {
		path string
		key  string
	}{
		{"/v1/operations/summary", "missions_total"},
		{"/v1/missions?limit=1&state=active", "items"},
		{"/v1/mission-proposals?limit=1", "items"},
		{"/v1/expansion-requests?status=pending", "items"},
		{"/v1/agents?status=active", "items"},
		{"/v1/tool-contracts?q=drive", "items"},
		{"/v1/projections?status=active", "items"},
		{"/v1/events?limit=2", "items"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
			}
			var body map[string]json.RawMessage
			decodeTestJSON(t, response.Body.Bytes(), &body)
			if _, ok := body[test.key]; !ok {
				t.Fatalf("missing %q in %s", test.key, response.Body.String())
			}
		})
	}

	badLimit := httptest.NewRecorder()
	router.ServeHTTP(badLimit, httptest.NewRequest(http.MethodGet, "/v1/missions?limit=zero", nil))
	if badLimit.Code != http.StatusBadRequest || !strings.Contains(badLimit.Body.String(), "positive integer") {
		t.Fatalf("bad limit = %d %s", badLimit.Code, badLimit.Body.String())
	}
	badCursor := httptest.NewRecorder()
	router.ServeHTTP(badCursor, httptest.NewRequest(http.MethodGet, "/v1/events?cursor=bad", nil))
	if badCursor.Code != http.StatusBadRequest || !strings.Contains(badCursor.Body.String(), "invalid cursor") {
		t.Fatalf("bad cursor = %d %s", badCursor.Code, badCursor.Body.String())
	}

	empty := httptest.NewRecorder()
	router.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/v1/agents?q=definitely-missing", nil))
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"items":[]`) {
		t.Fatalf("empty collection = %d %s", empty.Code, empty.Body.String())
	}
}

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

func TestTenantScopedAdminCannotCrossTenantBoundaries(t *testing.T) {
	service := testService()
	demoProposal := validProposalRequest()
	demoProposal.TenantID = "demo"
	if _, err := service.CreateProposal(demoProposal); err != nil {
		t.Fatalf("create demo proposal: %v", err)
	}
	otherProposal := validProposalRequest()
	otherProposal.TenantID = "other"
	otherProposal.Agent.InstanceID = "inst_other"
	if _, err := service.CreateProposal(otherProposal); err != nil {
		t.Fatalf("create other proposal: %v", err)
	}

	authenticator := NewBearerAdminAuthenticator("tenant-token", Principal{
		Subject:       "operator@example.com",
		Issuer:        "https://idp.example.com",
		TenantSubject: "demo",
	})
	router := NewHandlerWithAdminAuthenticator(service, authenticator).Routes()

	list := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/mission-proposals", nil)
	listReq.Header.Set("Authorization", "Bearer tenant-token")
	router.ServeHTTP(list, listReq)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", list.Code, list.Body.String())
	}
	var page CollectionPage[MissionProposal]
	decodeTestJSON(t, list.Body.Bytes(), &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].TenantID != "demo" {
		t.Fatalf("tenant-scoped page = %#v", page)
	}

	crossTenantList := httptest.NewRecorder()
	crossTenantReq := httptest.NewRequest(http.MethodGet, "/v1/mission-proposals?tenant_id=other", nil)
	crossTenantReq.Header.Set("Authorization", "Bearer tenant-token")
	router.ServeHTTP(crossTenantList, crossTenantReq)
	if crossTenantList.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant list status = %d body=%s", crossTenantList.Code, crossTenantList.Body.String())
	}

	createOther := httptest.NewRecorder()
	createReq := jsonRequestAsAdmin(http.MethodPost, "/v1/mission-proposals", otherProposal, "tenant-token")
	router.ServeHTTP(createOther, createReq)
	if createOther.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant create status = %d body=%s", createOther.Code, createOther.Body.String())
	}

	detail := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/v1/mission-proposals/"+page.Items[0].ProposalID, nil)
	detailReq.Header.Set("Authorization", "Bearer tenant-token")
	router.ServeHTTP(detail, detailReq)
	if detail.Code != http.StatusOK {
		t.Fatalf("tenant proposal detail status = %d body=%s", detail.Code, detail.Body.String())
	}

	rule, err := service.CreateContainmentRule(ContainmentRule{TenantID: "demo", TargetType: ContainmentTargetTenant, TargetID: "demo", CreatedBy: Principal{Subject: "admin"}})
	if err != nil {
		t.Fatalf("create containment rule: %v", err)
	}
	ruleDetail := httptest.NewRecorder()
	ruleReq := httptest.NewRequest(http.MethodGet, "/v1/containment-rules/"+rule.RuleID, nil)
	ruleReq.Header.Set("Authorization", "Bearer tenant-token")
	router.ServeHTTP(ruleDetail, ruleReq)
	if ruleDetail.Code != http.StatusOK {
		t.Fatalf("tenant containment detail status = %d body=%s", ruleDetail.Code, ruleDetail.Body.String())
	}
}

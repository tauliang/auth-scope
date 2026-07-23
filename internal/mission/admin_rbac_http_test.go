package mission

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminRBACAllowsAuditorReadAndDeniesGovernanceWrite(t *testing.T) {
	service := testService()
	router := NewHandlerWithAdminAuthenticator(service, NewCredentialAdminAuthenticator([]AdminCredential{{
		Token:         "audit-token",
		Subject:       "auditor@example.com",
		Issuer:        "https://idp.example.com",
		TenantSubject: "demo",
		Roles:         []string{string(AdminRoleAuditor)},
	}})).Routes()

	read := httptest.NewRecorder()
	readReq := httptest.NewRequest(http.MethodGet, "/v1/events?tenant_id=demo", nil)
	readReq.Header.Set("Authorization", "Bearer audit-token")
	router.ServeHTTP(read, readReq)
	if read.Code != http.StatusOK {
		t.Fatalf("read status = %d body=%s", read.Code, read.Body.String())
	}

	write := httptest.NewRecorder()
	router.ServeHTTP(write, jsonRequestAsAdmin(http.MethodPost, "/v1/approval-rules", ApprovalRule{
		TenantID:          "demo",
		AppliesTo:         ApprovalAppliesExpansion,
		RequiredApprovals: 2,
	}, "audit-token"))
	if write.Code != http.StatusForbidden {
		t.Fatalf("write status = %d body=%s", write.Code, write.Body.String())
	}
	audit := latestAdminAuditEvent(t, service)
	if audit.Payload["allowed"] != false || audit.Payload["permission"] != string(AdminPermissionManageGovernance) {
		t.Fatalf("audit payload = %#v", audit.Payload)
	}
	if audit.Actor["subject"] != "auditor@example.com" || !containsAny(audit.Actor["roles"], string(AdminRoleAuditor)) {
		t.Fatalf("audit actor = %#v", audit.Actor)
	}
}

func TestAdminRBACScopesIntegrationAdminAndAuditsTenantDenial(t *testing.T) {
	service := testService()
	router := NewHandlerWithAdminAuthenticator(service, NewCredentialAdminAuthenticator([]AdminCredential{{
		Token:         "integration-token",
		Subject:       "integrator@example.com",
		Issuer:        "https://idp.example.com",
		TenantSubject: "demo",
		Roles:         []string{string(AdminRoleIntegrationAdmin)},
	}})).Routes()

	create := httptest.NewRecorder()
	router.ServeHTTP(create, jsonRequestAsAdmin(http.MethodPost, "/v1/integrations/github/repositories", CreateGitHubRepositoryBindingRequest{
		TenantID:   "demo",
		Repository: "acme/auth-scope",
	}, "integration-token"))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}

	crossTenant := httptest.NewRecorder()
	router.ServeHTTP(crossTenant, jsonRequestAsAdmin(http.MethodPost, "/v1/integrations/github/repositories", CreateGitHubRepositoryBindingRequest{
		TenantID:   "other",
		Repository: "acme/other",
	}, "integration-token"))
	if crossTenant.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant status = %d body=%s", crossTenant.Code, crossTenant.Body.String())
	}
	audit := latestAdminAuditEvent(t, service)
	if audit.Payload["status_code"] != float64(http.StatusForbidden) && audit.Payload["status_code"] != http.StatusForbidden {
		t.Fatalf("audit payload = %#v", audit.Payload)
	}

	proposal, err := service.CreateProposal(validProposalRequest())
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	approve := httptest.NewRecorder()
	router.ServeHTTP(approve, jsonRequestAsAdmin(http.MethodPost, "/v1/mission-proposals/"+proposal.ProposalID+"/approve", ApproveProposalRequest{}, "integration-token"))
	if approve.Code != http.StatusForbidden {
		t.Fatalf("approve status = %d body=%s", approve.Code, approve.Body.String())
	}
	audit = latestAdminAuditEvent(t, service)
	if audit.Payload["allowed"] != false || audit.Payload["permission"] != string(AdminPermissionApprove) {
		t.Fatalf("approve audit payload = %#v", audit.Payload)
	}
}

func latestAdminAuditEvent(t *testing.T, service *Service) Event {
	t.Helper()
	events := service.Events()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == "admin.action" {
			return events[i]
		}
	}
	t.Fatalf("admin.action event not found in %#v", events)
	return Event{}
}

func containsAny(value any, target string) bool {
	switch typed := value.(type) {
	case []string:
		return containsString(typed, target)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text == target {
				return true
			}
		}
	}
	return false
}

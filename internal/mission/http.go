package mission

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxJSONRequestBodyBytes int64 = 1 << 20

type Handler struct {
	services               HandlerServices
	adminAuthenticator     AdminAuthenticator
	requireAgentSignatures bool
}

func NewHandler(service *Service) *Handler {
	return NewHandlerWithOptions(service, AdminAuthenticatorFromEnv(), HandlerOptions{})

}

func NewHandlerWithAdminAuthenticator(service *Service, authenticator AdminAuthenticator) *Handler {
	return NewHandlerWithOptions(service, authenticator, HandlerOptions{})
}

type HandlerOptions struct {
	RequireAgentSignatures bool
}

func NewHandlerWithOptions(service *Service, authenticator AdminAuthenticator, options HandlerOptions) *Handler {
	return NewHandlerWithServices(HandlerServices{
		Identity:        service,
		Mission:         service,
		Governance:      service,
		Projection:      service,
		GrandGovernance: service,
		AuthZEN:         service,
		Operator:        service,
		GitHub:          service,
		Okta:            service,
		Entra:           service,
		Slack:           service,
		Atlassian:       service,
	}, authenticator, options)
}

func NewHandlerWithServices(services HandlerServices, authenticator AdminAuthenticator, options ...HandlerOptions) *Handler {
	var selected HandlerOptions
	if len(options) > 0 {
		selected = options[0]
	}
	return &Handler{services: services, adminAuthenticator: authenticator, requireAgentSignatures: selected.RequireAgentSignatures}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /.well-known/mission-authority", h.discovery)
	mux.HandleFunc("GET /.well-known/authzen-configuration", h.authZENDiscovery)
	mux.HandleFunc("POST /access/v1/evaluation", h.authZENEvaluation)
	mux.HandleFunc("POST /access/v1/evaluations", h.authZENEvaluations)
	mux.Handle("POST /v1/agents", h.requireAdmin(http.HandlerFunc(h.registerAgent)))
	mux.Handle("GET /v1/admin/session", h.requireAdmin(http.HandlerFunc(h.adminSession)))
	mux.Handle("GET /v1/operations/summary", h.requireAdmin(http.HandlerFunc(h.operationsSummary)))
	mux.Handle("GET /v1/agents", h.requireAdmin(http.HandlerFunc(h.listAgents)))
	mux.Handle("GET /v1/agents/{agent_id}", h.requireAdmin(http.HandlerFunc(h.getAgent)))
	mux.Handle("POST /v1/agents/{agent_id}/revoke", h.requireAdmin(http.HandlerFunc(h.revokeAgent)))
	mux.Handle("POST /v1/mission-proposals", h.requireAdmin(http.HandlerFunc(h.createProposal)))
	mux.Handle("GET /v1/mission-proposals", h.requireAdmin(http.HandlerFunc(h.listProposals)))
	mux.Handle("GET /v1/mission-proposals/{proposal_id}", h.requireAdmin(http.HandlerFunc(h.getProposal)))
	mux.Handle("POST /v1/mission-proposals/{proposal_id}/approve", h.requireAdmin(http.HandlerFunc(h.approveProposal)))
	mux.HandleFunc("POST /v1/missions/{mission_ref}/evaluate", h.evaluate)
	mux.Handle("GET /v1/missions", h.requireAdmin(http.HandlerFunc(h.listMissions)))
	mux.HandleFunc("POST /v1/missions/{mission_ref}/authority/negotiations", h.createAuthorityNegotiation)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/expansion-requests", h.createExpansionRequest)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/resume", h.resume)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/delegate", h.delegate)
	mux.Handle("POST /v1/missions/{mission_ref}/revoke", h.requireAdmin(http.HandlerFunc(h.revoke)))
	mux.Handle("POST /v1/missions/{mission_ref}/complete", h.requireAdmin(http.HandlerFunc(h.complete)))
	mux.Handle("GET /v1/missions/{mission_ref}/introspect", h.requireAdmin(http.HandlerFunc(h.introspect)))
	mux.Handle("GET /v1/missions/{mission_ref}/lineage", h.requireAdmin(http.HandlerFunc(h.missionLineage)))
	mux.Handle("GET /v1/agents/{agent_id}/lineage", h.requireAdmin(http.HandlerFunc(h.agentLineage)))
	mux.Handle("GET /v1/expansion-requests/{expansion_id}", h.requireAdmin(http.HandlerFunc(h.getExpansionRequest)))
	mux.Handle("GET /v1/expansion-requests", h.requireAdmin(http.HandlerFunc(h.listExpansions)))
	mux.Handle("POST /v1/expansion-requests/{expansion_id}/approve", h.requireAdmin(http.HandlerFunc(h.approveExpansionRequest)))
	mux.Handle("POST /v1/expansion-requests/{expansion_id}/deny", h.requireAdmin(http.HandlerFunc(h.denyExpansionRequest)))
	mux.Handle("GET /v1/authority/negotiations/{negotiation_id}", h.requireAdmin(http.HandlerFunc(h.getAuthorityNegotiation)))
	mux.HandleFunc("POST /v1/decision-artifacts/verify", h.verifyDecisionArtifact)
	mux.Handle("POST /v1/tool-contracts", h.requireAdmin(http.HandlerFunc(h.registerToolContract)))
	mux.Handle("GET /v1/tool-contracts", h.requireAdmin(http.HandlerFunc(h.listToolContracts)))
	mux.Handle("GET /v1/tool-contracts/{tool_name}", h.requireAdmin(http.HandlerFunc(h.getToolContract)))
	mux.HandleFunc("POST /v1/tool-calls/authorize", h.authorizeToolCall)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/projections", h.createProjection)
	mux.Handle("GET /v1/projections", h.requireAdmin(http.HandlerFunc(h.listProjections)))
	mux.Handle("GET /v1/projections/{projection_id}/status", h.requireAdmin(http.HandlerFunc(h.getProjectionStatus)))
	mux.Handle("POST /v1/projections/{projection_id}/revoke", h.requireAdmin(http.HandlerFunc(h.revokeProjection)))
	mux.HandleFunc("POST /v1/projections/verify", h.verifyProjection)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/leases", h.createMissionLease)
	mux.HandleFunc("POST /v1/leases/{lease_id}/refresh", h.refreshMissionLease)
	mux.Handle("POST /v1/approval-rules", h.requireAdmin(http.HandlerFunc(h.createApprovalRule)))
	mux.Handle("GET /v1/approval-rules", h.requireAdmin(http.HandlerFunc(h.listApprovalRules)))
	mux.Handle("POST /v1/expansion-requests/{expansion_id}/approvals", h.requireAdmin(http.HandlerFunc(h.submitExpansionApproval)))
	mux.Handle("POST /v1/containment-rules", h.requireAdmin(http.HandlerFunc(h.createContainmentRule)))
	mux.Handle("GET /v1/containment-rules", h.requireAdmin(http.HandlerFunc(h.listContainmentRules)))
	mux.Handle("GET /v1/containment-rules/{rule_id}", h.requireAdmin(http.HandlerFunc(h.getContainmentRule)))
	mux.Handle("POST /v1/containment-rules/{rule_id}/lift", h.requireAdmin(http.HandlerFunc(h.liftContainmentRule)))
	mux.Handle("GET /v1/containment-rules/{rule_id}/blast-radius", h.requireAdmin(http.HandlerFunc(h.containmentBlastRadius)))
	mux.Handle("POST /v1/integrations/github/repositories", h.requireAdmin(http.HandlerFunc(h.createGitHubRepositoryBinding)))
	mux.Handle("GET /v1/integrations/github/repositories", h.requireAdmin(http.HandlerFunc(h.listGitHubRepositoryBindings)))
	mux.HandleFunc("POST /v1/integrations/github/webhooks", h.githubWebhook)
	mux.HandleFunc("POST /v1/integrations/github/check-runs/plan", h.githubCheckRunPlan)
	mux.Handle("POST /v1/integrations/okta/app-bindings", h.requireAdmin(http.HandlerFunc(h.createOktaAppBinding)))
	mux.Handle("GET /v1/integrations/okta/app-bindings", h.requireAdmin(http.HandlerFunc(h.listOktaAppBindings)))
	mux.HandleFunc("POST /v1/integrations/okta/authority-context/resolve", h.resolveOktaAuthorityContext)
	mux.Handle("POST /v1/integrations/entra/app-registrations", h.requireAdmin(http.HandlerFunc(h.createEntraAppRegistration)))
	mux.Handle("GET /v1/integrations/entra/app-registrations", h.requireAdmin(http.HandlerFunc(h.listEntraAppRegistrations)))
	mux.HandleFunc("POST /v1/integrations/entra/authority-context/resolve", h.resolveEntraAuthorityContext)
	mux.Handle("POST /v1/integrations/slack/workspace-bindings", h.requireAdmin(http.HandlerFunc(h.createSlackWorkspaceBinding)))
	mux.Handle("GET /v1/integrations/slack/workspace-bindings", h.requireAdmin(http.HandlerFunc(h.listSlackWorkspaceBindings)))
	mux.HandleFunc("POST /v1/integrations/slack/message-actions/authorize", h.authorizeSlackMessageAction)
	mux.Handle("POST /v1/integrations/atlassian/site-bindings", h.requireAdmin(http.HandlerFunc(h.createAtlassianSiteBinding)))
	mux.Handle("GET /v1/integrations/atlassian/site-bindings", h.requireAdmin(http.HandlerFunc(h.listAtlassianSiteBindings)))
	mux.HandleFunc("POST /v1/integrations/atlassian/jira/issues/authorize", h.authorizeAtlassianJiraIssueAction)
	mux.HandleFunc("POST /v1/integrations/atlassian/confluence/pages/authorize", h.authorizeAtlassianConfluencePageAction)
	mux.Handle("GET /v1/events", h.requireAdmin(http.HandlerFunc(h.events)))
	mux.Handle("GET /v1/events/stream", h.requireAdmin(http.HandlerFunc(h.eventsStream)))
	return requestID(mux)
}

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.adminAuthenticator == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, ErrAdminAuthenticationRequired)
			return
		}
		principal, err := h.adminAuthenticator.Authenticate(r)
		if err != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, ErrAdminAuthenticationRequired)
			return
		}
		next.ServeHTTP(w, r.WithContext(withAdminPrincipal(r.Context(), principal)))
	})
}

func bindAdminTenant(w http.ResponseWriter, r *http.Request, tenantID *string) bool {
	tenant := authenticatedAdmin(r).TenantSubject
	if tenant == "" {
		return true
	}
	requested := strings.TrimSpace(*tenantID)
	if requested != "" && requested != tenant {
		writeError(w, http.StatusForbidden, fmt.Errorf("tenant %q is outside the administrator scope", requested))
		return false
	}
	*tenantID = tenant
	return true
}

func ensureAdminTenantAccess(w http.ResponseWriter, r *http.Request, tenantID string) bool {
	tenant := authenticatedAdmin(r).TenantSubject
	if tenant == "" {
		return true
	}
	if strings.TrimSpace(tenantID) != tenant {
		writeError(w, http.StatusForbidden, fmt.Errorf("tenant %q is outside the administrator scope", tenantID))
		return false
	}
	return true
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) discovery(w http.ResponseWriter, r *http.Request) {
	base := "http://" + r.Host
	if r.TLS != nil {
		base = "https://" + r.Host
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":   base,
		"api_base": base + "/v1",
		"supports": map[string]any{
			"agent_identity_registry":   true,
			"authzen_evaluation":        true,
			"signed_agent_requests":     true,
			"signed_decision_artifacts": true,
			"policy_evidence":           true,
			"tool_gateway_enforcement":  true,
			"signed_projections":        true,
			"mission_leases":            true,
			"approval_rules":            true,
			"authority_negotiation":     true,
			"containment_rules":         true,
			"lineage_graphs":            true,
			"github_integration":        true,
			"okta_integration":          true,
			"entra_integration":         true,
			"slack_integration":         true,
			"atlassian_integration":     true,
			"jira_integration":          true,
			"confluence_integration":    true,
			"mission_proposals":         true,
			"delegation":                true,
			"expansion_requests":        true,
			"projection_types":          []string{"oauth_claims", "authzen_context", "mcp_context", "decision_artifact"},
			"revocation_events":         []string{"events", "outbox_events"},
		},
	})
}

func (h *Handler) authZENDiscovery(w http.ResponseWriter, r *http.Request) {
	base := "http://" + r.Host
	if r.TLS != nil {
		base = "https://" + r.Host
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                  base,
		"authorization_endpoint":  base + "/access/v1/evaluation",
		"authorizations_endpoint": base + "/access/v1/evaluations",
		"capabilities": map[string]any{
			"evaluations":                     true,
			"subject_action_resource_context": true,
			"signed_agent_requests":           true,
			"signed_decision_artifacts":       true,
		},
	})
}

func (h *Handler) registerAgent(w http.ResponseWriter, r *http.Request) {
	var req RegisterAgentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Identity.RegisterAgent(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) getAgent(w http.ResponseWriter, r *http.Request) {
	identity, err := h.services.Identity.GetAgentIdentity(r.PathValue("agent_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, identity.TenantID) {
		return
	}
	writeJSON(w, http.StatusOK, identity)
}

func (h *Handler) revokeAgent(w http.ResponseWriter, r *http.Request) {
	var req StateChangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	existing, err := h.services.Identity.GetAgentIdentity(r.PathValue("agent_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, existing.TenantID) {
		return
	}
	req.Actor = principalActor(authenticatedAdmin(r))
	identity, err := h.services.Identity.RevokeAgent(r.PathValue("agent_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, identity)
}

func (h *Handler) createProposal(w http.ResponseWriter, r *http.Request) {
	var req CreateProposalRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Mission.CreateProposal(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) approveProposal(w http.ResponseWriter, r *http.Request) {
	var req ApproveProposalRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	proposal, err := h.services.Operator.GetProposal(r.PathValue("proposal_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, proposal.TenantID) {
		return
	}
	req.Approver = authenticatedAdmin(r)
	resp, err := h.services.Mission.ApproveProposal(r.PathValue("proposal_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) evaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Mission.Evaluate(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) createExpansionRequest(w http.ResponseWriter, r *http.Request) {
	var req CreateExpansionRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Requester, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Governance.CreateExpansionRequest(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) createAuthorityNegotiation(w http.ResponseWriter, r *http.Request) {
	var req CreateAuthorityNegotiationRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.GrandGovernance.CreateAuthorityNegotiation(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) getAuthorityNegotiation(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.GrandGovernance.GetAuthorityNegotiation(r.PathValue("negotiation_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getExpansionRequest(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Governance.GetExpansionRequest(r.PathValue("expansion_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, resp.TenantID) {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) approveExpansionRequest(w http.ResponseWriter, r *http.Request) {
	var req ExpansionDecisionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	expansion, err := h.services.Governance.GetExpansionRequest(r.PathValue("expansion_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, expansion.TenantID) {
		return
	}
	req.Approver = authenticatedAdmin(r)
	resp, err := h.services.Governance.ApproveExpansionRequestContext(r.Context(), r.PathValue("expansion_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) denyExpansionRequest(w http.ResponseWriter, r *http.Request) {
	var req ExpansionDecisionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	expansion, err := h.services.Governance.GetExpansionRequest(r.PathValue("expansion_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, expansion.TenantID) {
		return
	}
	req.Approver = authenticatedAdmin(r)
	resp, err := h.services.Governance.DenyExpansionRequestContext(r.Context(), r.PathValue("expansion_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) verifyDecisionArtifact(w http.ResponseWriter, r *http.Request) {
	var req VerifyDecisionArtifactRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	writeJSON(w, http.StatusOK, h.services.Governance.VerifyDecisionArtifactEvidence(req))
}

func (h *Handler) registerToolContract(w http.ResponseWriter, r *http.Request) {
	var req ToolContract
	if !decodeJSON(w, r, &req) {
		return
	}
	req.CreatedBy = authenticatedAdmin(r)
	resp, err := h.services.Governance.RegisterToolContract(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) getToolContract(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Governance.GetToolContract(r.PathValue("tool_name"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) authorizeToolCall(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeToolCallRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Governance.AuthorizeToolCall(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) createProjection(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectionRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Projection.CreateProjection(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) getProjectionStatus(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Projection.GetProjectionStatus(r.PathValue("projection_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, resp.TenantID) {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) revokeProjection(w http.ResponseWriter, r *http.Request) {
	var req StateChangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	status, err := h.services.Projection.GetProjectionStatus(r.PathValue("projection_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, status.TenantID) {
		return
	}
	req.Actor = principalActor(authenticatedAdmin(r))
	resp, err := h.services.Projection.RevokeProjection(r.PathValue("projection_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) verifyProjection(w http.ResponseWriter, r *http.Request) {
	var req VerifyProjectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	writeJSON(w, http.StatusOK, h.services.Projection.VerifyProjection(req))
}

func (h *Handler) createMissionLease(w http.ResponseWriter, r *http.Request) {
	var req CreateLeaseRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Projection.CreateMissionLease(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) refreshMissionLease(w http.ResponseWriter, r *http.Request) {
	var req RefreshLeaseRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Projection.RefreshMissionLease(r.PathValue("lease_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) createApprovalRule(w http.ResponseWriter, r *http.Request) {
	var req ApprovalRule
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	req.CreatedBy = authenticatedAdmin(r)
	resp, err := h.services.Projection.CreateApprovalRule(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listApprovalRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.services.Projection.ListApprovalRules()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]ApprovalRule, 0, len(rules))
		for _, rule := range rules {
			if rule.TenantID == tenant {
				filtered = append(filtered, rule)
			}
		}
		rules = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"approval_rules": rules})
}

func (h *Handler) submitExpansionApproval(w http.ResponseWriter, r *http.Request) {
	var req SubmitExpansionApprovalRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Approver = authenticatedAdmin(r)
	resp, err := h.services.Projection.SubmitExpansionApprovalContext(r.Context(), r.PathValue("expansion_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) createContainmentRule(w http.ResponseWriter, r *http.Request) {
	var req ContainmentRule
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	req.CreatedBy = authenticatedAdmin(r)
	resp, err := h.services.GrandGovernance.CreateContainmentRule(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listContainmentRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.services.GrandGovernance.ListContainmentRules()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]ContainmentRule, 0, len(rules))
		for _, rule := range rules {
			if rule.TenantID == tenant {
				filtered = append(filtered, rule)
			}
		}
		rules = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"containment_rules": rules})
}

func (h *Handler) liftContainmentRule(w http.ResponseWriter, r *http.Request) {
	var req StateChangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, err := h.services.Operator.GetContainmentRule(r.PathValue("rule_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, rule.TenantID) {
		return
	}
	req.Actor = principalActor(authenticatedAdmin(r))
	resp, err := h.services.GrandGovernance.LiftContainmentRule(r.PathValue("rule_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) containmentBlastRadius(w http.ResponseWriter, r *http.Request) {
	rule, err := h.services.Operator.GetContainmentRule(r.PathValue("rule_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, rule.TenantID) {
		return
	}
	resp, err := h.services.GrandGovernance.ContainmentBlastRadiusContext(r.Context(), r.PathValue("rule_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) resume(w http.ResponseWriter, r *http.Request) {
	var req ResumeRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Mission.Resume(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) delegate(w http.ResponseWriter, r *http.Request) {
	var req DelegationRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindActorIdentity(&req.DelegatingActor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Mission.Delegate(r.PathValue("mission_ref"), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	var req StateChangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	mission, err := h.services.Mission.Introspect(r.PathValue("mission_ref"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, mission.TenantID) {
		return
	}
	req.Actor = principalActor(authenticatedAdmin(r))
	resp, err := h.services.Mission.Revoke(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	var req StateChangeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	mission, err := h.services.Mission.Introspect(r.PathValue("mission_ref"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, mission.TenantID) {
		return
	}
	req.Actor = principalActor(authenticatedAdmin(r))
	resp, err := h.services.Mission.Complete(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) introspect(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Mission.Introspect(r.PathValue("mission_ref"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, resp.TenantID) {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) missionLineage(w http.ResponseWriter, r *http.Request) {
	mission, err := h.services.Mission.Introspect(r.PathValue("mission_ref"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, mission.TenantID) {
		return
	}
	resp, err := h.services.GrandGovernance.MissionLineageContext(r.Context(), r.PathValue("mission_ref"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) agentLineage(w http.ResponseWriter, r *http.Request) {
	identity, err := h.services.Identity.GetAgentIdentity(r.PathValue("agent_id"))
	if err == nil {
		if !ensureAdminTenantAccess(w, r, identity.TenantID) {
			return
		}
	} else if authenticatedAdmin(r).TenantSubject != "" {
		writeServiceError(w, err)
		return
	}
	resp, err := h.services.GrandGovernance.AgentLineageContext(r.Context(), r.PathValue("agent_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) eventsStream(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "text/event-stream")
	w.Header().Set("cache-control", "no-cache")
	for _, event := range h.services.Mission.Events() {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(w, "event: %s\nid: %s\ndata: %s\n\n", event.Type, event.EventID, data)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (h *Handler) authZENEvaluation(w http.ResponseWriter, r *http.Request) {
	var req AuthZENEvaluationRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		if err := bindAuthZENSubjectIdentity(&req.Subject, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.AuthZEN.EvaluateAuthZEN(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) authZENEvaluations(w http.ResponseWriter, r *http.Request) {
	var req AuthZENEvaluationsRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil {
		for i := range req.Evaluations {
			if err := bindAuthZENSubjectIdentity(&req.Evaluations[i].Subject, *identity); err != nil {
				writeError(w, http.StatusUnauthorized, err)
				return
			}
		}
	}
	resp, err := h.services.AuthZEN.EvaluateAuthZENBatch(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	_, ok := decodeJSONBody(w, r, dst)
	return ok
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONRequestBodyBytes))
	_ = r.Body.Close()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, errors.New("request body must contain a single JSON object"))
		return nil, false
	}
	return body, true
}

func (h *Handler) verifiedAgentIdentity(w http.ResponseWriter, r *http.Request, body []byte) (*AgentIdentity, bool) {
	agentID := r.Header.Get("x-auth-scope-agent-id")
	nonce := r.Header.Get("x-auth-scope-nonce")
	signature := r.Header.Get("x-auth-scope-signature")
	if agentID == "" && nonce == "" && signature == "" {
		if h.requireAgentSignatures {
			writeError(w, http.StatusUnauthorized, ErrInvalidSignature)
			return nil, false
		}
		return nil, true
	}
	if agentID == "" || nonce == "" || signature == "" {
		writeError(w, http.StatusUnauthorized, ErrInvalidSignature)
		return nil, false
	}
	identity, err := h.services.Identity.VerifyAgentRequestSignature(r.Method, r.URL.RequestURI(), body, agentID, nonce, signature)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrAgentRevoked) {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return nil, false
	}
	return &identity, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, ErrConflict):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusBadRequest, err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	code := "bad_request"
	switch status {
	case http.StatusUnauthorized:
		code = "authentication_required"
	case http.StatusForbidden:
		code = "forbidden"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusConflict:
		code = "conflict"
	case http.StatusInternalServerError:
		code = "internal_error"
	}
	message := strings.TrimSpace(err.Error())
	writeJSON(w, status, map[string]any{
		"error":   message,
		"code":    code,
		"message": message,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("x-request-id"))
		if id == "" {
			id = newID("req")
			r.Header.Set("x-request-id", id)
		}
		w.Header().Set("x-request-id", id)
		next.ServeHTTP(w, r)
	})
}

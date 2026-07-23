package mission

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) adminSession(w http.ResponseWriter, r *http.Request) {
	identity := authenticatedAdminIdentity(r)
	writeJSON(w, http.StatusOK, AdminSessionResponse{
		Principal:    identity.Principal,
		Provider:     identity.Provider,
		Groups:       identity.Groups,
		Roles:        identity.Roles,
		Permissions:  identity.Permissions,
		Capabilities: AdminCapabilitiesForIdentity(identity),
		APIVersion:   "v1",
	})
}

func (h *Handler) operationsSummary(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	if !scopeListQueryToAdmin(w, r, &query) {
		return
	}
	result, err := h.services.Operator.OperationsSummary(query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listMissions(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	if !scopeListQueryToAdmin(w, r, &query) {
		return
	}
	result, err := h.services.Operator.ListMissions(query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listProposals(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	if !scopeListQueryToAdmin(w, r, &query) {
		return
	}
	result, err := h.services.Operator.ListProposals(query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getProposal(w http.ResponseWriter, r *http.Request) {
	result, err := h.services.Operator.GetProposal(r.PathValue("proposal_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, result.TenantID) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listExpansions(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	if !scopeListQueryToAdmin(w, r, &query) {
		return
	}
	result, err := h.services.Operator.ListExpansions(query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listAgents(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	if !scopeListQueryToAdmin(w, r, &query) {
		return
	}
	result, err := h.services.Operator.ListAgents(query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listToolContracts(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	result, err := h.services.Operator.ListToolContracts(query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listProjections(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	if !scopeListQueryToAdmin(w, r, &query) {
		return
	}
	result, err := h.services.Operator.ListProjections(query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getContainmentRule(w http.ResponseWriter, r *http.Request) {
	result, err := h.services.Operator.GetContainmentRule(r.PathValue("rule_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, result.TenantID) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeListQuery(w, r)
	if !ok {
		return
	}
	if !scopeListQueryToAdmin(w, r, &query) {
		return
	}
	result, err := h.services.Operator.ListEvents(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func scopeListQueryToAdmin(w http.ResponseWriter, r *http.Request, query *ListQuery) bool {
	return bindAdminTenant(w, r, &query.TenantID)
}

func decodeListQuery(w http.ResponseWriter, r *http.Request) (ListQuery, bool) {
	values := r.URL.Query()
	query := ListQuery{
		TenantID: strings.TrimSpace(values.Get("tenant_id")),
		State:    strings.TrimSpace(values.Get("state")),
		Status:   strings.TrimSpace(values.Get("status")),
		Type:     strings.TrimSpace(values.Get("type")),
		Query:    strings.TrimSpace(values.Get("q")),
		Cursor:   strings.TrimSpace(values.Get("cursor")),
	}
	if rawLimit := strings.TrimSpace(values.Get("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("limit must be a positive integer"))
			return ListQuery{}, false
		}
		query.Limit = min(limit, MaxCollectionLimit)
	}
	return query, true
}

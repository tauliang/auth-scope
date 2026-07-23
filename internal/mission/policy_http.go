package mission

import "net/http"

func (h *Handler) createPolicyBundle(w http.ResponseWriter, r *http.Request) {
	var req CreatePolicyBundleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Governance.CreatePolicyBundle(req, authenticatedAdmin(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listPolicyBundles(w http.ResponseWriter, r *http.Request) {
	bundles, err := h.services.Governance.ListPolicyBundles()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	tenant := authenticatedAdmin(r).TenantSubject
	if tenant != "" {
		filtered := bundles[:0]
		for _, bundle := range bundles {
			if bundle.TenantID == tenant {
				filtered = append(filtered, bundle)
			}
		}
		bundles = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy_bundles": bundles})
}

func (h *Handler) getPolicyBundle(w http.ResponseWriter, r *http.Request) {
	resp, err := h.services.Governance.GetPolicyBundle(r.PathValue("bundle_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, resp.TenantID) {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) activatePolicyBundle(w http.ResponseWriter, r *http.Request) {
	var req ActivatePolicyBundleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	bundle, err := h.services.Governance.GetPolicyBundle(r.PathValue("bundle_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, bundle.TenantID) {
		return
	}
	resp, err := h.services.Governance.ActivatePolicyBundle(r.PathValue("bundle_id"), req, authenticatedAdmin(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) simulatePolicyBundle(w http.ResponseWriter, r *http.Request) {
	var req SimulatePolicyBundleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	bundle, err := h.services.Governance.GetPolicyBundle(r.PathValue("bundle_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !ensureAdminTenantAccess(w, r, bundle.TenantID) {
		return
	}
	resp, err := h.services.Governance.SimulatePolicyBundle(r.PathValue("bundle_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

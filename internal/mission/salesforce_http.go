package mission

import "net/http"

func (h *Handler) createSalesforceOrgBinding(w http.ResponseWriter, r *http.Request) {
	var req CreateSalesforceOrgBindingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Salesforce.CreateSalesforceOrgBinding(req, authenticatedAdmin(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listSalesforceOrgBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.services.Salesforce.ListSalesforceOrgBindings()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]SalesforceOrgBinding, 0, len(bindings))
		for _, binding := range bindings {
			if binding.TenantID == tenant {
				filtered = append(filtered, binding)
			}
		}
		bindings = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"org_bindings": bindings})
}

func (h *Handler) authorizeSalesforceRecordAction(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeSalesforceRecordActionRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil && req.Evaluation != nil {
		if err := bindSalesforceActorIdentity(&req.Evaluation.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Salesforce.AuthorizeSalesforceRecordAction(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func bindSalesforceActorIdentity(actor *SalesforceActor, identity AgentIdentity) error {
	missionActorValue := missionActorFromSalesforce(*actor)
	if err := bindActorIdentity(&missionActorValue, identity); err != nil {
		return err
	}
	*actor = salesforceActor(missionActorValue)
	return nil
}

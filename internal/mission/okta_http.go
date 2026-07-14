package mission

import "net/http"

func (h *Handler) createOktaAppBinding(w http.ResponseWriter, r *http.Request) {
	var req CreateOktaAppBindingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Okta.CreateOktaAppBinding(req, authenticatedAdmin(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listOktaAppBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.services.Okta.ListOktaAppBindings()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]OktaAppBinding, 0, len(bindings))
		for _, binding := range bindings {
			if binding.TenantID == tenant {
				filtered = append(filtered, binding)
			}
		}
		bindings = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"app_bindings": bindings})
}

func (h *Handler) resolveOktaAuthorityContext(w http.ResponseWriter, r *http.Request) {
	var req ResolveOktaAuthorityContextRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil && req.Evaluation != nil {
		if err := bindOktaActorIdentity(&req.Evaluation.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Okta.ResolveOktaAuthorityContext(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func bindOktaActorIdentity(actor *OktaActor, identity AgentIdentity) error {
	missionActorValue := missionActorFromOkta(*actor)
	if err := bindActorIdentity(&missionActorValue, identity); err != nil {
		return err
	}
	*actor = oktaActor(missionActorValue)
	return nil
}

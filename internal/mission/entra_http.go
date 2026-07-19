package mission

import "net/http"

func (h *Handler) createEntraAppRegistration(w http.ResponseWriter, r *http.Request) {
	var req CreateEntraAppRegistrationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Entra.CreateEntraAppRegistration(req, authenticatedAdmin(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listEntraAppRegistrations(w http.ResponseWriter, r *http.Request) {
	registrations, err := h.services.Entra.ListEntraAppRegistrations()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]EntraAppRegistration, 0, len(registrations))
		for _, reg := range registrations {
			if reg.TenantID == tenant {
				filtered = append(filtered, reg)
			}
		}
		registrations = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"app_registrations": registrations})
}

func (h *Handler) resolveEntraAuthorityContext(w http.ResponseWriter, r *http.Request) {
	var req ResolveEntraAuthorityContextRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil && req.Evaluation != nil {
		if err := bindEntraActorIdentity(&req.Evaluation.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Entra.ResolveEntraAuthorityContext(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func bindEntraActorIdentity(actor *EntraActor, identity AgentIdentity) error {
	missionActorValue := missionActorFromEntra(*actor)
	if err := bindActorIdentity(&missionActorValue, identity); err != nil {
		return err
	}
	*actor = entraActor(missionActorValue)
	return nil
}

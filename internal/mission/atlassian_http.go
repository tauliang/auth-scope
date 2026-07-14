package mission

import "net/http"

func (h *Handler) createAtlassianSiteBinding(w http.ResponseWriter, r *http.Request) {
	var req CreateAtlassianSiteBindingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Atlassian.CreateAtlassianSiteBinding(req, authenticatedAdmin(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listAtlassianSiteBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.services.Atlassian.ListAtlassianSiteBindings()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]AtlassianSiteBinding, 0, len(bindings))
		for _, binding := range bindings {
			if binding.TenantID == tenant {
				filtered = append(filtered, binding)
			}
		}
		bindings = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"site_bindings": bindings})
}

func (h *Handler) authorizeAtlassianJiraIssueAction(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeAtlassianJiraIssueActionRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil && req.Evaluation != nil {
		if err := bindAtlassianActorIdentity(&req.Evaluation.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Atlassian.AuthorizeAtlassianJiraIssueAction(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) authorizeAtlassianConfluencePageAction(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeAtlassianConfluencePageActionRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil && req.Evaluation != nil {
		if err := bindAtlassianActorIdentity(&req.Evaluation.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Atlassian.AuthorizeAtlassianConfluencePageAction(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func bindAtlassianActorIdentity(actor *AtlassianActor, identity AgentIdentity) error {
	missionActorValue := missionActorFromAtlassian(*actor)
	if err := bindActorIdentity(&missionActorValue, identity); err != nil {
		return err
	}
	*actor = atlassianActor(missionActorValue)
	return nil
}

package mission

import "net/http"

func (h *Handler) createSlackWorkspaceBinding(w http.ResponseWriter, r *http.Request) {
	var req CreateSlackWorkspaceBindingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.Slack.CreateSlackWorkspaceBinding(req, authenticatedAdmin(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listSlackWorkspaceBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.services.Slack.ListSlackWorkspaceBindings()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]SlackWorkspaceBinding, 0, len(bindings))
		for _, binding := range bindings {
			if binding.TenantID == tenant {
				filtered = append(filtered, binding)
			}
		}
		bindings = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace_bindings": bindings})
}

func (h *Handler) authorizeSlackMessageAction(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeSlackMessageActionRequest
	body, ok := decodeJSONBody(w, r, &req)
	if !ok {
		return
	}
	identity, ok := h.verifiedAgentIdentity(w, r, body)
	if !ok {
		return
	}
	if identity != nil && req.Evaluation != nil {
		if err := bindSlackActorIdentity(&req.Evaluation.Actor, *identity); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
	}
	resp, err := h.services.Slack.AuthorizeSlackMessageAction(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func bindSlackActorIdentity(actor *SlackActor, identity AgentIdentity) error {
	missionActorValue := missionActorFromSlack(*actor)
	if err := bindActorIdentity(&missionActorValue, identity); err != nil {
		return err
	}
	*actor = slackActor(missionActorValue)
	return nil
}

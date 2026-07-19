package mission

import (
	"io"
	"net/http"
	"strings"
)

func (h *Handler) createGitHubRepositoryBinding(w http.ResponseWriter, r *http.Request) {
	var req CreateGitHubRepositoryBindingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !bindAdminTenant(w, r, &req.TenantID) {
		return
	}
	resp, err := h.services.GitHub.CreateGitHubRepositoryBinding(req, authenticatedAdmin(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listGitHubRepositoryBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.services.GitHub.ListGitHubRepositoryBindings()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if tenant := authenticatedAdmin(r).TenantSubject; tenant != "" {
		filtered := make([]GitHubRepositoryBinding, 0, len(bindings))
		for _, binding := range bindings {
			if binding.TenantID == tenant {
				filtered = append(filtered, binding)
			}
		}
		bindings = filtered
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": bindings})
}

func (h *Handler) githubWebhook(w http.ResponseWriter, r *http.Request) {
	body, ok := readRawRequestBody(w, r)
	if !ok {
		return
	}
	resp, err := h.services.GitHub.IngestGitHubWebhook(IngestGitHubWebhookRequest{
		Event:      r.Header.Get("X-GitHub-Event"),
		DeliveryID: r.Header.Get("X-GitHub-Delivery"),
		Signature:  r.Header.Get("X-Hub-Signature-256"),
		Body:       body,
	})
	if err != nil {
		if strings.Contains(err.Error(), "signature") || strings.Contains(err.Error(), "secret") {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) githubCheckRunPlan(w http.ResponseWriter, r *http.Request) {
	var req GitHubCheckRunPlanRequest
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
	resp, err := h.services.GitHub.PlanGitHubCheckRun(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func readRawRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONRequestBodyBytes))
	_ = r.Body.Close()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return nil, false
	}
	return body, true
}

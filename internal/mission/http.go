package mission

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /.well-known/mission-authority", h.discovery)
	mux.HandleFunc("POST /v1/mission-proposals", h.createProposal)
	mux.HandleFunc("POST /v1/mission-proposals/{proposal_id}/approve", h.approveProposal)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/evaluate", h.evaluate)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/resume", h.resume)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/delegate", h.delegate)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/revoke", h.revoke)
	mux.HandleFunc("POST /v1/missions/{mission_ref}/complete", h.complete)
	mux.HandleFunc("GET /v1/missions/{mission_ref}/introspect", h.introspect)
	mux.HandleFunc("GET /v1/events", h.events)
	return requestID(mux)
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
			"mission_proposals":  true,
			"delegation":         true,
			"expansion_requests": false,
			"projection_types":   []string{"oauth_claims", "authzen_context", "mcp_context"},
			"revocation_events":  []string{"in_memory_events"},
		},
	})
}

func (h *Handler) createProposal(w http.ResponseWriter, r *http.Request) {
	var req CreateProposalRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.service.CreateProposal(req)
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
	resp, err := h.service.ApproveProposal(r.PathValue("proposal_id"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) evaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.service.Evaluate(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) resume(w http.ResponseWriter, r *http.Request) {
	var req ResumeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.service.Resume(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) delegate(w http.ResponseWriter, r *http.Request) {
	var req DelegationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := h.service.Delegate(r.PathValue("mission_ref"), req)
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
	resp, err := h.service.Revoke(r.PathValue("mission_ref"), req)
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
	resp, err := h.service.Complete(r.PathValue("mission_ref"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) introspect(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Introspect(r.PathValue("mission_ref"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) events(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"events": h.service.Events()})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	default:
		writeError(w, http.StatusBadRequest, err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": strings.TrimSpace(err.Error()),
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-request-id") == "" {
			r.Header.Set("x-request-id", newID("req"))
		}
		next.ServeHTTP(w, r)
	})
}

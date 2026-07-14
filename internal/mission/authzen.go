package mission

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type AuthZENEntity struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Properties map[string]any `json:"properties,omitempty"`
}

type AuthZENEvaluationRequest struct {
	Subject  AuthZENEntity  `json:"subject"`
	Action   AuthZENEntity  `json:"action"`
	Resource AuthZENEntity  `json:"resource"`
	Context  map[string]any `json:"context,omitempty"`
}

type AuthZENEvaluationResponse struct {
	Decision bool           `json:"decision"`
	Context  map[string]any `json:"context,omitempty"`
}

type AuthZENEvaluationsRequest struct {
	Evaluations []AuthZENEvaluationRequest `json:"evaluations"`
}

type AuthZENEvaluationsResponse struct {
	Evaluations []AuthZENEvaluationResponse `json:"evaluations"`
}

func (s *Service) EvaluateAuthZEN(req AuthZENEvaluationRequest) (AuthZENEvaluationResponse, error) {
	missionRef := authZENMissionRef(req)
	if missionRef == "" {
		return AuthZENEvaluationResponse{}, fmt.Errorf("context.mission_ref is required")
	}
	evaluateReq := EvaluateRequest{
		MissionVersionSeen: authZENInt(req.Context, "mission_version_seen"),
		Actor: Actor{
			AgentInstanceID: authZENString(req.Subject.Properties, "agent_instance_id"),
			ClientID:        authZENString(req.Subject.Properties, "client_id"),
			KeyThumbprint:   authZENString(req.Subject.Properties, "key_thumbprint"),
		},
		Action: Action{
			Type: authZENString(req.Action.Properties, "type"),
			Name: authZENString(req.Action.Properties, "name"),
			Resource: ActionResource{
				Type: req.Resource.Type,
				ID:   req.Resource.ID,
			},
			Operation: authZENOperation(req),
		},
		Context: req.Context,
	}
	if evaluateReq.Actor.AgentInstanceID == "" {
		evaluateReq.Actor.AgentInstanceID = req.Subject.ID
	}
	if evaluateReq.Action.Type == "" {
		evaluateReq.Action.Type = req.Action.Type
	}
	if evaluateReq.Action.Name == "" {
		evaluateReq.Action.Name = req.Action.ID
	}

	resp, err := s.Evaluate(missionRef, evaluateReq)
	if err != nil {
		return AuthZENEvaluationResponse{}, err
	}
	return AuthZENEvaluationResponse{
		Decision: resp.Decision == DecisionAllow || resp.Decision == DecisionAllowWithConstraint,
		Context: map[string]any{
			"decision":          resp.Decision,
			"mission_ref":       resp.MissionRef,
			"mission_version":   resp.MissionVersion,
			"reason_codes":      resp.ReasonCodes,
			"human_reason":      resp.HumanReason,
			"constraints":       resp.Constraints,
			"escalation":        resp.Escalation,
			"decision_artifact": resp.DecisionArtifact,
		},
	}, nil
}

func (s *Service) EvaluateAuthZENBatch(req AuthZENEvaluationsRequest) (AuthZENEvaluationsResponse, error) {
	responses := make([]AuthZENEvaluationResponse, 0, len(req.Evaluations))
	for _, evaluation := range req.Evaluations {
		resp, err := s.EvaluateAuthZEN(evaluation)
		if err != nil {
			return AuthZENEvaluationsResponse{}, err
		}
		responses = append(responses, resp)
	}
	return AuthZENEvaluationsResponse{Evaluations: responses}, nil
}

func authZENMissionRef(req AuthZENEvaluationRequest) string {
	if value := authZENString(req.Context, "mission_ref"); value != "" {
		return value
	}
	if value := authZENString(req.Resource.Properties, "mission_ref"); value != "" {
		return value
	}
	return authZENString(req.Subject.Properties, "mission_ref")
}

func authZENOperation(req AuthZENEvaluationRequest) string {
	if value := authZENString(req.Action.Properties, "operation"); value != "" {
		return value
	}
	return req.Action.ID
}

func authZENString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func authZENInt(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := strconv.Atoi(typed.String())
		return parsed
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}

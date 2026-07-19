package mission

import (
	"errors"

	slackint "github.com/tauliang/auth-scope/internal/mission/integrations/slack"
)

func (s *Service) CreateSlackWorkspaceBinding(req CreateSlackWorkspaceBindingRequest, actor Principal) (SlackWorkspaceBinding, error) {
	return s.slackIntegration().CreateWorkspaceBinding(req, slackPrincipal(actor))
}

func (s *Service) ListSlackWorkspaceBindings() ([]SlackWorkspaceBinding, error) {
	return s.slackIntegration().ListWorkspaceBindings()
}

func (s *Service) AuthorizeSlackMessageAction(req AuthorizeSlackMessageActionRequest) (SlackMessageAuthorizationResponse, error) {
	return s.slackIntegration().AuthorizeMessageAction(req)
}

func (s *Service) slackIntegration() *slackint.Service {
	return slackint.NewService(slackint.Config{
		Store:     slackStoreAdapter{store: s.slack},
		Evaluator: slackEvaluator{service: s},
		Events:    slackEventSink{events: s.events},
		Clock:     s.clock,
		NewID:     newID,
		IsConflict: func(err error) bool {
			return errors.Is(err, ErrConflict)
		},
	})
}

type slackStoreAdapter struct {
	store SlackStore
}

func (a slackStoreAdapter) SaveWorkspaceBinding(binding slackint.WorkspaceBinding) error {
	return a.store.SaveSlackWorkspaceBinding(binding)
}

func (a slackStoreAdapter) GetWorkspaceBinding(id string) (slackint.WorkspaceBinding, error) {
	return a.store.GetSlackWorkspaceBinding(id)
}

func (a slackStoreAdapter) UpdateWorkspaceBinding(binding slackint.WorkspaceBinding) error {
	return a.store.UpdateSlackWorkspaceBinding(binding)
}

func (a slackStoreAdapter) ListWorkspaceBindings() ([]slackint.WorkspaceBinding, error) {
	return a.store.ListSlackWorkspaceBindings()
}

type slackEvaluator struct {
	service *Service
}

func (e slackEvaluator) Evaluate(missionRef string, req slackint.EvaluationRequest, context map[string]any) (slackint.EvaluationResponse, error) {
	missionActor := missionActorFromSlack(req.Actor)
	missionAction := missionActionFromSlack(req.Action)
	resp, err := e.service.EvaluateMissionAction(missionRef, missionActor, missionAction, context)
	if err != nil {
		return slackint.EvaluationResponse{}, err
	}
	return slackEvaluationResponse(resp), nil
}

type slackEventSink struct {
	events EventSink
}

func (s slackEventSink) AppendEvent(evt slackint.Event) error {
	if s.events == nil {
		return nil
	}
	return s.events.AppendEvent(evt.Type, evt.Payload)
}

func slackPrincipal(actor Principal) slackint.Principal {
	return slackint.Principal{
		UserID: actor.Subject,
		Email:  actor.Email,
	}
}

func missionActorFromSlack(actor slackint.Actor) Actor {
	return Actor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func slackActor(actor Actor) SlackActor {
	return SlackActor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func missionActionFromSlack(action slackint.MessageAction) Action {
	return Action{
		Type:      action.Type,
		Name:      action.Name,
		Resource:  ResourceRef{Type: action.Resource.Type, ID: action.Resource.ID},
		Operation: action.Operation,
	}
}

func slackEvaluationResponse(resp EvaluationResponse) slackint.EvaluationResponse {
	return slackint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}
}

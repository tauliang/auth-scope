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

func (e slackEvaluator) Evaluate(req slackint.EvaluationRequest) (slackint.EvaluationResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              missionActorFromSlack(req.Actor),
		Action: Action{
			Type: req.Action.Type,
			Name: req.Action.Name,
			Resource: ActionResource{
				Type: req.Action.Resource.Type,
				ID:   req.Action.Resource.ID,
			},
			Operation: req.Action.Operation,
		},
		Context: req.Context,
	})
	if err != nil {
		return slackint.EvaluationResponse{}, err
	}
	return slackint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}, nil
}

type slackEventSink struct {
	events EventStore
}

func (s slackEventSink) AppendEvent(event slackint.Event) error {
	if s.events == nil {
		return nil
	}
	return s.events.AppendEvent(Event{
		EventID:       event.EventID,
		MissionRef:    event.MissionRef,
		TenantID:      event.TenantID,
		Type:          event.Type,
		Actor:         event.Actor,
		Payload:       event.Payload,
		VersionBefore: event.VersionBefore,
		VersionAfter:  event.VersionAfter,
		OccurredAt:    event.OccurredAt,
	})
}

func slackPrincipal(actor Principal) slackint.Principal {
	return slackint.Principal{
		UserID: actor.Subject,
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

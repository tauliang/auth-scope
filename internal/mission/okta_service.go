package mission

import (
	"errors"

	oktaint "github.com/tauliang/auth-scope/internal/mission/integrations/okta"
)

func (s *Service) CreateOktaAppBinding(req CreateOktaAppBindingRequest, actor Principal) (OktaAppBinding, error) {
	return s.oktaIntegration().CreateAppBinding(req, oktaPrincipal(actor))
}

func (s *Service) ListOktaAppBindings() ([]OktaAppBinding, error) {
	return s.oktaIntegration().ListAppBindings()
}

func (s *Service) ResolveOktaAuthorityContext(req ResolveOktaAuthorityContextRequest) (OktaAuthorityContextResponse, error) {
	return s.oktaIntegration().ResolveAuthorityContext(req)
}

func (s *Service) oktaIntegration() *oktaint.Service {
	return oktaint.NewService(oktaint.Config{
		Store:     oktaStoreAdapter{store: s.okta},
		Evaluator: oktaEvaluator{service: s},
		Events:    oktaEventSink{events: s.events},
		Clock:     s.clock,
		NewID:     newID,
		IsConflict: func(err error) bool {
			return errors.Is(err, ErrConflict)
		},
	})
}

type oktaStoreAdapter struct {
	store OktaStore
}

func (a oktaStoreAdapter) SaveAppBinding(binding oktaint.AppBinding) error {
	return a.store.SaveOktaAppBinding(binding)
}

func (a oktaStoreAdapter) GetAppBinding(id string) (oktaint.AppBinding, error) {
	return a.store.GetOktaAppBinding(id)
}

func (a oktaStoreAdapter) UpdateAppBinding(binding oktaint.AppBinding) error {
	return a.store.UpdateOktaAppBinding(binding)
}

func (a oktaStoreAdapter) ListAppBindings() ([]oktaint.AppBinding, error) {
	return a.store.ListOktaAppBindings()
}

type oktaEvaluator struct {
	service *Service
}

func (e oktaEvaluator) Evaluate(req oktaint.EvaluationRequest) (oktaint.EvaluationResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              missionActorFromOkta(req.Actor),
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
		return oktaint.EvaluationResponse{}, err
	}
	return oktaint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}, nil
}

type oktaEventSink struct {
	events EventStore
}

func (s oktaEventSink) AppendEvent(event oktaint.Event) error {
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

func oktaPrincipal(principal Principal) OktaPrincipal {
	return OktaPrincipal{
		Subject: principal.Subject,
		Issuer:  principal.Issuer,
	}
}

func missionActorFromOkta(actor OktaActor) Actor {
	return Actor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func oktaActor(actor Actor) OktaActor {
	return OktaActor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

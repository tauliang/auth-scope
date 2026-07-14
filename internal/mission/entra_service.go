package mission

import (
	"errors"

	entraint "github.com/tauliang/auth-scope/internal/mission/integrations/entra"
)

func (s *Service) CreateEntraAppRegistration(req CreateEntraAppRegistrationRequest, actor Principal) (EntraAppRegistration, error) {
	return s.entraIntegration().CreateAppRegistration(req, entraPrincipal(actor))
}

func (s *Service) ListEntraAppRegistrations() ([]EntraAppRegistration, error) {
	return s.entraIntegration().ListAppRegistrations()
}

func (s *Service) ResolveEntraAuthorityContext(req ResolveEntraAuthorityContextRequest) (EntraAuthorityContextResponse, error) {
	return s.entraIntegration().ResolveAuthorityContext(req)
}

func (s *Service) entraIntegration() *entraint.Service {
	return entraint.NewService(entraint.Config{
		Store:     entraStoreAdapter{store: s.entra},
		Evaluator: entraEvaluator{service: s},
		Events:    entraEventSink{events: s.events},
		Clock:     s.clock,
		NewID:     newID,
		IsConflict: func(err error) bool {
			return errors.Is(err, ErrConflict)
		},
	})
}

type entraStoreAdapter struct {
	store EntraStore
}

func (a entraStoreAdapter) SaveAppRegistration(reg entraint.AppRegistration) error {
	return a.store.SaveEntraAppRegistration(reg)
}

func (a entraStoreAdapter) GetAppRegistration(id string) (entraint.AppRegistration, error) {
	return a.store.GetEntraAppRegistration(id)
}

func (a entraStoreAdapter) UpdateAppRegistration(reg entraint.AppRegistration) error {
	return a.store.UpdateEntraAppRegistration(reg)
}

func (a entraStoreAdapter) ListAppRegistrations() ([]entraint.AppRegistration, error) {
	return a.store.ListEntraAppRegistrations()
}

type entraEvaluator struct {
	service *Service
}

func (e entraEvaluator) Evaluate(req entraint.EvaluationRequest) (entraint.EvaluationResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              missionActorFromEntra(req.Actor),
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
		return entraint.EvaluationResponse{}, err
	}
	return entraint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}, nil
}

type entraEventSink struct {
	events EventStore
}

func (s entraEventSink) AppendEvent(event entraint.Event) error {
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

func entraPrincipal(actor Principal) entraint.Principal {
	return entraint.Principal{
		Subject: actor.Subject,
		Issuer:  actor.Issuer,
	}
}

func missionActorFromEntra(actor entraint.Actor) Actor {
	return Actor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func entraActor(actor Actor) EntraActor {
	return EntraActor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

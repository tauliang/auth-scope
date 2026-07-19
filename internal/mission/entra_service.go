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

func (e entraEvaluator) Evaluate(missionRef string, req entraint.EvaluationRequest, context map[string]any) (entraint.EvaluationResponse, error) {
	missionActor := missionActorFromEntra(req.Actor)
	missionAction := missionActionFromEntra(req.Action)
	resp, err := e.service.EvaluateMissionAction(missionRef, missionActor, missionAction, context)
	if err != nil {
		return entraint.EvaluationResponse{}, err
	}
	return entraEvaluationResponse(resp), nil
}

type entraEventSink struct {
	events EventSink
}

func (s entraEventSink) AppendEvent(evt entraint.Event) error {
	if s.events == nil {
		return nil
	}
	return s.events.AppendEvent(evt.Type, evt.Payload)
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

func missionActionFromEntra(action entraint.EvaluationAction) Action {
	return Action{
		Type:      action.Type,
		Name:      action.Name,
		Resource:  ResourceRef{Type: action.Resource.Type, ID: action.Resource.ID},
		Operation: action.Operation,
	}
}

func entraEvaluationResponse(resp EvaluationResponse) entraint.EvaluationResponse {
	return entraint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}
}

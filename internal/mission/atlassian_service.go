package mission

import (
	"errors"

	atlassianint "github.com/tauliang/auth-scope/internal/mission/integrations/atlassian"
)

func (s *Service) CreateAtlassianSiteBinding(req CreateAtlassianSiteBindingRequest, actor Principal) (AtlassianSiteBinding, error) {
	return s.atlassianIntegration().CreateSiteBinding(req, atlassianPrincipal(actor))
}

func (s *Service) ListAtlassianSiteBindings() ([]AtlassianSiteBinding, error) {
	return s.atlassianIntegration().ListSiteBindings()
}

func (s *Service) AuthorizeAtlassianJiraIssueAction(req AuthorizeAtlassianJiraIssueActionRequest) (AtlassianActionAuthorizationResponse, error) {
	return s.atlassianIntegration().AuthorizeJiraIssueAction(req)
}

func (s *Service) AuthorizeAtlassianConfluencePageAction(req AuthorizeAtlassianConfluencePageActionRequest) (AtlassianActionAuthorizationResponse, error) {
	return s.atlassianIntegration().AuthorizeConfluencePageAction(req)
}

func (s *Service) atlassianIntegration() *atlassianint.Service {
	return atlassianint.NewService(atlassianint.Config{
		Store:     atlassianStoreAdapter{store: s.atlassian},
		Evaluator: atlassianEvaluator{service: s},
		Events:    atlassianEventSink{events: s.events},
		Clock:     s.clock,
		NewID:     newID,
		IsConflict: func(err error) bool {
			return errors.Is(err, ErrConflict)
		},
	})
}

type atlassianStoreAdapter struct {
	store AtlassianStore
}

func (a atlassianStoreAdapter) SaveSiteBinding(binding atlassianint.SiteBinding) error {
	return a.store.SaveAtlassianSiteBinding(binding)
}

func (a atlassianStoreAdapter) GetSiteBinding(id string) (atlassianint.SiteBinding, error) {
	return a.store.GetAtlassianSiteBinding(id)
}

func (a atlassianStoreAdapter) UpdateSiteBinding(binding atlassianint.SiteBinding) error {
	return a.store.UpdateAtlassianSiteBinding(binding)
}

func (a atlassianStoreAdapter) ListSiteBindings() ([]atlassianint.SiteBinding, error) {
	return a.store.ListAtlassianSiteBindings()
}

type atlassianEvaluator struct {
	service *Service
}

func (e atlassianEvaluator) Evaluate(req atlassianint.EvaluationRequest) (atlassianint.EvaluationResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              missionActorFromAtlassian(req.Actor),
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
		return atlassianint.EvaluationResponse{}, err
	}
	return atlassianint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}, nil
}

type atlassianEventSink struct {
	events EventStore
}

func (s atlassianEventSink) AppendEvent(event atlassianint.Event) error {
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

func atlassianPrincipal(principal Principal) AtlassianPrincipal {
	return AtlassianPrincipal{
		Subject: principal.Subject,
		Issuer:  principal.Issuer,
	}
}

func missionActorFromAtlassian(actor AtlassianActor) Actor {
	return Actor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func atlassianActor(actor Actor) AtlassianActor {
	return AtlassianActor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

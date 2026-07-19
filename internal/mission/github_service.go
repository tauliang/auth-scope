package mission

import (
	"errors"

	githubint "github.com/tauliang/auth-scope/internal/mission/integrations/github"
)

func (s *Service) CreateGitHubRepositoryBinding(req CreateGitHubRepositoryBindingRequest, actor Principal) (GitHubRepositoryBinding, error) {
	return s.githubIntegration().CreateRepositoryBinding(req, githubPrincipal(actor))
}

func (s *Service) ListGitHubRepositoryBindings() ([]GitHubRepositoryBinding, error) {
	return s.githubIntegration().ListRepositoryBindings()
}

func (s *Service) IngestGitHubWebhook(req IngestGitHubWebhookRequest) (GitHubWebhookResponse, error) {
	return s.githubIntegration().IngestWebhook(req)
}

func (s *Service) PlanGitHubCheckRun(req GitHubCheckRunPlanRequest) (GitHubCheckRunPlanResponse, error) {
	return s.githubIntegration().PlanCheckRun(req)
}

func (s *Service) githubIntegration() *githubint.Service {
	return githubint.NewService(githubint.Config{
		Store:         githubStoreAdapter{store: s.github},
		Evaluator:     githubEvaluator{service: s},
		Events:        githubEventSink{events: s.events},
		Clock:         s.clock,
		WebhookSecret: s.githubWebhookSecret,
		NewID:         newID,
		IsConflict: func(err error) bool {
			return errors.Is(err, ErrConflict)
		},
	})
}

type githubStoreAdapter struct {
	store GitHubStore
}

func (a githubStoreAdapter) SaveRepositoryBinding(binding githubint.RepositoryBinding) error {
	return a.store.SaveGitHubRepositoryBinding(binding)
}

func (a githubStoreAdapter) GetRepositoryBinding(id string) (githubint.RepositoryBinding, error) {
	return a.store.GetGitHubRepositoryBinding(id)
}

func (a githubStoreAdapter) UpdateRepositoryBinding(binding githubint.RepositoryBinding) error {
	return a.store.UpdateGitHubRepositoryBinding(binding)
}

func (a githubStoreAdapter) ListRepositoryBindings() ([]githubint.RepositoryBinding, error) {
	return a.store.ListGitHubRepositoryBindings()
}

func (a githubStoreAdapter) SaveWebhookDelivery(delivery githubint.WebhookDelivery) error {
	return a.store.SaveGitHubWebhookDelivery(delivery)
}

func (a githubStoreAdapter) GetWebhookDelivery(id string) (githubint.WebhookDelivery, error) {
	return a.store.GetGitHubWebhookDelivery(id)
}

type githubEvaluator struct {
	service *Service
}

func (e githubEvaluator) Evaluate(req githubint.EvaluationRequest) (githubint.EvaluationResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              missionActor(req.Actor),
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
		return githubint.EvaluationResponse{}, err
	}
	return githubint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}, nil
}

type githubEventSink struct {
	events EventStore
}

func (s githubEventSink) AppendEvent(event githubint.Event) error {
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

func githubPrincipal(principal Principal) GitHubPrincipal {
	return GitHubPrincipal{
		Subject: principal.Subject,
		Issuer:  principal.Issuer,
	}
}

func missionActor(actor GitHubActor) Actor {
	return Actor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func githubActor(actor Actor) GitHubActor {
	return GitHubActor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

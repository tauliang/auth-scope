package mission

import (
	"errors"

	salesforceint "github.com/tauliang/auth-scope/internal/mission/integrations/salesforce"
)

func (s *Service) CreateSalesforceOrgBinding(req CreateSalesforceOrgBindingRequest, actor Principal) (SalesforceOrgBinding, error) {
	return s.salesforceIntegration().CreateOrgBinding(req, salesforcePrincipal(actor))
}

func (s *Service) ListSalesforceOrgBindings() ([]SalesforceOrgBinding, error) {
	return s.salesforceIntegration().ListOrgBindings()
}

func (s *Service) AuthorizeSalesforceRecordAction(req AuthorizeSalesforceRecordActionRequest) (SalesforceRecordActionAuthorizationResponse, error) {
	return s.salesforceIntegration().AuthorizeRecordAction(req)
}

func (s *Service) salesforceIntegration() *salesforceint.Service {
	return salesforceint.NewService(s.salesforceConfig())
}

func (s *Service) salesforceConfig() salesforceint.Config {
	return salesforceint.Config{
		Store:     salesforceStoreAdapter{store: s.salesforce},
		Evaluator: salesforceEvaluator{service: s},
		Events:    salesforceEventSink{events: s.events},
		Clock:     s.clock,
		NewID:     newID,
		IsConflict: func(err error) bool {
			return errors.Is(err, ErrConflict)
		},
	}
}

type salesforceStoreAdapter struct {
	store SalesforceStore
}

func (a salesforceStoreAdapter) SaveOrgBinding(binding salesforceint.OrgBinding) error {
	return a.store.SaveSalesforceOrgBinding(binding)
}

func (a salesforceStoreAdapter) GetOrgBinding(id string) (salesforceint.OrgBinding, error) {
	return a.store.GetSalesforceOrgBinding(id)
}

func (a salesforceStoreAdapter) UpdateOrgBinding(binding salesforceint.OrgBinding) error {
	return a.store.UpdateSalesforceOrgBinding(binding)
}

func (a salesforceStoreAdapter) ListOrgBindings() ([]salesforceint.OrgBinding, error) {
	return a.store.ListSalesforceOrgBindings()
}

type salesforceEvaluator struct {
	service *Service
}

func (e salesforceEvaluator) Evaluate(req salesforceint.EvaluationRequest) (salesforceint.EvaluationResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              missionActorFromSalesforce(req.Actor),
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
		return salesforceint.EvaluationResponse{}, err
	}
	return salesforceint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}, nil
}

type salesforceEventSink struct {
	events EventStore
}

func (s salesforceEventSink) AppendEvent(event salesforceint.Event) error {
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

func salesforcePrincipal(principal Principal) SalesforcePrincipal {
	return SalesforcePrincipal{
		Subject: principal.Subject,
		Issuer:  principal.Issuer,
	}
}

func missionActorFromSalesforce(actor SalesforceActor) Actor {
	return Actor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func salesforceActor(actor Actor) SalesforceActor {
	return SalesforceActor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

package mission

import (
	"context"

	snint "github.com/tauliang/auth-scope/internal/mission/integrations/servicenow"
)

func (s *Service) CreateServiceNowTicketBinding(ctx context.Context, req CreateServiceNowTicketBindingRequest, actor Principal) (ServiceNowTicketBinding, error) {
	resp, err := s.servicenowIntegration().CreateTicketBinding(ctx, &snint.CreateTicketBindingRequest{
		TenantID:        req.TenantID,
		InstanceURL:     req.InstanceURL,
		ServiceNowSysID: req.ServiceNowSysID,
		State:           req.State,
		MissionRef:      req.MissionRef,
		AssignmentGroup: req.AssignmentGroup,
		CallerID:        req.CallerID,
		RequiredGroups:  req.RequiredGroups,
		AdminGroups:     req.AdminGroups,
		AllowedSubjects: req.AllowedSubjects,
		GroupClaim:      req.GroupClaim,
		SubjectClaim:    req.SubjectClaim,
		GroupMatchMode:  req.GroupMatchMode,
		Metadata:        req.Metadata,
	})
	if err != nil {
		return ServiceNowTicketBinding{}, err
	}
	return *resp, nil
}

func (s *Service) ListServiceNowTicketBindings(ctx context.Context) ([]ServiceNowTicketBinding, error) {
	resp, err := s.servicenowIntegration().ListTicketBindingsByMissionRef(ctx, "")
	if err != nil {
		return nil, err
	}
	result := make([]ServiceNowTicketBinding, len(resp))
	for i, b := range resp {
		result[i] = *b
	}
	return result, nil
}

func (s *Service) GetServiceNowTicketBinding(ctx context.Context, bindingID string) (ServiceNowTicketBinding, error) {
	resp, err := s.servicenowIntegration().GetTicketBinding(ctx, bindingID)
	if err != nil {
		return ServiceNowTicketBinding{}, err
	}
	return *resp, nil
}

func (s *Service) UpdateServiceNowTicketStatus(ctx context.Context, bindingID string, newState string) (ServiceNowTicketBinding, error) {
	resp, err := s.servicenowIntegration().UpdateTicketStatus(ctx, bindingID, newState)
	if err != nil {
		return ServiceNowTicketBinding{}, err
	}
	return *resp, nil
}

func (s *Service) ResolveServiceNowAuthorityContext(ctx context.Context, req ResolveServiceNowAuthorityContextRequest) (ServiceNowAuthorityContextResponse, error) {
	resp, err := s.servicenowIntegration().ResolveAuthorityContext(ctx, &snint.ResolveAuthorityContextRequest{
		MissionRef: req.MissionRef,
		TenantID:   req.TenantID,
		Issuer:     req.Issuer,
		ClientID:   req.ClientID,
		Subject:    req.Subject,
		Groups:     req.Groups,
		Claims:     req.Claims,
		Context:    req.Context,
		Evaluation: &snint.EvaluationRequest{
			MissionVersionSeen: req.Evaluation.MissionVersionSeen,
			Actor: snint.Actor{
				AgentInstanceID: req.Evaluation.Actor.AgentInstanceID,
				ClientID:        req.Evaluation.Actor.ClientID,
				KeyThumbprint:   req.Evaluation.Actor.KeyThumbprint,
			},
			Action: snint.EvaluationAction{
				Type: req.Evaluation.Action.Type,
				Name: req.Evaluation.Action.Name,
				Resource: snint.EvaluationActionResource{
					Type: req.Evaluation.Action.Resource.Type,
					ID:   req.Evaluation.Action.Resource.ID,
				},
				Operation: req.Evaluation.Action.Operation,
			},
		},
	})
	if err != nil {
		return ServiceNowAuthorityContextResponse{}, err
	}
	return *resp, nil
}

func (s *Service) DeleteServiceNowTicketBinding(ctx context.Context, bindingID string) error {
	return s.servicenowIntegration().DeleteTicketBinding(ctx, bindingID)
}

func (s *Service) servicenowIntegration() *snint.Service {
	return snint.NewService(&snint.Config{
		Store:     serviceNowStoreAdapter{store: s.servicenow},
		Evaluator: serviceNowEvaluator{service: s},
		EventSink: serviceNowEventSink{events: s.events},
		Clock:     s.clock,
	})
}

type serviceNowStoreAdapter struct {
	store ServiceNowStore
}

func (a serviceNowStoreAdapter) CreateTicketBinding(ctx context.Context, binding *snint.TicketBinding) error {
	return a.store.SaveServiceNowTicketBinding(*binding)
}

func (a serviceNowStoreAdapter) GetTicketBindingByID(ctx context.Context, bindingID string) (*snint.TicketBinding, error) {
	binding, err := a.store.GetServiceNowTicketBinding(bindingID)
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (a serviceNowStoreAdapter) GetTicketBindingByMissionRefAndSysID(ctx context.Context, missionRef, sysID string) (*snint.TicketBinding, error) {
	binding, err := a.store.GetServiceNowTicketBindingByMissionRefAndSysID(missionRef, sysID)
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (a serviceNowStoreAdapter) ListTicketBindingsByMissionRef(ctx context.Context, missionRef string) ([]*snint.TicketBinding, error) {
	bindings, err := a.store.ListServiceNowTicketBindings()
	if err != nil {
		return nil, err
	}
	result := make([]*snint.TicketBinding, len(bindings))
	for i, b := range bindings {
		result[i] = &b
	}
	return result, nil
}

func (a serviceNowStoreAdapter) UpdateTicketBinding(ctx context.Context, binding *snint.TicketBinding) error {
	return a.store.UpdateServiceNowTicketBinding(*binding)
}

func (a serviceNowStoreAdapter) DeleteTicketBinding(ctx context.Context, bindingID string) error {
	return a.store.DeleteServiceNowTicketBinding(bindingID)
}

type serviceNowEvaluator struct {
	service *Service
}

func (e serviceNowEvaluator) EvaluateAuthorityContext(ctx context.Context, req *snint.ResolveAuthorityContextRequest) (*snint.AuthorityContextResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.Evaluation.MissionVersionSeen,
		Actor:              missionActorFromServiceNow(req.Evaluation.Actor),
		Action: Action{
			Type:      req.Evaluation.Action.Type,
			Name:      req.Evaluation.Action.Name,
			Resource:  actionResourceFromServiceNow(req.Evaluation.Action.Resource),
			Operation: req.Evaluation.Action.Operation,
		},
		Context: req.Context,
	})
	if err != nil {
		return nil, err
	}

	return &snint.AuthorityContextResponse{
		Accepted:    resp.Decision == DecisionAllow,
		Status:      snint.ResolutionStatusAccepted,
		MissionRef:  resp.MissionRef,
		ReasonCodes: resp.ReasonCodes,
		HumanReason: resp.HumanReason,
	}, nil
}

type serviceNowEventSink struct {
	events EventStore
}

func (s serviceNowEventSink) PublishEvent(ctx context.Context, event *snint.Event) error {
	return s.events.AppendEvent(Event{
		EventID:       event.EventID,
		MissionRef:    event.MissionRef,
		TenantID:      event.TenantID,
		Type:          event.Type,
		Actor:         event.Actor,
		Payload:       event.Payload,
		VersionBefore: 0,
		VersionAfter:  0,
		OccurredAt:    event.OccurredAt,
	})
}

func serviceNowPrincipal(principal Principal) ServiceNowPrincipal {
	return ServiceNowPrincipal{
		Subject: principal.Subject,
		Issuer:  principal.Issuer,
	}
}

func missionActorFromServiceNow(actor ServiceNowActor) Actor {
	return Actor{
		AgentInstanceID: actor.AgentInstanceID,
		ClientID:        actor.ClientID,
		KeyThumbprint:   actor.KeyThumbprint,
	}
}

func actionResourceFromServiceNow(resource ServiceNowEvaluationActionResource) ActionResource {
	return ActionResource{
		Type: resource.Type,
		ID:   resource.ID,
	}
}

package mission

import (
	"context"

	snint "github.com/tauliang/auth-scope/internal/mission/integrations/servicenow"
)

func (s *Service) CreateServiceNowTicketBinding(ctx context.Context, req CreateServiceNowTicketBindingRequest, actor Principal) (ServiceNowTicketBinding, error) {
	resp, err := s.servicenowIntegration().CreateTicketBinding(snint.CreateTicketBindingRequest{
		TenantID:         req.TenantID,
		InstanceURL:      req.InstanceURL,
		ServiceNowSysID:  req.ServiceNowSysID,
		ServiceNowNumber: req.ServiceNowNumber,
		State:            req.State,
		MissionRef:       req.MissionRef,
		AssignmentGroup:  req.AssignmentGroup,
		CallerID:         req.CallerID,
		RequiredGroups:   req.RequiredGroups,
		AdminGroups:      req.AdminGroups,
		AllowedSubjects:  req.AllowedSubjects,
		GroupClaim:       req.GroupClaim,
		SubjectClaim:     req.SubjectClaim,
		GroupMatchMode:   req.GroupMatchMode,
		Metadata:         req.Metadata,
	}, serviceNowPrincipal(actor))
	if err != nil {
		return ServiceNowTicketBinding{}, err
	}
	return resp, nil
}

func (s *Service) ListServiceNowTicketBindings(ctx context.Context) ([]ServiceNowTicketBinding, error) {
	return s.servicenowIntegration().ListTicketBindings()
}

func (s *Service) GetServiceNowTicketBinding(ctx context.Context, bindingID string) (ServiceNowTicketBinding, error) {
	return s.servicenowIntegration().GetTicketBinding(bindingID)
}

func (s *Service) UpdateServiceNowTicketStatus(ctx context.Context, bindingID string, newState string) (ServiceNowTicketBinding, error) {
	return s.servicenowIntegration().UpdateTicketStatus(bindingID, newState)
}

func (s *Service) ResolveServiceNowAuthorityContext(ctx context.Context, req ResolveServiceNowAuthorityContextRequest) (ServiceNowAuthorityContextResponse, error) {
	resolveReq := snint.ResolveAuthorityContextRequest{
		MissionRef: req.MissionRef,
		TenantID:   req.TenantID,
		Issuer:     req.Issuer,
		ClientID:   req.ClientID,
		Subject:    req.Subject,
		Groups:     req.Groups,
		Claims:     req.Claims,
		Context:    req.Context,
	}
	if req.Evaluation != nil {
		evalReq := *req.Evaluation
		resolveReq.Evaluation = &evalReq
	}
	resp, err := s.servicenowIntegration().ResolveAuthorityContext(resolveReq)
	if err != nil {
		return ServiceNowAuthorityContextResponse{}, err
	}
	return resp, nil
}

func (s *Service) DeleteServiceNowTicketBinding(ctx context.Context, bindingID string) error {
	return s.servicenowIntegration().DeleteTicketBinding(bindingID)
}

func (s *Service) servicenowIntegration() *snint.Service {
	return snint.NewService(snint.Config{
		Store:     serviceNowStoreAdapter{store: s.servicenow},
		Evaluator: serviceNowEvaluator{service: s},
		Events:    serviceNowEventSink{events: s.events},
		Clock:     s.clock,
		NewID:     newID,
	})
}

type serviceNowStoreAdapter struct {
	store ServiceNowStore
}

func (a serviceNowStoreAdapter) SaveTicketBinding(binding snint.TicketBinding) error {
	return a.store.SaveServiceNowTicketBinding(binding)
}

func (a serviceNowStoreAdapter) GetTicketBinding(bindingID string) (snint.TicketBinding, error) {
	return a.store.GetServiceNowTicketBinding(bindingID)
}

func (a serviceNowStoreAdapter) ListTicketBindings() ([]snint.TicketBinding, error) {
	return a.store.ListServiceNowTicketBindings()
}

func (a serviceNowStoreAdapter) UpdateTicketBinding(binding snint.TicketBinding) error {
	return a.store.UpdateServiceNowTicketBinding(binding)
}

func (a serviceNowStoreAdapter) DeleteTicketBinding(bindingID string) error {
	return a.store.DeleteServiceNowTicketBinding(bindingID)
}

type serviceNowEvaluator struct {
	service *Service
}

func (e serviceNowEvaluator) Evaluate(req snint.EvaluationRequest) (snint.EvaluationResponse, error) {
	resp, err := e.service.Evaluate(req.MissionRef, EvaluateRequest{
		MissionVersionSeen: req.MissionVersionSeen,
		Actor:              missionActorFromServiceNow(req.Actor),
		Action: Action{
			Type:      req.Action.Type,
			Name:      req.Action.Name,
			Resource:  actionResourceFromServiceNow(req.Action.Resource),
			Operation: req.Action.Operation,
		},
		Context: req.Context,
	})
	if err != nil {
		return snint.EvaluationResponse{}, err
	}

	return snint.EvaluationResponse{
		Decision:         string(resp.Decision),
		MissionRef:       resp.MissionRef,
		MissionVersion:   resp.MissionVersion,
		ReasonCodes:      resp.ReasonCodes,
		HumanReason:      resp.HumanReason,
		DecisionArtifact: resp.DecisionArtifact,
		Constraints:      resp.Constraints,
	}, nil
}

type serviceNowEventSink struct {
	events EventStore
}

func (s serviceNowEventSink) AppendEvent(event snint.Event) error {
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

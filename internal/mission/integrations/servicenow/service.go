package servicenow

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrTicketBindingNotFound = errors.New("ticket binding not found")
	ErrInvalidState          = errors.New("invalid ticket state")
	ErrMissionRefRequired    = errors.New("mission reference is required")
	ErrInstanceURLRequired   = errors.New("instance URL is required")
)

type Store interface {
	CreateTicketBinding(ctx context.Context, binding *TicketBinding) error
	GetTicketBindingByID(ctx context.Context, bindingID string) (*TicketBinding, error)
	GetTicketBindingByMissionRefAndSysID(ctx context.Context, missionRef, sysID string) (*TicketBinding, error)
	ListTicketBindingsByMissionRef(ctx context.Context, missionRef string) ([]*TicketBinding, error)
	UpdateTicketBinding(ctx context.Context, binding *TicketBinding) error
	DeleteTicketBinding(ctx context.Context, bindingID string) error
}

type Evaluator interface {
	EvaluateAuthorityContext(ctx context.Context, req *ResolveAuthorityContextRequest) (*AuthorityContextResponse, error)
}

type EventSink interface {
	PublishEvent(ctx context.Context, event *Event) error
}

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type Config struct {
	Store      Store
	Evaluator  Evaluator
	EventSink  EventSink
	Clock      Clock
}

type Service struct {
	store      Store
	evaluator  Evaluator
	eventSink  EventSink
	clock      Clock
}

func NewService(cfg *Config) *Service {
	if cfg.Clock == nil {
		cfg.Clock = RealClock{}
	}
	return &Service{
		store:     cfg.Store,
		evaluator: cfg.Evaluator,
		eventSink: cfg.EventSink,
		clock:     cfg.Clock,
	}
}

func (s *Service) CreateTicketBinding(ctx context.Context, req *CreateTicketBindingRequest) (*TicketBinding, error) {
	if req.MissionRef == "" {
		return nil, ErrMissionRefRequired
	}
	if req.InstanceURL == "" {
		return nil, ErrInstanceURLRequired
	}

	binding := &TicketBinding{
		BindingID:          fmt.Sprintf("sn-%s", generateUniqueID()),
		TenantID:           req.TenantID,
		InstanceURL:        req.InstanceURL,
		ServiceNowSysID:    req.ServiceNowSysID,
		State:              req.State,
		MissionRef:         req.MissionRef,
		AssignmentGroup:    req.AssignmentGroup,
		CallerID:           req.CallerID,
		RequiredGroups:     req.RequiredGroups,
		AdminGroups:        req.AdminGroups,
		AllowedSubjects:    req.AllowedSubjects,
		GroupClaim:         req.GroupClaim,
		SubjectClaim:       req.SubjectClaim,
		GroupMatchMode:     req.GroupMatchMode,
		Status:             TicketBindingStatusActive,
		Metadata:           req.Metadata,
		CreatedAt:          s.clock.Now(),
	}

	if err := s.store.CreateTicketBinding(ctx, binding); err != nil {
		return nil, fmt.Errorf("failed to create ticket binding: %w", err)
	}

	if err := s.eventSink.PublishEvent(ctx, &Event{
		EventID:    fmt.Sprintf("evt-%s", generateUniqueID()),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "ticket_binding.created",
		Payload: map[string]any{
			"binding_id":      binding.BindingID,
			"service_now_sys_id": binding.ServiceNowSysID,
			"state":           binding.State,
		},
		OccurredAt: s.clock.Now(),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	return binding, nil
}

func (s *Service) GetTicketBinding(ctx context.Context, bindingID string) (*TicketBinding, error) {
	binding, err := s.store.GetTicketBindingByID(ctx, bindingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket binding: %w", err)
	}
	if binding == nil {
		return nil, ErrTicketBindingNotFound
	}
	return binding, nil
}

func (s *Service) ListTicketBindingsByMissionRef(ctx context.Context, missionRef string) ([]*TicketBinding, error) {
	bindings, err := s.store.ListTicketBindingsByMissionRef(ctx, missionRef)
	if err != nil {
		return nil, fmt.Errorf("failed to list ticket bindings: %w", err)
	}
	return bindings, nil
}

func (s *Service) UpdateTicketStatus(ctx context.Context, bindingID string, newState string) (*TicketBinding, error) {
	binding, err := s.GetTicketBinding(ctx, bindingID)
	if err != nil {
		return nil, err
	}

	validStates := map[string]bool{
		"new":       true,
		"in_progress": true,
		"on_hold":   true,
		"resolved":  true,
		"closed":    true,
	}

	if !validStates[newState] {
		return nil, ErrInvalidState
	}

	binding.State = newState
	binding.LastResolvedAt = s.clock.Now()

	if err := s.store.UpdateTicketBinding(ctx, binding); err != nil {
		return nil, fmt.Errorf("failed to update ticket status: %w", err)
	}

	if err := s.eventSink.PublishEvent(ctx, &Event{
		EventID:    fmt.Sprintf("evt-%s", generateUniqueID()),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "ticket_binding.status_updated",
		Payload: map[string]any{
			"binding_id": binding.BindingID,
			"state":      newState,
		},
		OccurredAt: s.clock.Now(),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	return binding, nil
}

func (s *Service) ResolveAuthorityContext(ctx context.Context, req *ResolveAuthorityContextRequest) (*AuthorityContextResponse, error) {
	resp, err := s.evaluator.EvaluateAuthorityContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate authority context: %w", err)
	}

	if err := s.eventSink.PublishEvent(ctx, &Event{
		EventID:    fmt.Sprintf("evt-%s", generateUniqueID()),
		MissionRef: req.MissionRef,
		TenantID:   req.TenantID,
		Type:       "authority_context.resolved",
		Payload: map[string]any{
			"subject":  req.Subject,
			"accepted": resp != nil && resp.Accepted,
		},
		OccurredAt: s.clock.Now(),
	}); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	return resp, nil
}

func (s *Service) DeleteTicketBinding(ctx context.Context, bindingID string) error {
	binding, err := s.GetTicketBinding(ctx, bindingID)
	if err != nil {
		return err
	}

	if err := s.store.DeleteTicketBinding(ctx, bindingID); err != nil {
		return fmt.Errorf("failed to delete ticket binding: %w", err)
	}

	if err := s.eventSink.PublishEvent(ctx, &Event{
		EventID:    fmt.Sprintf("evt-%s", generateUniqueID()),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "ticket_binding.deleted",
		Payload: map[string]any{
			"binding_id": bindingID,
		},
		OccurredAt: s.clock.Now(),
	}); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

func generateUniqueID() string {
	// TODO: Implement proper unique ID generation using UUID or similar
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

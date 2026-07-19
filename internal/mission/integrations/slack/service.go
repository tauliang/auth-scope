package slack

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Clock interface {
	Now() time.Time
}

type Store interface {
	SaveWorkspaceBinding(WorkspaceBinding) error
	GetWorkspaceBinding(string) (WorkspaceBinding, error)
	UpdateWorkspaceBinding(WorkspaceBinding) error
	ListWorkspaceBindings() ([]WorkspaceBinding, error)
}

type Evaluator interface {
	Evaluate(string, EvaluationRequest, map[string]any) (EvaluationResponse, error)
}

type EventSink interface {
	AppendEvent(Event) error
}

type Config struct {
	Store      Store
	Evaluator  Evaluator
	Events     EventSink
	Clock      Clock
	NewID      func(string) string
	IsConflict func(error) bool
}

type Service struct {
	store      Store
	evaluator  Evaluator
	events     EventSink
	clock      Clock
	newID      func(string) string
	isConflict func(error) bool
}

func NewService(config Config) *Service {
	isConflict := config.IsConflict
	if isConflict == nil {
		isConflict = func(error) bool { return false }
	}
	return &Service{
		store:      config.Store,
		evaluator:  config.Evaluator,
		events:     config.Events,
		clock:      config.Clock,
		newID:      config.NewID,
		isConflict: isConflict,
	}
}

func (s *Service) CreateWorkspaceBinding(req CreateWorkspaceBindingRequest, actor Principal) (WorkspaceBinding, error) {
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return WorkspaceBinding{}, fmt.Errorf("workspace_id is required")
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		return WorkspaceBinding{}, fmt.Errorf("mission_ref is required")
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	roleMatchMode := strings.ToLower(strings.TrimSpace(req.RoleMatchMode))
	if roleMatchMode == "" {
		roleMatchMode = RoleMatchAny
	}
	if roleMatchMode != RoleMatchAny && roleMatchMode != RoleMatchAll {
		return WorkspaceBinding{}, fmt.Errorf("role_match_mode must be any or all")
	}
	roleClaim := strings.TrimSpace(req.RoleClaim)
	if roleClaim == "" {
		roleClaim = "roles"
	}

	now := s.now()
	binding := WorkspaceBinding{
		BindingID:       s.id("slb"),
		TenantID:        strings.TrimSpace(req.TenantID),
		WorkspaceID:     workspaceID,
		WorkspaceName:   strings.TrimSpace(req.WorkspaceName),
		WorkspaceURL:    strings.TrimSpace(req.WorkspaceURL),
		MissionRef:      missionRef,
		RequiredRoles:   CleanStringList(req.RequiredRoles),
		AdminRoles:      CleanStringList(req.AdminRoles),
		AllowedChannels: CleanStringList(req.AllowedChannels),
		BlockedChannels: CleanStringList(req.BlockedChannels),
		AllowedUsers:    CleanStringList(req.AllowedUsers),
		AllowedActions:  CleanStringList(req.AllowedActions),
		RoleClaim:       roleClaim,
		RoleMatchMode:   roleMatchMode,
		Status:          WorkspaceBindingStatusActive,
		Metadata:        CloneStringMap(req.Metadata),
		CreatedBy:       actor,
		CreatedAt:       now,
	}
	if err := s.store.SaveWorkspaceBinding(binding); err != nil {
		return WorkspaceBinding{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		TenantID:   binding.TenantID,
		MissionRef: binding.MissionRef,
		Type:       "slack.workspace_bound",
		Actor:      map[string]any{"user_id": actor.UserID, "email": actor.Email},
		Payload: map[string]any{
			"binding_id":   binding.BindingID,
			"workspace_id": binding.WorkspaceID,
			"mission_ref":  binding.MissionRef,
			"roles_bound":  len(binding.RequiredRoles),
		},
		OccurredAt: now,
	})
	return binding, nil
}

func (s *Service) ListWorkspaceBindings() ([]WorkspaceBinding, error) {
	return s.store.ListWorkspaceBindings()
}

func (s *Service) AuthorizeMessageAction(req AuthorizeMessageActionRequest) (MessageAuthorizationResponse, error) {
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return MessageAuthorizationResponse{}, fmt.Errorf("workspace_id is required")
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return MessageAuthorizationResponse{}, fmt.Errorf("action is required")
	}

	binding, ok, err := s.bindingFor(workspaceID, strings.TrimSpace(req.MissionRef), strings.TrimSpace(req.TenantID))
	if err != nil {
		return MessageAuthorizationResponse{}, err
	}
	if !ok {
		return s.deniedResponse(WorkspaceBinding{}, workspaceID, strings.TrimSpace(req.UserID), req.Email, action, req.ChannelID, []string{"slack_no_matching_binding"}, "No active Slack workspace binding found."), nil
	}

	userID, email, roles, err := ExtractUserContext(req, binding)
	if err != nil {
		return MessageAuthorizationResponse{}, err
	}

	reasons := make([]string, 0, 3)

	// Check allowed users
	if len(binding.AllowedUsers) > 0 && !ContainsString(binding.AllowedUsers, userID) && !ContainsString(binding.AllowedUsers, email) {
		reasons = append(reasons, "slack_user_not_allowed")
	}

	// Check channel restrictions
	channelID := strings.TrimSpace(req.ChannelID)
	if len(binding.BlockedChannels) > 0 && ContainsString(binding.BlockedChannels, channelID) {
		reasons = append(reasons, "slack_channel_blocked")
	}
	if len(binding.AllowedChannels) > 0 && !ContainsString(binding.AllowedChannels, channelID) {
		reasons = append(reasons, "slack_channel_not_allowed")
	}

	// Check required roles
	if !RoleRequirementSatisfied(binding.RequiredRoles, roles, binding.RoleMatchMode) {
		reasons = append(reasons, "slack_required_role_missing")
	}

	// Check allowed actions
	if len(binding.AllowedActions) > 0 && !ContainsString(binding.AllowedActions, action) {
		reasons = append(reasons, "slack_action_not_allowed")
	}

	now := s.now()
	binding.LastResolvedAt = now
	binding.LastUserID = userID

	if len(reasons) > 0 {
		binding.LastResolutionStatus = ResolutionStatusDenied
		_ = s.store.UpdateWorkspaceBinding(binding)
		resp := s.deniedResponse(binding, workspaceID, userID, email, action, channelID, reasons, "User does not satisfy Slack workspace binding requirements.")
		s.appendResolutionEvent(binding, resp, now)
		return resp, nil
	}

	context := CloneContext(req.Context)
	admin := HasAny(roles, binding.AdminRoles)
	context["slack.workspace_id"] = binding.WorkspaceID
	context["slack.user_id"] = userID
	context["slack.email"] = email
	context["slack.roles"] = roles
	context["slack.channel_id"] = channelID
	context["slack.action"] = action
	context["slack.binding_id"] = binding.BindingID
	context["slack.admin"] = admin
	context["slack.required_roles"] = binding.RequiredRoles
	context["slack.role_match_mode"] = binding.RoleMatchMode

	resp := MessageAuthorizationResponse{
		Accepted:    true,
		Status:      ResolutionStatusAccepted,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		WorkspaceID: workspaceID,
		UserID:      userID,
		Email:       email,
		Roles:       roles,
		ChannelID:   channelID,
		Action:      action,
		Admin:       admin,
		ReasonCodes: []string{"slack_binding_satisfied"},
		HumanReason: "User satisfies the active Slack workspace binding requirements.",
		Context:     context,
		ResolvedAt:  now.Format(time.RFC3339),
	}

	if req.Evaluation != nil {
		if s.evaluator == nil {
			return MessageAuthorizationResponse{}, fmt.Errorf("slack evaluator is not configured")
		}
		decision, err := s.evaluator.Evaluate(binding.MissionRef, *req.Evaluation, context)
		if err != nil {
			return MessageAuthorizationResponse{}, err
		}
		resp.Evaluation = &decision
	}

	binding.LastResolutionStatus = ResolutionStatusAccepted
	_ = s.store.UpdateWorkspaceBinding(binding)
	s.appendResolutionEvent(binding, resp, now)
	return resp, nil
}

func (s *Service) bindingFor(workspaceID string, missionRef string, tenantID string) (WorkspaceBinding, bool, error) {
	bindings, err := s.store.ListWorkspaceBindings()
	if err != nil {
		return WorkspaceBinding{}, false, err
	}
	for _, binding := range bindings {
		if binding.Status != WorkspaceBindingStatusActive {
			continue
		}
		if binding.WorkspaceID != workspaceID {
			continue
		}
		if missionRef != "" && binding.MissionRef != missionRef {
			continue
		}
		if tenantID != "" && binding.TenantID != tenantID {
			continue
		}
		return binding, true, nil
	}
	return WorkspaceBinding{}, false, nil
}

func (s *Service) deniedResponse(binding WorkspaceBinding, workspaceID string, userID string, email string, action string, channelID string, reasons []string, humanReason string) MessageAuthorizationResponse {
	return MessageAuthorizationResponse{
		Accepted:    false,
		Status:      ResolutionStatusDenied,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		WorkspaceID: workspaceID,
		UserID:      userID,
		Email:       email,
		ChannelID:   channelID,
		Action:      action,
		ReasonCodes: reasons,
		HumanReason: humanReason,
		ResolvedAt:  s.now().Format(time.RFC3339),
	}
}

func (s *Service) appendResolutionEvent(binding WorkspaceBinding, resp MessageAuthorizationResponse, occurredAt time.Time) {
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "slack.message_action_evaluated",
		Actor:      map[string]any{"user_id": resp.UserID, "email": resp.Email},
		Payload: map[string]any{
			"binding_id":   resp.BindingID,
			"workspace_id": resp.WorkspaceID,
			"user_id":      resp.UserID,
			"action":       resp.Action,
			"channel_id":   resp.ChannelID,
			"accepted":     resp.Accepted,
			"status":       resp.Status,
			"reason_codes": resp.ReasonCodes,
			"admin":        resp.Admin,
		},
		OccurredAt: occurredAt,
	})
}

func (s *Service) appendEvent(event Event) {
	if s.events != nil {
		_ = s.events.AppendEvent(event)
	}
}

func (s *Service) now() time.Time {
	if s.clock == nil {
		return time.Now().UTC()
	}
	return s.clock.Now()
}

func (s *Service) id(prefix string) string {
	if s.newID == nil {
		return prefix
	}
	return s.newID(prefix)
}

func IsConflict(conflict error) func(error) bool {
	return func(err error) bool {
		return errors.Is(err, conflict)
	}
}

package slack

import (
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

const (
	WorkspaceBindingStatusActive   = contract.BindingStatusActive
	WorkspaceBindingStatusDisabled = contract.BindingStatusDisabled

	RoleMatchAny = contract.MatchAny
	RoleMatchAll = contract.MatchAll

	ResolutionStatusAccepted = contract.ResolutionStatusAccepted
	ResolutionStatusDenied   = contract.ResolutionStatusDenied

	ActionTypePostMessage   = "post_message"
	ActionTypeEditMessage   = "edit_message"
	ActionTypeDeleteMessage = "delete_message"
	ActionTypeReactMessage  = "react_message"
	ActionTypeStartThread   = "start_thread"
)

type Principal struct {
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
}

type Actor = contract.Actor

type WorkspaceBinding struct {
	BindingID            string            `json:"binding_id"`
	TenantID             string            `json:"tenant_id,omitempty"`
	WorkspaceID          string            `json:"workspace_id"`
	WorkspaceName        string            `json:"workspace_name,omitempty"`
	WorkspaceURL         string            `json:"workspace_url"`
	MissionRef           string            `json:"mission_ref"`
	RequiredRoles        []string          `json:"required_roles,omitempty"`
	AdminRoles           []string          `json:"admin_roles,omitempty"`
	AllowedChannels      []string          `json:"allowed_channels,omitempty"`
	BlockedChannels      []string          `json:"blocked_channels,omitempty"`
	AllowedUsers         []string          `json:"allowed_users,omitempty"`
	AllowedActions       []string          `json:"allowed_actions,omitempty"`
	RoleClaim            string            `json:"role_claim,omitempty"`
	RoleMatchMode        string            `json:"role_match_mode,omitempty"`
	Status               string            `json:"status"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	CreatedBy            Principal         `json:"created_by,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	LastResolvedAt       time.Time         `json:"last_resolved_at,omitempty"`
	LastUserID           string            `json:"last_user_id,omitempty"`
	LastResolutionStatus string            `json:"last_resolution_status,omitempty"`
}

type CreateWorkspaceBindingRequest struct {
	TenantID        string            `json:"tenant_id,omitempty"`
	WorkspaceID     string            `json:"workspace_id"`
	WorkspaceName   string            `json:"workspace_name,omitempty"`
	WorkspaceURL    string            `json:"workspace_url"`
	MissionRef      string            `json:"mission_ref"`
	RequiredRoles   []string          `json:"required_roles,omitempty"`
	AdminRoles      []string          `json:"admin_roles,omitempty"`
	AllowedChannels []string          `json:"allowed_channels,omitempty"`
	BlockedChannels []string          `json:"blocked_channels,omitempty"`
	AllowedUsers    []string          `json:"allowed_users,omitempty"`
	AllowedActions  []string          `json:"allowed_actions,omitempty"`
	RoleClaim       string            `json:"role_claim,omitempty"`
	RoleMatchMode   string            `json:"role_match_mode,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type ActionResource = contract.EvaluationActionResource
type MessageAction = contract.EvaluationAction
type EvaluationRequest = contract.EvaluationRequest
type EvaluationResponse = contract.EvaluationResponse

type AuthorizeMessageActionRequest struct {
	TenantID    string             `json:"tenant_id,omitempty"`
	MissionRef  string             `json:"mission_ref,omitempty"`
	WorkspaceID string             `json:"workspace_id,omitempty"`
	UserID      string             `json:"user_id,omitempty"`
	Email       string             `json:"email,omitempty"`
	Roles       []string           `json:"roles,omitempty"`
	ChannelID   string             `json:"channel_id,omitempty"`
	Action      string             `json:"action,omitempty"`
	Claims      map[string]any     `json:"claims,omitempty"`
	Context     map[string]any     `json:"context,omitempty"`
	Evaluation  *EvaluationRequest `json:"evaluation,omitempty"`
}

type MessageAuthorizationResponse struct {
	Accepted    bool                `json:"accepted"`
	Status      string              `json:"status"`
	BindingID   string              `json:"binding_id,omitempty"`
	TenantID    string              `json:"tenant_id,omitempty"`
	MissionRef  string              `json:"mission_ref,omitempty"`
	WorkspaceID string              `json:"workspace_id,omitempty"`
	UserID      string              `json:"user_id,omitempty"`
	Email       string              `json:"email,omitempty"`
	Roles       []string            `json:"roles,omitempty"`
	ChannelID   string              `json:"channel_id,omitempty"`
	Action      string              `json:"action,omitempty"`
	Admin       bool                `json:"admin"`
	ReasonCodes []string            `json:"reason_codes,omitempty"`
	HumanReason string              `json:"human_reason,omitempty"`
	Context     map[string]any      `json:"context,omitempty"`
	Evaluation  *EvaluationResponse `json:"evaluation,omitempty"`
	ResolvedAt  string              `json:"resolved_at,omitempty"`
}

type Event = contract.Event

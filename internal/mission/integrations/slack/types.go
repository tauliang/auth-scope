package slack

import "time"

const (
	WorkspaceBindingStatusActive   = "active"
	WorkspaceBindingStatusDisabled = "disabled"

	RoleMatchAny = "any"
	RoleMatchAll = "all"

	ResolutionStatusAccepted = "accepted"
	ResolutionStatusDenied   = "denied"

	ActionTypePostMessage = "post_message"
	ActionTypeEditMessage = "edit_message"
	ActionTypeDeleteMessage = "delete_message"
	ActionTypeReactMessage = "react_message"
	ActionTypeStartThread = "start_thread"
)

type Principal struct {
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
}

type Actor struct {
	AgentInstanceID string `json:"agent_instance_id"`
	ClientID        string `json:"client_id"`
	KeyThumbprint   string `json:"key_thumbprint,omitempty"`
}

type WorkspaceBinding struct {
	BindingID          string            `json:"binding_id"`
	TenantID           string            `json:"tenant_id,omitempty"`
	WorkspaceID        string            `json:"workspace_id"`
	WorkspaceName      string            `json:"workspace_name,omitempty"`
	WorkspaceURL       string            `json:"workspace_url"`
	MissionRef         string            `json:"mission_ref"`
	RequiredRoles      []string          `json:"required_roles,omitempty"`
	AdminRoles         []string          `json:"admin_roles,omitempty"`
	AllowedChannels    []string          `json:"allowed_channels,omitempty"`
	BlockedChannels    []string          `json:"blocked_channels,omitempty"`
	AllowedUsers       []string          `json:"allowed_users,omitempty"`
	AllowedActions     []string          `json:"allowed_actions,omitempty"`
	RoleClaim          string            `json:"role_claim,omitempty"`
	RoleMatchMode      string            `json:"role_match_mode,omitempty"`
	Status             string            `json:"status"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	CreatedBy          Principal         `json:"created_by,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	LastResolvedAt     time.Time         `json:"last_resolved_at,omitempty"`
	LastUserID         string            `json:"last_user_id,omitempty"`
	LastResolutionStatus string           `json:"last_resolution_status,omitempty"`
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

type ActionResource struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ChannelID string `json:"channel_id,omitempty"`
}

type MessageAction struct {
	Type      string         `json:"type"`
	Name      string         `json:"name,omitempty"`
	Resource  ActionResource `json:"resource"`
	Operation string         `json:"operation"`
}

type EvaluationRequest struct {
	MissionVersionSeen int            `json:"mission_version_seen,omitempty"`
	Actor              Actor          `json:"actor"`
	Action             MessageAction  `json:"action"`
}

type EvaluationResponse struct {
	Decision         string         `json:"decision"`
	MissionRef       string         `json:"mission_ref,omitempty"`
	MissionVersion   int            `json:"mission_version,omitempty"`
	ReasonCodes      []string       `json:"reason_codes,omitempty"`
	HumanReason      string         `json:"human_reason,omitempty"`
	DecisionArtifact string         `json:"decision_artifact,omitempty"`
	Constraints      map[string]any `json:"constraints,omitempty"`
}

type AuthorizeMessageActionRequest struct {
	TenantID   string             `json:"tenant_id,omitempty"`
	MissionRef string             `json:"mission_ref,omitempty"`
	WorkspaceID string            `json:"workspace_id,omitempty"`
	UserID     string             `json:"user_id,omitempty"`
	Email      string             `json:"email,omitempty"`
	Roles      []string           `json:"roles,omitempty"`
	ChannelID  string             `json:"channel_id,omitempty"`
	Action     string             `json:"action,omitempty"`
	Claims     map[string]any     `json:"claims,omitempty"`
	Context    map[string]any     `json:"context,omitempty"`
	Evaluation *EvaluationRequest `json:"evaluation,omitempty"`
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

type Event struct {
	EventID       string
	MissionRef    string
	TenantID      string
	Type          string
	Actor         map[string]any
	Payload       map[string]any
	VersionBefore int
	VersionAfter  int
	OccurredAt    time.Time
}

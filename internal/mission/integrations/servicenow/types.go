package servicenow

import "time"

const (
	TicketBindingStatusActive   = "active"
	TicketBindingStatusDisabled = "disabled"

	ResolutionStatusAccepted = "accepted"
	ResolutionStatusDenied   = "denied"
)

type Principal struct {
	Subject string `json:"subject"`
	Issuer  string `json:"issuer"`
}

type Actor struct {
	AgentInstanceID string `json:"agent_instance_id"`
	ClientID        string `json:"client_id"`
	KeyThumbprint   string `json:"key_thumbprint,omitempty"`
}

type TicketBinding struct {
	BindingID            string            `json:"binding_id"`
	TenantID             string            `json:"tenant_id,omitempty"`
	InstanceURL          string            `json:"instance_url"`
	ServiceNowSysID      string            `json:"servicenow_sys_id,omitempty"`
	ServiceNowNumber     string            `json:"service_now_number,omitempty"`
	State                string            `json:"state"`
	MissionRef           string            `json:"mission_ref"`
	AssignmentGroup      string            `json:"assignment_group,omitempty"`
	CallerID             string            `json:"caller_id,omitempty"`
	RequiredGroups       []string          `json:"required_groups,omitempty"`
	AdminGroups          []string          `json:"admin_groups,omitempty"`
	AllowedSubjects      []string          `json:"allowed_subjects,omitempty"`
	GroupClaim           string            `json:"group_claim,omitempty"`
	SubjectClaim         string            `json:"subject_claim,omitempty"`
	GroupMatchMode       string            `json:"group_match_mode,omitempty"`
	Status               string            `json:"status"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	CreatedBy            Principal         `json:"created_by,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	LastResolvedAt       time.Time         `json:"last_resolved_at,omitempty"`
	LastSubject          string            `json:"last_subject,omitempty"`
	LastResolutionStatus string            `json:"last_resolution_status,omitempty"`
}

type CreateTicketBindingRequest struct {
	TenantID           string            `json:"tenant_id,omitempty"`
	InstanceURL        string            `json:"instance_url"`
	ServiceNowSysID    string            `json:"servicenow_sys_id,omitempty"`
	State              string            `json:"state"`
	MissionRef         string            `json:"mission_ref"`
	AssignmentGroup    string            `json:"assignment_group,omitempty"`
	CallerID           string            `json:"caller_id,omitempty"`
	RequiredGroups     []string          `json:"required_groups,omitempty"`
	AdminGroups        []string          `json:"admin_groups,omitempty"`
	AllowedSubjects    []string          `json:"allowed_subjects,omitempty"`
	GroupClaim         string            `json:"group_claim,omitempty"`
	SubjectClaim       string            `json:"subject_claim,omitempty"`
	GroupMatchMode     string            `json:"group_match_mode,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type EvaluationActionResource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type EvaluationAction struct {
	Type      string                   `json:"type"`
	Name      string                   `json:"name,omitempty"`
	Resource  EvaluationActionResource `json:"resource"`
	Operation string                   `json:"operation"`
}

type EvaluationRequest struct {
	MissionVersionSeen int              `json:"mission_version_seen,omitempty"`
	Actor              Actor            `json:"actor"`
	Action             EvaluationAction `json:"action"`
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

type ResolveAuthorityContextRequest struct {
	TenantID   string             `json:"tenant_id,omitempty"`
	MissionRef string             `json:"mission_ref,omitempty"`
	Issuer     string             `json:"issuer,omitempty"`
	ClientID   string             `json:"client_id,omitempty"`
	Subject    string             `json:"subject,omitempty"`
	Groups     []string           `json:"groups,omitempty"`
	Claims     map[string]any     `json:"claims,omitempty"`
	Context    map[string]any     `json:"context,omitempty"`
	Evaluation *EvaluationRequest `json:"evaluation,omitempty"`
}

type AuthorityContextResponse struct {
	Accepted       bool                `json:"accepted"`
	Status         string              `json:"status"`
	BindingID      string              `json:"binding_id,omitempty"`
	TenantID       string              `json:"tenant_id,omitempty"`
	MissionRef     string              `json:"mission_ref,omitempty"`
	Subject        string              `json:"subject,omitempty"`
	Groups         []string            `json:"groups,omitempty"`
	Admin          bool                `json:"admin"`
	ReasonCodes    []string            `json:"reason_codes,omitempty"`
	HumanReason    string              `json:"human_reason,omitempty"`
	Context        map[string]any      `json:"context,omitempty"`
	Evaluation     *EvaluationResponse `json:"evaluation,omitempty"`
	ResolvedAt     string              `json:"resolved_at,omitempty"`
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

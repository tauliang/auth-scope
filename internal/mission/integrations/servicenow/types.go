package servicenow

import (
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

const (
	TicketBindingStatusActive   = contract.BindingStatusActive
	TicketBindingStatusDisabled = contract.BindingStatusDisabled

	ResolutionStatusAccepted = contract.ResolutionStatusAccepted
	ResolutionStatusDenied   = contract.ResolutionStatusDenied
)

type Principal = contract.Principal
type Actor = contract.Actor

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
	TenantID         string            `json:"tenant_id,omitempty"`
	InstanceURL      string            `json:"instance_url"`
	ServiceNowSysID  string            `json:"servicenow_sys_id,omitempty"`
	ServiceNowNumber string            `json:"service_now_number,omitempty"`
	State            string            `json:"state"`
	MissionRef       string            `json:"mission_ref"`
	AssignmentGroup  string            `json:"assignment_group,omitempty"`
	CallerID         string            `json:"caller_id,omitempty"`
	RequiredGroups   []string          `json:"required_groups,omitempty"`
	AdminGroups      []string          `json:"admin_groups,omitempty"`
	AllowedSubjects  []string          `json:"allowed_subjects,omitempty"`
	GroupClaim       string            `json:"group_claim,omitempty"`
	SubjectClaim     string            `json:"subject_claim,omitempty"`
	GroupMatchMode   string            `json:"group_match_mode,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type EvaluationActionResource = contract.EvaluationActionResource
type EvaluationAction = contract.EvaluationAction
type EvaluationRequest = contract.EvaluationRequest
type EvaluationResponse = contract.EvaluationResponse

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
	Accepted    bool                `json:"accepted"`
	Status      string              `json:"status"`
	BindingID   string              `json:"binding_id,omitempty"`
	TenantID    string              `json:"tenant_id,omitempty"`
	MissionRef  string              `json:"mission_ref,omitempty"`
	Subject     string              `json:"subject,omitempty"`
	Groups      []string            `json:"groups,omitempty"`
	Admin       bool                `json:"admin"`
	ReasonCodes []string            `json:"reason_codes,omitempty"`
	HumanReason string              `json:"human_reason,omitempty"`
	Context     map[string]any      `json:"context,omitempty"`
	Evaluation  *EvaluationResponse `json:"evaluation,omitempty"`
	ResolvedAt  string              `json:"resolved_at,omitempty"`
}

type Event = contract.Event

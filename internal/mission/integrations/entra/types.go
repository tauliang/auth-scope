package entra

import (
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

const (
	AppRegistrationStatusActive   = contract.BindingStatusActive
	AppRegistrationStatusDisabled = contract.BindingStatusDisabled

	GroupMatchAny = contract.MatchAny
	GroupMatchAll = contract.MatchAll

	ResolutionStatusAccepted = contract.ResolutionStatusAccepted
	ResolutionStatusDenied   = contract.ResolutionStatusDenied
)

type Principal = contract.Principal
type Actor = contract.Actor

type AppRegistration struct {
	RegistrationID       string            `json:"registration_id"`
	TenantID             string            `json:"tenant_id,omitempty"`
	TenantName           string            `json:"tenant_name,omitempty"`
	Issuer               string            `json:"issuer"`
	DiscoveryURL         string            `json:"discovery_url,omitempty"`
	JWKSURI              string            `json:"jwks_uri,omitempty"`
	ClientID             string            `json:"client_id"`
	AppID                string            `json:"app_id,omitempty"`
	AppName              string            `json:"app_name,omitempty"`
	MissionRef           string            `json:"mission_ref"`
	RequiredGroups       []string          `json:"required_groups,omitempty"`
	AdminGroups          []string          `json:"admin_groups,omitempty"`
	AllowedSubjects      []string          `json:"allowed_subjects,omitempty"`
	GroupClaim           string            `json:"group_claim,omitempty"`
	SubjectClaim         string            `json:"subject_claim,omitempty"`
	RolesClaim           string            `json:"roles_claim,omitempty"`
	GroupMatchMode       string            `json:"group_match_mode,omitempty"`
	Status               string            `json:"status"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	CreatedBy            Principal         `json:"created_by,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	LastResolvedAt       time.Time         `json:"last_resolved_at,omitempty"`
	LastSubject          string            `json:"last_subject,omitempty"`
	LastResolutionStatus string            `json:"last_resolution_status,omitempty"`
}

type CreateAppRegistrationRequest struct {
	TenantID        string            `json:"tenant_id,omitempty"`
	TenantName      string            `json:"tenant_name,omitempty"`
	Issuer          string            `json:"issuer"`
	ClientID        string            `json:"client_id"`
	AppID           string            `json:"app_id,omitempty"`
	AppName         string            `json:"app_name,omitempty"`
	MissionRef      string            `json:"mission_ref"`
	RequiredGroups  []string          `json:"required_groups,omitempty"`
	AdminGroups     []string          `json:"admin_groups,omitempty"`
	AllowedSubjects []string          `json:"allowed_subjects,omitempty"`
	GroupClaim      string            `json:"group_claim,omitempty"`
	SubjectClaim    string            `json:"subject_claim,omitempty"`
	RolesClaim      string            `json:"roles_claim,omitempty"`
	GroupMatchMode  string            `json:"group_match_mode,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
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
	Roles      []string           `json:"roles,omitempty"`
	Claims     map[string]any     `json:"claims,omitempty"`
	Context    map[string]any     `json:"context,omitempty"`
	Evaluation *EvaluationRequest `json:"evaluation,omitempty"`
}

type AuthorityContextResponse struct {
	Accepted       bool                `json:"accepted"`
	Status         string              `json:"status"`
	RegistrationID string              `json:"registration_id,omitempty"`
	TenantID       string              `json:"tenant_id,omitempty"`
	MissionRef     string              `json:"mission_ref,omitempty"`
	Issuer         string              `json:"issuer,omitempty"`
	ClientID       string              `json:"client_id,omitempty"`
	Subject        string              `json:"subject,omitempty"`
	Groups         []string            `json:"groups,omitempty"`
	Roles          []string            `json:"roles,omitempty"`
	Admin          bool                `json:"admin"`
	ReasonCodes    []string            `json:"reason_codes,omitempty"`
	HumanReason    string              `json:"human_reason,omitempty"`
	Context        map[string]any      `json:"context,omitempty"`
	Evaluation     *EvaluationResponse `json:"evaluation,omitempty"`
	ResolvedAt     string              `json:"resolved_at,omitempty"`
}

type Event = contract.Event

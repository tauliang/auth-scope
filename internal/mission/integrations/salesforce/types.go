package salesforce

import (
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

const (
	OrgBindingStatusActive   = contract.BindingStatusActive
	OrgBindingStatusDisabled = contract.BindingStatusDisabled

	PermissionMatchAny = contract.MatchAny
	PermissionMatchAll = contract.MatchAll

	ResolutionStatusAccepted = contract.ResolutionStatusAccepted
	ResolutionStatusDenied   = contract.ResolutionStatusDenied

	ActionReadRecord   = "read_record"
	ActionCreateRecord = "create_record"
	ActionUpdateRecord = "update_record"
	ActionDeleteRecord = "delete_record"
	ActionUpsertRecord = "upsert_record"
	ActionSubmitRecord = "submit_record"
)

type Principal = contract.Principal
type Actor = contract.Actor
type EvaluationActionResource = contract.EvaluationActionResource
type EvaluationAction = contract.EvaluationAction
type EvaluationRequest = contract.EvaluationRequest
type EvaluationResponse = contract.EvaluationResponse
type Event = contract.Event

type OrgBinding struct {
	BindingID              string            `json:"binding_id"`
	TenantID               string            `json:"tenant_id,omitempty"`
	InstanceURL            string            `json:"instance_url"`
	OrgID                  string            `json:"org_id,omitempty"`
	OrgName                string            `json:"org_name,omitempty"`
	MissionRef             string            `json:"mission_ref"`
	AllowedObjectAPINames  []string          `json:"allowed_object_api_names,omitempty"`
	AllowedRecordTypeIDs   []string          `json:"allowed_record_type_ids,omitempty"`
	AllowedRecordTypeNames []string          `json:"allowed_record_type_names,omitempty"`
	AllowedActions         []string          `json:"allowed_actions,omitempty"`
	RequiredProfiles       []string          `json:"required_profiles,omitempty"`
	RequiredPermissionSets []string          `json:"required_permission_sets,omitempty"`
	AdminProfiles          []string          `json:"admin_profiles,omitempty"`
	AdminPermissionSets    []string          `json:"admin_permission_sets,omitempty"`
	AllowedSubjects        []string          `json:"allowed_subjects,omitempty"`
	ProfileClaim           string            `json:"profile_claim,omitempty"`
	PermissionSetsClaim    string            `json:"permission_sets_claim,omitempty"`
	SubjectClaim           string            `json:"subject_claim,omitempty"`
	UsernameClaim          string            `json:"username_claim,omitempty"`
	EmailClaim             string            `json:"email_claim,omitempty"`
	PermissionSetMatchMode string            `json:"permission_set_match_mode,omitempty"`
	Status                 string            `json:"status"`
	Metadata               map[string]string `json:"metadata,omitempty"`
	CreatedBy              Principal         `json:"created_by,omitempty"`
	CreatedAt              time.Time         `json:"created_at"`
	LastResolvedAt         time.Time         `json:"last_resolved_at,omitempty"`
	LastSubject            string            `json:"last_subject,omitempty"`
	LastResolutionStatus   string            `json:"last_resolution_status,omitempty"`
}

type CreateOrgBindingRequest struct {
	TenantID               string            `json:"tenant_id,omitempty"`
	InstanceURL            string            `json:"instance_url"`
	OrgID                  string            `json:"org_id,omitempty"`
	OrgName                string            `json:"org_name,omitempty"`
	MissionRef             string            `json:"mission_ref"`
	AllowedObjectAPINames  []string          `json:"allowed_object_api_names,omitempty"`
	AllowedRecordTypeIDs   []string          `json:"allowed_record_type_ids,omitempty"`
	AllowedRecordTypeNames []string          `json:"allowed_record_type_names,omitempty"`
	AllowedActions         []string          `json:"allowed_actions,omitempty"`
	RequiredProfiles       []string          `json:"required_profiles,omitempty"`
	RequiredPermissionSets []string          `json:"required_permission_sets,omitempty"`
	AdminProfiles          []string          `json:"admin_profiles,omitempty"`
	AdminPermissionSets    []string          `json:"admin_permission_sets,omitempty"`
	AllowedSubjects        []string          `json:"allowed_subjects,omitempty"`
	ProfileClaim           string            `json:"profile_claim,omitempty"`
	PermissionSetsClaim    string            `json:"permission_sets_claim,omitempty"`
	SubjectClaim           string            `json:"subject_claim,omitempty"`
	UsernameClaim          string            `json:"username_claim,omitempty"`
	EmailClaim             string            `json:"email_claim,omitempty"`
	PermissionSetMatchMode string            `json:"permission_set_match_mode,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty"`
}

type AuthorizeRecordActionRequest struct {
	TenantID       string             `json:"tenant_id,omitempty"`
	MissionRef     string             `json:"mission_ref,omitempty"`
	InstanceURL    string             `json:"instance_url,omitempty"`
	OrgID          string             `json:"org_id,omitempty"`
	ObjectAPIName  string             `json:"object_api_name,omitempty"`
	RecordID       string             `json:"record_id,omitempty"`
	RecordTypeID   string             `json:"record_type_id,omitempty"`
	RecordTypeName string             `json:"record_type_name,omitempty"`
	UserID         string             `json:"user_id,omitempty"`
	Subject        string             `json:"subject,omitempty"`
	Username       string             `json:"username,omitempty"`
	Email          string             `json:"email,omitempty"`
	Profile        string             `json:"profile,omitempty"`
	PermissionSets []string           `json:"permission_sets,omitempty"`
	Action         string             `json:"action,omitempty"`
	Claims         map[string]any     `json:"claims,omitempty"`
	Context        map[string]any     `json:"context,omitempty"`
	Evaluation     *EvaluationRequest `json:"evaluation,omitempty"`
}

type RecordActionAuthorizationResponse struct {
	Accepted       bool                `json:"accepted"`
	Status         string              `json:"status"`
	BindingID      string              `json:"binding_id,omitempty"`
	TenantID       string              `json:"tenant_id,omitempty"`
	MissionRef     string              `json:"mission_ref,omitempty"`
	InstanceURL    string              `json:"instance_url,omitempty"`
	OrgID          string              `json:"org_id,omitempty"`
	ObjectAPIName  string              `json:"object_api_name,omitempty"`
	RecordID       string              `json:"record_id,omitempty"`
	RecordTypeID   string              `json:"record_type_id,omitempty"`
	RecordTypeName string              `json:"record_type_name,omitempty"`
	UserID         string              `json:"user_id,omitempty"`
	Subject        string              `json:"subject,omitempty"`
	Username       string              `json:"username,omitempty"`
	Email          string              `json:"email,omitempty"`
	Profile        string              `json:"profile,omitempty"`
	PermissionSets []string            `json:"permission_sets,omitempty"`
	Action         string              `json:"action,omitempty"`
	Admin          bool                `json:"admin"`
	ReasonCodes    []string            `json:"reason_codes,omitempty"`
	HumanReason    string              `json:"human_reason,omitempty"`
	Context        map[string]any      `json:"context,omitempty"`
	Evaluation     *EvaluationResponse `json:"evaluation,omitempty"`
	ResolvedAt     string              `json:"resolved_at,omitempty"`
}

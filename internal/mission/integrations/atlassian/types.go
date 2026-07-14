package atlassian

import (
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

const (
	SiteBindingStatusActive   = contract.BindingStatusActive
	SiteBindingStatusDisabled = contract.BindingStatusDisabled

	GroupMatchAny = contract.MatchAny
	GroupMatchAll = contract.MatchAll

	ResolutionStatusAccepted = contract.ResolutionStatusAccepted
	ResolutionStatusDenied   = contract.ResolutionStatusDenied

	JiraActionViewIssue       = "view_issue"
	JiraActionCreateIssue     = "create_issue"
	JiraActionUpdateIssue     = "update_issue"
	JiraActionTransitionIssue = "transition_issue"
	JiraActionCommentIssue    = "comment_issue"

	ConfluenceActionViewPage    = "view_page"
	ConfluenceActionCreatePage  = "create_page"
	ConfluenceActionUpdatePage  = "update_page"
	ConfluenceActionCommentPage = "comment_page"
)

type Principal = contract.Principal
type Actor = contract.Actor
type EvaluationActionResource = contract.EvaluationActionResource
type EvaluationAction = contract.EvaluationAction
type EvaluationRequest = contract.EvaluationRequest
type EvaluationResponse = contract.EvaluationResponse
type Event = contract.Event

type SiteBinding struct {
	BindingID            string            `json:"binding_id"`
	TenantID             string            `json:"tenant_id,omitempty"`
	SiteURL              string            `json:"site_url"`
	CloudID              string            `json:"cloud_id,omitempty"`
	SiteName             string            `json:"site_name,omitempty"`
	MissionRef           string            `json:"mission_ref"`
	JiraProjectKeys      []string          `json:"jira_project_keys,omitempty"`
	ConfluenceSpaceKeys  []string          `json:"confluence_space_keys,omitempty"`
	AllowedJiraActions   []string          `json:"allowed_jira_actions,omitempty"`
	AllowedPageActions   []string          `json:"allowed_page_actions,omitempty"`
	RequiredGroups       []string          `json:"required_groups,omitempty"`
	AdminGroups          []string          `json:"admin_groups,omitempty"`
	AllowedSubjects      []string          `json:"allowed_subjects,omitempty"`
	GroupClaim           string            `json:"group_claim,omitempty"`
	SubjectClaim         string            `json:"subject_claim,omitempty"`
	EmailClaim           string            `json:"email_claim,omitempty"`
	GroupMatchMode       string            `json:"group_match_mode,omitempty"`
	Status               string            `json:"status"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	CreatedBy            Principal         `json:"created_by,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	LastResolvedAt       time.Time         `json:"last_resolved_at,omitempty"`
	LastSubject          string            `json:"last_subject,omitempty"`
	LastResolutionStatus string            `json:"last_resolution_status,omitempty"`
}

type CreateSiteBindingRequest struct {
	TenantID            string            `json:"tenant_id,omitempty"`
	SiteURL             string            `json:"site_url"`
	CloudID             string            `json:"cloud_id,omitempty"`
	SiteName            string            `json:"site_name,omitempty"`
	MissionRef          string            `json:"mission_ref"`
	JiraProjectKeys     []string          `json:"jira_project_keys,omitempty"`
	ConfluenceSpaceKeys []string          `json:"confluence_space_keys,omitempty"`
	AllowedJiraActions  []string          `json:"allowed_jira_actions,omitempty"`
	AllowedPageActions  []string          `json:"allowed_page_actions,omitempty"`
	RequiredGroups      []string          `json:"required_groups,omitempty"`
	AdminGroups         []string          `json:"admin_groups,omitempty"`
	AllowedSubjects     []string          `json:"allowed_subjects,omitempty"`
	GroupClaim          string            `json:"group_claim,omitempty"`
	SubjectClaim        string            `json:"subject_claim,omitempty"`
	EmailClaim          string            `json:"email_claim,omitempty"`
	GroupMatchMode      string            `json:"group_match_mode,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

type AuthorizeJiraIssueActionRequest struct {
	TenantID   string             `json:"tenant_id,omitempty"`
	MissionRef string             `json:"mission_ref,omitempty"`
	SiteURL    string             `json:"site_url,omitempty"`
	CloudID    string             `json:"cloud_id,omitempty"`
	ProjectKey string             `json:"project_key,omitempty"`
	IssueKey   string             `json:"issue_key,omitempty"`
	IssueType  string             `json:"issue_type,omitempty"`
	AccountID  string             `json:"account_id,omitempty"`
	Subject    string             `json:"subject,omitempty"`
	Email      string             `json:"email,omitempty"`
	Groups     []string           `json:"groups,omitempty"`
	Action     string             `json:"action,omitempty"`
	Claims     map[string]any     `json:"claims,omitempty"`
	Context    map[string]any     `json:"context,omitempty"`
	Evaluation *EvaluationRequest `json:"evaluation,omitempty"`
}

type AuthorizeConfluencePageActionRequest struct {
	TenantID   string             `json:"tenant_id,omitempty"`
	MissionRef string             `json:"mission_ref,omitempty"`
	SiteURL    string             `json:"site_url,omitempty"`
	CloudID    string             `json:"cloud_id,omitempty"`
	SpaceKey   string             `json:"space_key,omitempty"`
	PageID     string             `json:"page_id,omitempty"`
	PageTitle  string             `json:"page_title,omitempty"`
	AccountID  string             `json:"account_id,omitempty"`
	Subject    string             `json:"subject,omitempty"`
	Email      string             `json:"email,omitempty"`
	Groups     []string           `json:"groups,omitempty"`
	Action     string             `json:"action,omitempty"`
	Claims     map[string]any     `json:"claims,omitempty"`
	Context    map[string]any     `json:"context,omitempty"`
	Evaluation *EvaluationRequest `json:"evaluation,omitempty"`
}

type ActionAuthorizationResponse struct {
	Accepted    bool                `json:"accepted"`
	Status      string              `json:"status"`
	Product     string              `json:"product"`
	BindingID   string              `json:"binding_id,omitempty"`
	TenantID    string              `json:"tenant_id,omitempty"`
	MissionRef  string              `json:"mission_ref,omitempty"`
	SiteURL     string              `json:"site_url,omitempty"`
	CloudID     string              `json:"cloud_id,omitempty"`
	ProjectKey  string              `json:"project_key,omitempty"`
	IssueKey    string              `json:"issue_key,omitempty"`
	IssueType   string              `json:"issue_type,omitempty"`
	SpaceKey    string              `json:"space_key,omitempty"`
	PageID      string              `json:"page_id,omitempty"`
	PageTitle   string              `json:"page_title,omitempty"`
	AccountID   string              `json:"account_id,omitempty"`
	Subject     string              `json:"subject,omitempty"`
	Email       string              `json:"email,omitempty"`
	Groups      []string            `json:"groups,omitempty"`
	Action      string              `json:"action,omitempty"`
	Admin       bool                `json:"admin"`
	ReasonCodes []string            `json:"reason_codes,omitempty"`
	HumanReason string              `json:"human_reason,omitempty"`
	Context     map[string]any      `json:"context,omitempty"`
	Evaluation  *EvaluationResponse `json:"evaluation,omitempty"`
	ResolvedAt  string              `json:"resolved_at,omitempty"`
}

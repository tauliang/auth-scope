package mission

import atlassianint "github.com/tauliang/auth-scope/internal/mission/integrations/atlassian"

const (
	AtlassianSiteBindingStatusActive   = atlassianint.SiteBindingStatusActive
	AtlassianSiteBindingStatusDisabled = atlassianint.SiteBindingStatusDisabled

	AtlassianGroupMatchAny = atlassianint.GroupMatchAny
	AtlassianGroupMatchAll = atlassianint.GroupMatchAll

	AtlassianResolutionStatusAccepted = atlassianint.ResolutionStatusAccepted
	AtlassianResolutionStatusDenied   = atlassianint.ResolutionStatusDenied

	AtlassianJiraActionViewIssue       = atlassianint.JiraActionViewIssue
	AtlassianJiraActionCreateIssue     = atlassianint.JiraActionCreateIssue
	AtlassianJiraActionUpdateIssue     = atlassianint.JiraActionUpdateIssue
	AtlassianJiraActionTransitionIssue = atlassianint.JiraActionTransitionIssue
	AtlassianJiraActionCommentIssue    = atlassianint.JiraActionCommentIssue

	AtlassianConfluenceActionViewPage    = atlassianint.ConfluenceActionViewPage
	AtlassianConfluenceActionCreatePage  = atlassianint.ConfluenceActionCreatePage
	AtlassianConfluenceActionUpdatePage  = atlassianint.ConfluenceActionUpdatePage
	AtlassianConfluenceActionCommentPage = atlassianint.ConfluenceActionCommentPage
)

type AtlassianPrincipal = atlassianint.Principal
type AtlassianActor = atlassianint.Actor
type AtlassianSiteBinding = atlassianint.SiteBinding
type CreateAtlassianSiteBindingRequest = atlassianint.CreateSiteBindingRequest
type AtlassianEvaluationActionResource = atlassianint.EvaluationActionResource
type AtlassianEvaluationAction = atlassianint.EvaluationAction
type AtlassianEvaluationRequest = atlassianint.EvaluationRequest
type AtlassianEvaluationResponse = atlassianint.EvaluationResponse
type AuthorizeAtlassianJiraIssueActionRequest = atlassianint.AuthorizeJiraIssueActionRequest
type AuthorizeAtlassianConfluencePageActionRequest = atlassianint.AuthorizeConfluencePageActionRequest
type AtlassianActionAuthorizationResponse = atlassianint.ActionAuthorizationResponse

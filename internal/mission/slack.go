package mission

import slackint "github.com/tauliang/auth-scope/internal/mission/integrations/slack"

const (
	SlackWorkspaceBindingStatusActive   = slackint.WorkspaceBindingStatusActive
	SlackWorkspaceBindingStatusDisabled = slackint.WorkspaceBindingStatusDisabled

	SlackRoleMatchAny = slackint.RoleMatchAny
	SlackRoleMatchAll = slackint.RoleMatchAll

	SlackResolutionStatusAccepted = slackint.ResolutionStatusAccepted
	SlackResolutionStatusDenied   = slackint.ResolutionStatusDenied

	SlackActionTypePostMessage   = slackint.ActionTypePostMessage
	SlackActionTypeEditMessage   = slackint.ActionTypeEditMessage
	SlackActionTypeDeleteMessage = slackint.ActionTypeDeleteMessage
	SlackActionTypeReactMessage  = slackint.ActionTypeReactMessage
	SlackActionTypeStartThread   = slackint.ActionTypeStartThread
)

type SlackPrincipal = slackint.Principal
type SlackActor = slackint.Actor
type SlackWorkspaceBinding = slackint.WorkspaceBinding
type CreateSlackWorkspaceBindingRequest = slackint.CreateWorkspaceBindingRequest
type SlackActionResource = slackint.ActionResource
type SlackMessageAction = slackint.MessageAction
type SlackEvaluationRequest = slackint.EvaluationRequest
type SlackEvaluationResponse = slackint.EvaluationResponse
type AuthorizeSlackMessageActionRequest = slackint.AuthorizeMessageActionRequest
type SlackMessageAuthorizationResponse = slackint.MessageAuthorizationResponse

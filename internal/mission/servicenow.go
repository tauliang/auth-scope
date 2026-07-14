package mission

import snint "github.com/tauliang/auth-scope/internal/mission/integrations/servicenow"

const (
	ServiceNowTicketBindingStatusActive   = snint.TicketBindingStatusActive
	ServiceNowTicketBindingStatusDisabled = snint.TicketBindingStatusDisabled

	ServiceNowResolutionStatusAccepted = snint.ResolutionStatusAccepted
	ServiceNowResolutionStatusDenied   = snint.ResolutionStatusDenied
)

type ServiceNowPrincipal = snint.Principal
type ServiceNowActor = snint.Actor
type ServiceNowTicketBinding = snint.TicketBinding
type CreateServiceNowTicketBindingRequest = snint.CreateTicketBindingRequest
type ServiceNowEvaluationActionResource = snint.EvaluationActionResource
type ServiceNowEvaluationAction = snint.EvaluationAction
type ServiceNowEvaluationRequest = snint.EvaluationRequest
type ServiceNowEvaluationResponse = snint.EvaluationResponse
type ResolveServiceNowAuthorityContextRequest = snint.ResolveAuthorityContextRequest
type ServiceNowAuthorityContextResponse = snint.AuthorityContextResponse

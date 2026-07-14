package mission

import oktaint "github.com/tauliang/auth-scope/internal/mission/integrations/okta"

const (
	OktaAppBindingStatusActive   = oktaint.AppBindingStatusActive
	OktaAppBindingStatusDisabled = oktaint.AppBindingStatusDisabled

	OktaGroupMatchAny = oktaint.GroupMatchAny
	OktaGroupMatchAll = oktaint.GroupMatchAll

	OktaResolutionStatusAccepted = oktaint.ResolutionStatusAccepted
	OktaResolutionStatusDenied   = oktaint.ResolutionStatusDenied
)

type OktaPrincipal = oktaint.Principal
type OktaActor = oktaint.Actor
type OktaAppBinding = oktaint.AppBinding
type CreateOktaAppBindingRequest = oktaint.CreateAppBindingRequest
type OktaEvaluationActionResource = oktaint.EvaluationActionResource
type OktaEvaluationAction = oktaint.EvaluationAction
type OktaEvaluationRequest = oktaint.EvaluationRequest
type OktaEvaluationResponse = oktaint.EvaluationResponse
type ResolveOktaAuthorityContextRequest = oktaint.ResolveAuthorityContextRequest
type OktaAuthorityContextResponse = oktaint.AuthorityContextResponse

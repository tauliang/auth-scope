package mission

import entraint "github.com/tauliang/auth-scope/internal/mission/integrations/entra"

const (
	EntraAppRegistrationStatusActive   = entraint.AppRegistrationStatusActive
	EntraAppRegistrationStatusDisabled = entraint.AppRegistrationStatusDisabled

	EntraGroupMatchAny = entraint.GroupMatchAny
	EntraGroupMatchAll = entraint.GroupMatchAll

	EntraResolutionStatusAccepted = entraint.ResolutionStatusAccepted
	EntraResolutionStatusDenied   = entraint.ResolutionStatusDenied
)

type EntraPrincipal = entraint.Principal
type EntraActor = entraint.Actor
type EntraAppRegistration = entraint.AppRegistration
type CreateEntraAppRegistrationRequest = entraint.CreateAppRegistrationRequest
type EntraEvaluationActionResource = entraint.EvaluationActionResource
type EntraEvaluationAction = entraint.EvaluationAction
type EntraEvaluationRequest = entraint.EvaluationRequest
type EntraEvaluationResponse = entraint.EvaluationResponse
type ResolveEntraAuthorityContextRequest = entraint.ResolveAuthorityContextRequest
type EntraAuthorityContextResponse = entraint.AuthorityContextResponse

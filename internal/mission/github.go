package mission

import githubint "github.com/tauliang/auth-scope/internal/mission/integrations/github"

const (
	GitHubRepositoryBindingStatusActive   = githubint.RepositoryBindingStatusActive
	GitHubRepositoryBindingStatusDisabled = githubint.RepositoryBindingStatusDisabled

	GitHubWebhookDeliveryStatusAccepted  = githubint.WebhookDeliveryStatusAccepted
	GitHubWebhookDeliveryStatusDuplicate = githubint.WebhookDeliveryStatusDuplicate
	GitHubWebhookDeliveryStatusIgnored   = githubint.WebhookDeliveryStatusIgnored

	GitHubCheckConclusionSuccess        = githubint.CheckConclusionSuccess
	GitHubCheckConclusionFailure        = githubint.CheckConclusionFailure
	GitHubCheckConclusionActionRequired = githubint.CheckConclusionActionRequired
	GitHubCheckConclusionNeutral        = githubint.CheckConclusionNeutral
)

type GitHubPrincipal = githubint.Principal
type GitHubActor = githubint.Actor
type GitHubRepositoryBinding = githubint.RepositoryBinding
type CreateGitHubRepositoryBindingRequest = githubint.CreateRepositoryBindingRequest
type GitHubWebhookDelivery = githubint.WebhookDelivery
type IngestGitHubWebhookRequest = githubint.IngestWebhookRequest
type GitHubWebhookResponse = githubint.WebhookResponse
type GitHubChangedFile = githubint.ChangedFile
type GitHubCheckRunPlanRequest = githubint.CheckRunPlanRequest
type GitHubCheckEvaluation = githubint.CheckEvaluation
type GitHubCheckRunPlanResponse = githubint.CheckRunPlanResponse

func ValidateGitHubWebhookSignature(secret []byte, signatureHeader string, body []byte) bool {
	return githubint.ValidateWebhookSignature(secret, signatureHeader, body)
}

func SignGitHubWebhookBody(secret []byte, body []byte) string {
	return githubint.SignWebhookBody(secret, body)
}

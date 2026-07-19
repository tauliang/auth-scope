package github

import "time"

const (
	RepositoryBindingStatusActive   = "active"
	RepositoryBindingStatusDisabled = "disabled"

	WebhookDeliveryStatusAccepted  = "accepted"
	WebhookDeliveryStatusDuplicate = "duplicate"
	WebhookDeliveryStatusIgnored   = "ignored"

	CheckConclusionSuccess        = "success"
	CheckConclusionFailure        = "failure"
	CheckConclusionActionRequired = "action_required"
	CheckConclusionNeutral        = "neutral"
)

type Principal struct {
	Subject string `json:"subject"`
	Issuer  string `json:"issuer"`
}

type Actor struct {
	AgentInstanceID string `json:"agent_instance_id"`
	ClientID        string `json:"client_id"`
	KeyThumbprint   string `json:"key_thumbprint,omitempty"`
}

type RepositoryBinding struct {
	BindingID       string            `json:"binding_id"`
	TenantID        string            `json:"tenant_id,omitempty"`
	Owner           string            `json:"owner"`
	Repo            string            `json:"repo"`
	Repository      string            `json:"repository"`
	DefaultBranch   string            `json:"default_branch,omitempty"`
	InstallationID  int64             `json:"installation_id,omitempty"`
	MissionRef      string            `json:"mission_ref,omitempty"`
	BranchPatterns  []string          `json:"branch_patterns,omitempty"`
	RequiredChecks  []string          `json:"required_checks,omitempty"`
	Status          string            `json:"status"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedBy       Principal         `json:"created_by,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	LastWebhookAt   time.Time         `json:"last_webhook_at,omitempty"`
	LastDeliveryID  string            `json:"last_delivery_id,omitempty"`
	LastCheckSHA    string            `json:"last_check_sha,omitempty"`
	LastCheckStatus string            `json:"last_check_status,omitempty"`
}

type CreateRepositoryBindingRequest struct {
	TenantID       string            `json:"tenant_id,omitempty"`
	Owner          string            `json:"owner,omitempty"`
	Repo           string            `json:"repo,omitempty"`
	Repository     string            `json:"repository,omitempty"`
	DefaultBranch  string            `json:"default_branch,omitempty"`
	InstallationID int64             `json:"installation_id,omitempty"`
	MissionRef     string            `json:"mission_ref,omitempty"`
	BranchPatterns []string          `json:"branch_patterns,omitempty"`
	RequiredChecks []string          `json:"required_checks,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type WebhookDelivery struct {
	DeliveryID     string         `json:"delivery_id"`
	Event          string         `json:"event"`
	Action         string         `json:"action,omitempty"`
	Repository     string         `json:"repository,omitempty"`
	Ref            string         `json:"ref,omitempty"`
	SHA            string         `json:"sha,omitempty"`
	PullRequest    int            `json:"pull_request,omitempty"`
	Branch         string         `json:"branch,omitempty"`
	BindingID      string         `json:"binding_id,omitempty"`
	TenantID       string         `json:"tenant_id,omitempty"`
	MissionRef     string         `json:"mission_ref,omitempty"`
	Status         string         `json:"status"`
	ReceivedAt     time.Time      `json:"received_at"`
	PayloadSummary map[string]any `json:"payload_summary,omitempty"`
}

type IngestWebhookRequest struct {
	Event      string
	DeliveryID string
	Signature  string
	Body       []byte
}

type WebhookResponse struct {
	Accepted   bool   `json:"accepted"`
	Status     string `json:"status"`
	Event      string `json:"event"`
	DeliveryID string `json:"delivery_id"`
	Repository string `json:"repository,omitempty"`
	Action     string `json:"action,omitempty"`
	Ref        string `json:"ref,omitempty"`
	SHA        string `json:"sha,omitempty"`
	BindingID  string `json:"binding_id,omitempty"`
	MissionRef string `json:"mission_ref,omitempty"`
	Message    string `json:"message,omitempty"`
	ReceivedAt string `json:"received_at,omitempty"`
}

type ChangedFile struct {
	Path      string `json:"path"`
	Status    string `json:"status,omitempty"`
	Operation string `json:"operation,omitempty"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
}

type CheckRunPlanRequest struct {
	MissionRef         string         `json:"mission_ref,omitempty"`
	MissionVersionSeen int            `json:"mission_version_seen,omitempty"`
	Actor              Actor          `json:"actor"`
	Repository         string         `json:"repository"`
	PullRequest        int            `json:"pull_request,omitempty"`
	HeadSHA            string         `json:"head_sha"`
	Branch             string         `json:"branch,omitempty"`
	ChangedFiles       []ChangedFile  `json:"changed_files"`
	Context            map[string]any `json:"context,omitempty"`
}

type CheckEvaluation struct {
	Path             string         `json:"path"`
	ResourceID       string         `json:"resource_id"`
	Operation        string         `json:"operation"`
	Decision         string         `json:"decision"`
	ReasonCodes      []string       `json:"reason_codes,omitempty"`
	HumanReason      string         `json:"human_reason,omitempty"`
	DecisionArtifact string         `json:"decision_artifact,omitempty"`
	Constraints      map[string]any `json:"constraints,omitempty"`
}

type CheckRunPlanResponse struct {
	Name           string            `json:"name"`
	ExternalID     string            `json:"external_id"`
	Repository     string            `json:"repository"`
	HeadSHA        string            `json:"head_sha"`
	Status         string            `json:"status"`
	Conclusion     string            `json:"conclusion"`
	Title          string            `json:"title"`
	Summary        string            `json:"summary"`
	Text           string            `json:"text,omitempty"`
	MissionRef     string            `json:"mission_ref"`
	MissionVersion int               `json:"mission_version,omitempty"`
	BindingID      string            `json:"binding_id,omitempty"`
	Evaluations    []CheckEvaluation `json:"evaluations"`
}

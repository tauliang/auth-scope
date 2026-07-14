package mission

import "time"

type State string

const (
	StateDraft           State = "draft"
	StatePendingApproval State = "pending_approval"
	StateActive          State = "active"
	StateSuspended       State = "suspended"
	StateCompleted       State = "completed"
	StateRevoked         State = "revoked"
	StateExpired         State = "expired"
	StateRejected        State = "rejected"
)

type Decision string

const (
	DecisionAllow               Decision = "allow"
	DecisionDeny                Decision = "deny"
	DecisionRequireApproval     Decision = "require_approval"
	DecisionRequireExpansion    Decision = "require_expansion"
	DecisionAllowWithConstraint Decision = "allow_with_constraints"
	DecisionRequireRefresh      Decision = "require_refresh"
	DecisionSuspend             Decision = "suspend"
)

type Principal struct {
	Subject       string `json:"subject"`
	Issuer        string `json:"issuer"`
	TenantSubject string `json:"tenant_subject,omitempty"`
	GrantRef      string `json:"grant_ref,omitempty"`
}

type Agent struct {
	Provider      string `json:"provider"`
	ClientID      string `json:"client_id"`
	InstanceID    string `json:"instance_id"`
	KeyThumbprint string `json:"key_thumbprint,omitempty"`
}

type Purpose struct {
	Objective          string `json:"objective"`
	BusinessContext    string `json:"business_context,omitempty"`
	Template           string `json:"template,omitempty"`
	DisplaySummaryHash string `json:"display_summary_hash,omitempty"`
}

type AuthorityRegion struct {
	Resources        []ResourceGrant `json:"resources"`
	ForbiddenActions []string        `json:"forbidden_actions,omitempty"`
}

type ResourceGrant struct {
	Type        string         `json:"type"`
	ID          string         `json:"id"`
	Actions     []string       `json:"actions"`
	Constraints map[string]any `json:"constraints,omitempty"`
}

type Condition struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Evaluation string `json:"evaluation,omitempty"`
	OnFailure  string `json:"on_failure,omitempty"`
}

type Lifecycle struct {
	CreatedAt      time.Time `json:"created_at,omitempty"`
	NotBefore      time.Time `json:"not_before,omitempty"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
	TerminalEvents []string  `json:"terminal_events,omitempty"`
	OnExpiry       string    `json:"on_expiry,omitempty"`
}

type DelegationPolicy struct {
	Permitted         bool   `json:"permitted"`
	MaxDepth          int    `json:"max_depth"`
	CurrentDepth      int    `json:"current_depth"`
	Attenuation       string `json:"attenuation,omitempty"`
	CascadeRevocation bool   `json:"cascade_revocation"`
	ParentMissionRef  string `json:"parent_mission_ref,omitempty"`
}

type RiskPolicy struct {
	DefaultMode                   string   `json:"default_mode,omitempty"`
	SyncRequiredFor               []string `json:"sync_required_for,omitempty"`
	FailClosedFor                 []string `json:"fail_closed_for,omitempty"`
	RevocationLagToleranceSeconds int      `json:"revocation_lag_tolerance_seconds,omitempty"`
}

type Mission struct {
	MissionID       string           `json:"mission_id"`
	MissionRef      string           `json:"mission_ref"`
	TenantID        string           `json:"tenant_id"`
	State           State            `json:"state"`
	Version         int              `json:"version"`
	Principal       Principal        `json:"principal"`
	Agent           Agent            `json:"agent"`
	Purpose         Purpose          `json:"purpose"`
	AuthorityRegion AuthorityRegion  `json:"authority_region"`
	Conditions      []Condition      `json:"conditions,omitempty"`
	Lifecycle       Lifecycle        `json:"lifecycle"`
	Delegation      DelegationPolicy `json:"delegation"`
	Risk            RiskPolicy       `json:"risk,omitempty"`
	Approval        ApprovalEvidence `json:"approval_evidence,omitempty"`
}

type ApprovalEvidence struct {
	Approver    Principal `json:"approver,omitempty"`
	ApprovedAt  time.Time `json:"approved_at,omitempty"`
	DisplayHash string    `json:"display_hash,omitempty"`
	Method      string    `json:"method,omitempty"`
}

type MissionProposal struct {
	ProposalID      string           `json:"proposal_id"`
	Status          State            `json:"status"`
	TenantID        string           `json:"tenant_id"`
	Principal       Principal        `json:"principal"`
	Agent           Agent            `json:"agent"`
	Intent          Purpose          `json:"intent"`
	AuthorityRegion AuthorityRegion  `json:"authority_region"`
	Conditions      []Condition      `json:"conditions,omitempty"`
	Lifecycle       Lifecycle        `json:"lifecycle"`
	Delegation      DelegationPolicy `json:"delegation"`
	Risk            RiskPolicy       `json:"risk,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

type Event struct {
	EventID       string         `json:"event_id"`
	MissionRef    string         `json:"mission_ref,omitempty"`
	TenantID      string         `json:"tenant_id,omitempty"`
	Type          string         `json:"type"`
	Actor         map[string]any `json:"actor,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
	VersionBefore int            `json:"version_before,omitempty"`
	VersionAfter  int            `json:"version_after,omitempty"`
	OccurredAt    time.Time      `json:"occurred_at"`
	CausationID   string         `json:"causation_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}

type CreateProposalRequest struct {
	TenantID        string           `json:"tenant_id"`
	Principal       Principal        `json:"principal"`
	Agent           Agent            `json:"agent"`
	Intent          Purpose          `json:"intent"`
	AuthorityRegion AuthorityRegion  `json:"authority_region"`
	Conditions      []Condition      `json:"conditions,omitempty"`
	Lifecycle       Lifecycle        `json:"lifecycle"`
	Delegation      DelegationPolicy `json:"delegation"`
	Risk            RiskPolicy       `json:"risk,omitempty"`
}

type CreateProposalResponse struct {
	ProposalID         string `json:"proposal_id"`
	Status             State  `json:"status"`
	ProposedMissionRef string `json:"proposed_mission_ref"`
	ApprovalURL        string `json:"approval_url"`
	DisplaySummary     string `json:"display_summary"`
}

type ApproveProposalRequest struct {
	Approver         Principal        `json:"approver"`
	ApprovalEvidence ApprovalEvidence `json:"approval_evidence"`
}

type ApproveProposalResponse struct {
	MissionRef     string `json:"mission_ref"`
	MissionVersion int    `json:"mission_version"`
	State          State  `json:"state"`
}

type Actor struct {
	AgentInstanceID string `json:"agent_instance_id"`
	ClientID        string `json:"client_id"`
	KeyThumbprint   string `json:"key_thumbprint,omitempty"`
}

type ActionResource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Action struct {
	Type      string         `json:"type"`
	Name      string         `json:"name,omitempty"`
	Resource  ActionResource `json:"resource"`
	Operation string         `json:"operation"`
}

type EvaluateRequest struct {
	MissionVersionSeen int            `json:"mission_version_seen,omitempty"`
	Actor              Actor          `json:"actor"`
	Action             Action         `json:"action"`
	Context            map[string]any `json:"context,omitempty"`
}

type EvaluateResponse struct {
	Decision         Decision       `json:"decision"`
	MissionRef       string         `json:"mission_ref,omitempty"`
	MissionVersion   int            `json:"mission_version,omitempty"`
	ReasonCodes      []string       `json:"reason_codes,omitempty"`
	HumanReason      string         `json:"human_reason,omitempty"`
	Constraints      map[string]any `json:"constraints,omitempty"`
	Escalation       *Escalation    `json:"escalation,omitempty"`
	DecisionArtifact string         `json:"decision_artifact,omitempty"`
}

type Escalation struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

type ResumeRequest struct {
	MissionVersionSeen int   `json:"mission_version_seen,omitempty"`
	Actor              Actor `json:"actor"`
}

type DelegationRequest struct {
	DelegatingActor    Actor            `json:"delegating_actor"`
	TargetAgent        Agent            `json:"target_agent"`
	RequestedAuthority AuthorityRegion  `json:"requested_authority"`
	Conditions         []Condition      `json:"conditions,omitempty"`
	ExpiresAt          time.Time        `json:"expires_at,omitempty"`
	Delegation         DelegationPolicy `json:"delegation"`
}

type DelegationResponse struct {
	ChildMissionRef  string `json:"child_mission_ref"`
	ParentMissionRef string `json:"parent_mission_ref"`
	State            State  `json:"state"`
	Attenuation      string `json:"attenuation"`
}

type StateChangeRequest struct {
	Actor       map[string]any `json:"actor,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	CausationID string         `json:"causation_id,omitempty"`
}

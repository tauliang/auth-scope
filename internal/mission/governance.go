package mission

import "time"

const (
	ExpansionStatusPending  = "pending"
	ExpansionStatusApproved = "approved"
	ExpansionStatusDenied   = "denied"

	DefaultPolicyVersionID = "mission-policy/v1"
)

type ExpansionRequest struct {
	ExpansionID        string            `json:"expansion_id"`
	MissionRef         string            `json:"mission_ref"`
	MissionVersionSeen int               `json:"mission_version_seen,omitempty"`
	TenantID           string            `json:"tenant_id"`
	Requester          Actor             `json:"requester"`
	Action             Action            `json:"action"`
	Context            map[string]any    `json:"context,omitempty"`
	RequestedAuthority AuthorityRegion   `json:"requested_authority"`
	Justification      string            `json:"justification,omitempty"`
	Status             string            `json:"status"`
	CreatedAt          time.Time         `json:"created_at"`
	DecidedAt          time.Time         `json:"decided_at,omitempty"`
	Approver           Principal         `json:"approver,omitempty"`
	DecisionReason     string            `json:"decision_reason,omitempty"`
	ApprovalEvidence   ApprovalEvidence  `json:"approval_evidence,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type CreateExpansionRequest struct {
	MissionVersionSeen int             `json:"mission_version_seen,omitempty"`
	Requester          Actor           `json:"requester"`
	Action             Action          `json:"action"`
	Context            map[string]any  `json:"context,omitempty"`
	RequestedAuthority AuthorityRegion `json:"requested_authority,omitempty"`
	Justification      string          `json:"justification,omitempty"`
}

type ExpansionDecisionRequest struct {
	Approver         Principal        `json:"approver"`
	ApprovalEvidence ApprovalEvidence `json:"approval_evidence,omitempty"`
	Reason           string           `json:"reason,omitempty"`
}

type ExpansionDecisionResponse struct {
	ExpansionID    string `json:"expansion_id"`
	Status         string `json:"status"`
	MissionRef     string `json:"mission_ref"`
	MissionVersion int    `json:"mission_version,omitempty"`
}

type ConditionEvaluation struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Result     bool   `json:"result"`
	Error      string `json:"error,omitempty"`
}

type EvaluationEvidence struct {
	EvidenceID       string                `json:"evidence_id"`
	MissionRef       string                `json:"mission_ref"`
	MissionVersion   int                   `json:"mission_version"`
	TenantID         string                `json:"tenant_id,omitempty"`
	PolicyVersion    string                `json:"policy_version"`
	Actor            Actor                 `json:"actor"`
	Action           Action                `json:"action"`
	ContextHash      string                `json:"context_hash"`
	Decision         Decision              `json:"decision"`
	ReasonCodes      []string              `json:"reason_codes,omitempty"`
	ConditionResults []ConditionEvaluation `json:"condition_results,omitempty"`
	Artifact         string                `json:"decision_artifact"`
	CreatedAt        time.Time             `json:"created_at"`
}

type VerifyDecisionArtifactRequest struct {
	DecisionArtifact string `json:"decision_artifact"`
}

type VerifyDecisionArtifactResponse struct {
	Valid    bool                    `json:"valid"`
	Payload  DecisionArtifactPayload `json:"payload,omitempty"`
	Evidence *EvaluationEvidence     `json:"evidence,omitempty"`
	Error    string                  `json:"error,omitempty"`
}

type ToolContract struct {
	ToolName        string            `json:"tool_name"`
	ResourceType    string            `json:"resource_type"`
	ResourceID      string            `json:"resource_id,omitempty"`
	ResourceIDParam string            `json:"resource_id_param,omitempty"`
	Operation       string            `json:"operation"`
	OperationParam  string            `json:"operation_param,omitempty"`
	ActionType      string            `json:"action_type,omitempty"`
	RequiredContext []string          `json:"required_context,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedBy       Principal         `json:"created_by,omitempty"`
	CreatedAt       time.Time         `json:"created_at,omitempty"`
}

type AuthorizeToolCallRequest struct {
	MissionRef         string         `json:"mission_ref"`
	MissionVersionSeen int            `json:"mission_version_seen,omitempty"`
	Actor              Actor          `json:"actor"`
	ToolName           string         `json:"tool_name"`
	Arguments          map[string]any `json:"arguments,omitempty"`
	Context            map[string]any `json:"context,omitempty"`
}

type AuthorizeToolCallResponse struct {
	ToolName         string         `json:"tool_name"`
	Decision         Decision       `json:"decision"`
	MissionRef       string         `json:"mission_ref,omitempty"`
	MissionVersion   int            `json:"mission_version,omitempty"`
	ReasonCodes      []string       `json:"reason_codes,omitempty"`
	HumanReason      string         `json:"human_reason,omitempty"`
	DecisionArtifact string         `json:"decision_artifact,omitempty"`
	Constraints      map[string]any `json:"constraints,omitempty"`
	Escalation       *Escalation    `json:"escalation,omitempty"`
}

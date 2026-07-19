package mission

import "time"

const (
	NegotiationStatusAccepted         = "accepted"
	NegotiationStatusCounteroffered   = "counteroffered"
	NegotiationStatusRequiresApproval = "requires_human_approval"
	NegotiationStatusDenied           = "denied"

	ContainmentStatusActive = "active"
	ContainmentStatusLifted = "lifted"

	ContainmentTargetAgent     = "agent"
	ContainmentTargetPrincipal = "principal"
	ContainmentTargetTool      = "tool"
	ContainmentTargetResource  = "resource"
	ContainmentTargetMission   = "mission"
	ContainmentTargetTenant    = "tenant"
)

type AuthorityNegotiation struct {
	NegotiationID      string          `json:"negotiation_id"`
	MissionRef         string          `json:"mission_ref"`
	MissionVersion     int             `json:"mission_version"`
	TenantID           string          `json:"tenant_id,omitempty"`
	Actor              Actor           `json:"actor"`
	RequestedAuthority AuthorityRegion `json:"requested_authority"`
	ProposedAuthority  AuthorityRegion `json:"proposed_authority,omitempty"`
	DeniedAuthority    AuthorityRegion `json:"denied_authority,omitempty"`
	Status             string          `json:"status"`
	Rationale          []string        `json:"rationale,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type CreateAuthorityNegotiationRequest struct {
	MissionVersionSeen int             `json:"mission_version_seen,omitempty"`
	Actor              Actor           `json:"actor"`
	RequestedAuthority AuthorityRegion `json:"requested_authority"`
	Context            map[string]any  `json:"context,omitempty"`
}

type ContainmentRule struct {
	RuleID     string         `json:"rule_id"`
	TenantID   string         `json:"tenant_id,omitempty"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Status     string         `json:"status"`
	Reason     string         `json:"reason,omitempty"`
	CreatedBy  Principal      `json:"created_by,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	ExpiresAt  time.Time      `json:"expires_at,omitempty"`
	LiftedAt   time.Time      `json:"lifted_at,omitempty"`
}

type BlastRadius struct {
	Rule              ContainmentRule    `json:"rule"`
	Missions          []Mission          `json:"missions,omitempty"`
	Projections       []Projection       `json:"projections,omitempty"`
	Leases            []MissionLease     `json:"leases,omitempty"`
	ExpansionRequests []ExpansionRequest `json:"expansion_requests,omitempty"`
	Agents            []AgentIdentity    `json:"agents,omitempty"`
	ToolContracts     []ToolContract     `json:"tool_contracts,omitempty"`
}

type LineageGraph struct {
	Nodes []LineageNode `json:"nodes"`
	Edges []LineageEdge `json:"edges"`
}

type LineageNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Label    string         `json:"label"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type LineageEdge struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Type     string         `json:"type"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

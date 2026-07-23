package mission

import "time"

const (
	ProjectionStatusActive  = "active"
	ProjectionStatusRevoked = "revoked"
	ProjectionStatusExpired = "expired"

	ProjectionTypeOAuthClaims      = "oauth_claims"
	ProjectionTypeMCPContext       = "mcp_context"
	ProjectionTypeToolGatewayToken = "tool_gateway_token"

	ApprovalTargetExpansion  = "expansion_request"
	ApprovalAppliesExpansion = "expansion"

	defaultProjectionTTLSeconds = 300
	defaultCredentialTTLSeconds = 120
	defaultLeaseTTLSeconds      = 60
	maxCredentialTTLSeconds     = 300
	maxLeaseTTLSeconds          = 300
)

type Projection struct {
	ProjectionID    string                     `json:"projection_id"`
	MissionRef      string                     `json:"mission_ref"`
	MissionVersion  int                        `json:"mission_version"`
	TenantID        string                     `json:"tenant_id,omitempty"`
	Type            string                     `json:"type"`
	Actor           Actor                      `json:"actor"`
	Scopes          []string                   `json:"scopes,omitempty"`
	Audience        string                     `json:"audience,omitempty"`
	ToolName        string                     `json:"tool_name,omitempty"`
	Resource        *ActionResource            `json:"resource,omitempty"`
	Operation       string                     `json:"operation,omitempty"`
	Claims          map[string]any             `json:"claims,omitempty"`
	Token           string                     `json:"token,omitempty"`
	Status          string                     `json:"status"`
	IssuedAt        time.Time                  `json:"issued_at"`
	ExpiresAt       time.Time                  `json:"expires_at"`
	RevokedAt       time.Time                  `json:"revoked_at,omitempty"`
	ExchangeRecords []CredentialExchangeRecord `json:"exchange_records,omitempty"`
}

type ProjectionPayload struct {
	JTI            string            `json:"jti,omitempty"`
	Issuer         string            `json:"iss,omitempty"`
	Subject        string            `json:"sub,omitempty"`
	Audience       string            `json:"aud,omitempty"`
	TokenUse       string            `json:"token_use,omitempty"`
	ProjectionID   string            `json:"projection_id"`
	MissionRef     string            `json:"mission_ref"`
	MissionVersion int               `json:"mission_version"`
	TenantID       string            `json:"tenant_id,omitempty"`
	Type           string            `json:"type"`
	Actor          Actor             `json:"actor"`
	Agent          Agent             `json:"agent"`
	AuthorityHash  string            `json:"authority_hash"`
	Scopes         []string          `json:"scope,omitempty"`
	ToolName       string            `json:"tool_name,omitempty"`
	Resource       *ActionResource   `json:"resource,omitempty"`
	Operation      string            `json:"operation,omitempty"`
	Confirmation   map[string]string `json:"cnf,omitempty"`
	Claims         map[string]any    `json:"claims,omitempty"`
	IssuedAt       time.Time         `json:"issued_at"`
	NotBefore      time.Time         `json:"not_before,omitempty"`
	ExpiresAt      time.Time         `json:"expires_at"`
}

type CreateProjectionRequest struct {
	MissionVersionSeen int             `json:"mission_version_seen,omitempty"`
	Actor              Actor           `json:"actor"`
	Type               string          `json:"type"`
	Scopes             []string        `json:"scopes,omitempty"`
	Audience           string          `json:"audience,omitempty"`
	ToolName           string          `json:"tool_name,omitempty"`
	Resource           *ActionResource `json:"resource,omitempty"`
	Operation          string          `json:"operation,omitempty"`
	Claims             map[string]any  `json:"claims,omitempty"`
	TTLSeconds         int             `json:"ttl_seconds,omitempty"`
}

type ProjectionResponse struct {
	ProjectionID   string          `json:"projection_id"`
	MissionRef     string          `json:"mission_ref"`
	MissionVersion int             `json:"mission_version"`
	Type           string          `json:"type"`
	Status         string          `json:"status"`
	Scopes         []string        `json:"scopes,omitempty"`
	Audience       string          `json:"audience,omitempty"`
	ToolName       string          `json:"tool_name,omitempty"`
	Resource       *ActionResource `json:"resource,omitempty"`
	Operation      string          `json:"operation,omitempty"`
	Token          string          `json:"token,omitempty"`
	ExpiresAt      time.Time       `json:"expires_at"`
}

type ProjectionStatusResponse struct {
	ProjectionID   string    `json:"projection_id"`
	MissionRef     string    `json:"mission_ref,omitempty"`
	MissionVersion int       `json:"mission_version,omitempty"`
	TenantID       string    `json:"tenant_id,omitempty"`
	Type           string    `json:"type,omitempty"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
	RevokedAt      time.Time `json:"revoked_at,omitempty"`
	ExchangeCount  int       `json:"exchange_count,omitempty"`
}

type VerifyProjectionRequest struct {
	Token string `json:"token"`
}

type VerifyProjectionResponse struct {
	Valid      bool              `json:"valid"`
	Payload    ProjectionPayload `json:"payload,omitempty"`
	Projection *Projection       `json:"projection,omitempty"`
	Error      string            `json:"error,omitempty"`
}

type CredentialExchangeRecord struct {
	ExchangeID      string          `json:"exchange_id"`
	ProjectionID    string          `json:"projection_id"`
	JTI             string          `json:"jti"`
	Nonce           string          `json:"nonce"`
	Actor           Actor           `json:"actor"`
	Scopes          []string        `json:"scopes,omitempty"`
	Audience        string          `json:"audience,omitempty"`
	ToolName        string          `json:"tool_name,omitempty"`
	Resource        *ActionResource `json:"resource,omitempty"`
	Operation       string          `json:"operation,omitempty"`
	AccessTokenHash string          `json:"access_token_hash,omitempty"`
	Status          string          `json:"status"`
	IssuedAt        time.Time       `json:"issued_at"`
	ExpiresAt       time.Time       `json:"expires_at"`
	RevokedAt       time.Time       `json:"revoked_at,omitempty"`
}

type ExchangeProjectionTokenRequest struct {
	ProjectionToken string          `json:"projection_token"`
	Actor           Actor           `json:"actor"`
	Nonce           string          `json:"nonce"`
	RequestedScopes []string        `json:"requested_scopes,omitempty"`
	Audience        string          `json:"audience,omitempty"`
	ToolName        string          `json:"tool_name,omitempty"`
	Resource        *ActionResource `json:"resource,omitempty"`
	Operation       string          `json:"operation,omitempty"`
	TTLSeconds      int             `json:"ttl_seconds,omitempty"`
}

type CredentialAccessTokenResponse struct {
	AccessToken    string          `json:"access_token"`
	TokenType      string          `json:"token_type"`
	ExchangeID     string          `json:"exchange_id"`
	JTI            string          `json:"jti"`
	ProjectionID   string          `json:"projection_id"`
	MissionRef     string          `json:"mission_ref"`
	MissionVersion int             `json:"mission_version"`
	TenantID       string          `json:"tenant_id,omitempty"`
	Actor          Actor           `json:"actor"`
	Scopes         []string        `json:"scopes,omitempty"`
	Audience       string          `json:"audience,omitempty"`
	ToolName       string          `json:"tool_name,omitempty"`
	Resource       *ActionResource `json:"resource,omitempty"`
	Operation      string          `json:"operation,omitempty"`
	ExpiresIn      int             `json:"expires_in"`
	ExpiresAt      time.Time       `json:"expires_at"`
}

type CredentialAccessTokenPayload struct {
	JTI            string            `json:"jti"`
	Issuer         string            `json:"iss"`
	Subject        string            `json:"sub"`
	Audience       string            `json:"aud,omitempty"`
	TokenUse       string            `json:"token_use"`
	ExchangeID     string            `json:"exchange_id"`
	ProjectionID   string            `json:"projection_id"`
	MissionRef     string            `json:"mission_ref"`
	MissionVersion int               `json:"mission_version"`
	TenantID       string            `json:"tenant_id,omitempty"`
	Type           string            `json:"type"`
	Actor          Actor             `json:"actor"`
	Agent          Agent             `json:"agent"`
	AuthorityHash  string            `json:"authority_hash"`
	Scopes         []string          `json:"scope,omitempty"`
	ToolName       string            `json:"tool_name,omitempty"`
	Resource       *ActionResource   `json:"resource,omitempty"`
	Operation      string            `json:"operation,omitempty"`
	Confirmation   map[string]string `json:"cnf,omitempty"`
	IssuedAt       time.Time         `json:"issued_at"`
	NotBefore      time.Time         `json:"not_before,omitempty"`
	ExpiresAt      time.Time         `json:"expires_at"`
}

type VerifyCredentialAccessTokenRequest struct {
	Token     string          `json:"token"`
	Actor     Actor           `json:"actor,omitempty"`
	Audience  string          `json:"audience,omitempty"`
	ToolName  string          `json:"tool_name,omitempty"`
	Resource  *ActionResource `json:"resource,omitempty"`
	Operation string          `json:"operation,omitempty"`
}

type VerifyCredentialAccessTokenResponse struct {
	Valid      bool                         `json:"valid"`
	Payload    CredentialAccessTokenPayload `json:"payload,omitempty"`
	Projection *Projection                  `json:"projection,omitempty"`
	Exchange   *CredentialExchangeRecord    `json:"exchange,omitempty"`
	Error      string                       `json:"error,omitempty"`
}

type MissionLease struct {
	LeaseID        string    `json:"lease_id"`
	MissionRef     string    `json:"mission_ref"`
	MissionVersion int       `json:"mission_version"`
	TenantID       string    `json:"tenant_id,omitempty"`
	Actor          Actor     `json:"actor"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	RefreshedAt    time.Time `json:"refreshed_at,omitempty"`
}

type CreateLeaseRequest struct {
	MissionVersionSeen int   `json:"mission_version_seen,omitempty"`
	Actor              Actor `json:"actor"`
	TTLSeconds         int   `json:"ttl_seconds,omitempty"`
}

type RefreshLeaseRequest struct {
	Actor      Actor `json:"actor"`
	TTLSeconds int   `json:"ttl_seconds,omitempty"`
}

type LeaseResponse struct {
	LeaseID        string    `json:"lease_id,omitempty"`
	MissionRef     string    `json:"mission_ref,omitempty"`
	MissionVersion int       `json:"mission_version,omitempty"`
	Decision       Decision  `json:"decision"`
	ReasonCodes    []string  `json:"reason_codes,omitempty"`
	HumanReason    string    `json:"human_reason,omitempty"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
}

type ApprovalRule struct {
	RuleID            string    `json:"rule_id"`
	TenantID          string    `json:"tenant_id,omitempty"`
	AppliesTo         string    `json:"applies_to"`
	ResourceType      string    `json:"resource_type,omitempty"`
	ResourceID        string    `json:"resource_id,omitempty"`
	Operation         string    `json:"operation,omitempty"`
	RiskLevel         string    `json:"risk_level,omitempty"`
	RequiredApprovals int       `json:"required_approvals"`
	AllowedSubjects   []string  `json:"allowed_subjects,omitempty"`
	AllowedIssuers    []string  `json:"allowed_issuers,omitempty"`
	CreatedBy         Principal `json:"created_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type ApprovalRecord struct {
	ApprovalID       string           `json:"approval_id"`
	RuleID           string           `json:"rule_id,omitempty"`
	TargetType       string           `json:"target_type"`
	TargetID         string           `json:"target_id"`
	TenantID         string           `json:"tenant_id,omitempty"`
	Approver         Principal        `json:"approver"`
	ApprovalEvidence ApprovalEvidence `json:"approval_evidence,omitempty"`
	Reason           string           `json:"reason,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
}

type SubmitExpansionApprovalRequest struct {
	Approver         Principal        `json:"approver"`
	ApprovalEvidence ApprovalEvidence `json:"approval_evidence,omitempty"`
	Reason           string           `json:"reason,omitempty"`
}

type SubmitExpansionApprovalResponse struct {
	ExpansionID       string `json:"expansion_id"`
	Status            string `json:"status"`
	RuleID            string `json:"rule_id,omitempty"`
	ApprovalsRequired int    `json:"approvals_required"`
	ApprovalsReceived int    `json:"approvals_received"`
	MissionRef        string `json:"mission_ref"`
	MissionVersion    int    `json:"mission_version,omitempty"`
}

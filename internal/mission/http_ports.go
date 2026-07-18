package mission

import "context"

type IdentityAPI interface {
	RegisterAgent(RegisterAgentRequest) (RegisterAgentResponse, error)
	GetAgentIdentity(string) (AgentIdentity, error)
	RevokeAgent(string, StateChangeRequest) (AgentIdentity, error)
	VerifyAgentRequestSignature(string, string, []byte, string, string, string) (AgentIdentity, error)
}

type MissionAPI interface {
	CreateProposal(CreateProposalRequest) (CreateProposalResponse, error)
	ApproveProposal(string, ApproveProposalRequest) (ApproveProposalResponse, error)
	Evaluate(string, EvaluateRequest) (EvaluateResponse, error)
	Resume(string, ResumeRequest) (EvaluateResponse, error)
	Delegate(string, DelegationRequest) (DelegationResponse, error)
	Revoke(string, StateChangeRequest) (Mission, error)
	Complete(string, StateChangeRequest) (Mission, error)
	Introspect(string) (Mission, error)
	Events() []Event
}

type GovernanceAPI interface {
	CreateExpansionRequest(string, CreateExpansionRequest) (ExpansionRequest, error)
	GetExpansionRequest(string) (ExpansionRequest, error)
	ApproveExpansionRequest(string, ExpansionDecisionRequest) (ExpansionDecisionResponse, error)
	ApproveExpansionRequestContext(context.Context, string, ExpansionDecisionRequest) (ExpansionDecisionResponse, error)
	DenyExpansionRequest(string, ExpansionDecisionRequest) (ExpansionDecisionResponse, error)
	DenyExpansionRequestContext(context.Context, string, ExpansionDecisionRequest) (ExpansionDecisionResponse, error)
	VerifyDecisionArtifactEvidence(VerifyDecisionArtifactRequest) VerifyDecisionArtifactResponse
	RegisterToolContract(ToolContract) (ToolContract, error)
	GetToolContract(string) (ToolContract, error)
	AuthorizeToolCall(AuthorizeToolCallRequest) (AuthorizeToolCallResponse, error)
}

type ProjectionAPI interface {
	CreateProjection(string, CreateProjectionRequest) (ProjectionResponse, error)
	GetProjectionStatus(string) (ProjectionStatusResponse, error)
	RevokeProjection(string, StateChangeRequest) (ProjectionStatusResponse, error)
	VerifyProjection(VerifyProjectionRequest) VerifyProjectionResponse
	CreateMissionLease(string, CreateLeaseRequest) (LeaseResponse, error)
	RefreshMissionLease(string, RefreshLeaseRequest) (LeaseResponse, error)
	CreateApprovalRule(ApprovalRule) (ApprovalRule, error)
	ListApprovalRules() ([]ApprovalRule, error)
	SubmitExpansionApproval(string, SubmitExpansionApprovalRequest) (SubmitExpansionApprovalResponse, error)
	SubmitExpansionApprovalContext(context.Context, string, SubmitExpansionApprovalRequest) (SubmitExpansionApprovalResponse, error)
}

type GrandGovernanceAPI interface {
	CreateAuthorityNegotiation(string, CreateAuthorityNegotiationRequest) (AuthorityNegotiation, error)
	GetAuthorityNegotiation(string) (AuthorityNegotiation, error)
	CreateContainmentRule(ContainmentRule) (ContainmentRule, error)
	ListContainmentRules() ([]ContainmentRule, error)
	LiftContainmentRule(string, StateChangeRequest) (ContainmentRule, error)
	ContainmentBlastRadiusContext(context.Context, string) (BlastRadius, error)
	MissionLineageContext(context.Context, string) (LineageGraph, error)
	AgentLineageContext(context.Context, string) (LineageGraph, error)
}

type AuthZENAPI interface {
	EvaluateAuthZEN(AuthZENEvaluationRequest) (AuthZENEvaluationResponse, error)
	EvaluateAuthZENBatch(AuthZENEvaluationsRequest) (AuthZENEvaluationsResponse, error)
}

type HandlerServices struct {
	Identity        IdentityAPI
	Mission         MissionAPI
	Governance      GovernanceAPI
	Projection      ProjectionAPI
	GrandGovernance GrandGovernanceAPI
	AuthZEN         AuthZENAPI
}

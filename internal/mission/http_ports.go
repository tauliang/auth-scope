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
	CreatePolicyBundle(CreatePolicyBundleRequest, Principal) (PolicyBundle, error)
	GetPolicyBundle(string) (PolicyBundle, error)
	ListPolicyBundles() ([]PolicyBundle, error)
	ActivatePolicyBundle(string, ActivatePolicyBundleRequest, Principal) (PolicyBundle, error)
	SimulatePolicyBundle(string, SimulatePolicyBundleRequest) (SimulatePolicyBundleResponse, error)
}

type ProjectionAPI interface {
	CreateProjection(string, CreateProjectionRequest) (ProjectionResponse, error)
	GetProjectionStatus(string) (ProjectionStatusResponse, error)
	RevokeProjection(string, StateChangeRequest) (ProjectionStatusResponse, error)
	VerifyProjection(VerifyProjectionRequest) VerifyProjectionResponse
	ExchangeProjectionToken(ExchangeProjectionTokenRequest) (CredentialAccessTokenResponse, error)
	VerifyCredentialAccessToken(VerifyCredentialAccessTokenRequest) VerifyCredentialAccessTokenResponse
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

type OperatorAPI interface {
	OperationsSummary(ListQuery) (OperationsSummary, error)
	ListMissions(ListQuery) (CollectionPage[Mission], error)
	ListProposals(ListQuery) (CollectionPage[MissionProposal], error)
	GetProposal(string) (MissionProposal, error)
	ListExpansions(ListQuery) (CollectionPage[ExpansionRequest], error)
	ListAgents(ListQuery) (CollectionPage[AgentIdentity], error)
	ListToolContracts(ListQuery) (CollectionPage[ToolContract], error)
	ListProjections(ListQuery) (CollectionPage[Projection], error)
	GetContainmentRule(string) (ContainmentRule, error)
	ListEvents(ListQuery) (CollectionPage[Event], error)
}

type GitHubAPI interface {
	CreateGitHubRepositoryBinding(CreateGitHubRepositoryBindingRequest, Principal) (GitHubRepositoryBinding, error)
	ListGitHubRepositoryBindings() ([]GitHubRepositoryBinding, error)
	IngestGitHubWebhook(IngestGitHubWebhookRequest) (GitHubWebhookResponse, error)
	PlanGitHubCheckRun(GitHubCheckRunPlanRequest) (GitHubCheckRunPlanResponse, error)
}

type OktaAPI interface {
	CreateOktaAppBinding(CreateOktaAppBindingRequest, Principal) (OktaAppBinding, error)
	ListOktaAppBindings() ([]OktaAppBinding, error)
	ResolveOktaAuthorityContext(ResolveOktaAuthorityContextRequest) (OktaAuthorityContextResponse, error)
}

type EntraAPI interface {
	CreateEntraAppRegistration(CreateEntraAppRegistrationRequest, Principal) (EntraAppRegistration, error)
	ListEntraAppRegistrations() ([]EntraAppRegistration, error)
	ResolveEntraAuthorityContext(ResolveEntraAuthorityContextRequest) (EntraAuthorityContextResponse, error)
}

type SlackAPI interface {
	CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest, Principal) (SlackWorkspaceBinding, error)
	ListSlackWorkspaceBindings() ([]SlackWorkspaceBinding, error)
	AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest) (SlackMessageAuthorizationResponse, error)
}

type AtlassianAPI interface {
	CreateAtlassianSiteBinding(CreateAtlassianSiteBindingRequest, Principal) (AtlassianSiteBinding, error)
	ListAtlassianSiteBindings() ([]AtlassianSiteBinding, error)
	AuthorizeAtlassianJiraIssueAction(AuthorizeAtlassianJiraIssueActionRequest) (AtlassianActionAuthorizationResponse, error)
	AuthorizeAtlassianConfluencePageAction(AuthorizeAtlassianConfluencePageActionRequest) (AtlassianActionAuthorizationResponse, error)
}

type SalesforceAPI interface {
	CreateSalesforceOrgBinding(CreateSalesforceOrgBindingRequest, Principal) (SalesforceOrgBinding, error)
	ListSalesforceOrgBindings() ([]SalesforceOrgBinding, error)
	AuthorizeSalesforceRecordAction(AuthorizeSalesforceRecordActionRequest) (SalesforceRecordActionAuthorizationResponse, error)
}

type HandlerServices struct {
	Identity        IdentityAPI
	Mission         MissionAPI
	Governance      GovernanceAPI
	Projection      ProjectionAPI
	GrandGovernance GrandGovernanceAPI
	AuthZEN         AuthZENAPI
	Operator        OperatorAPI
	GitHub          GitHubAPI
	Okta            OktaAPI
	Entra           EntraAPI
	Slack           SlackAPI
	Atlassian       AtlassianAPI
	Salesforce      SalesforceAPI
	AdminAudit      AdminAuditAPI
}

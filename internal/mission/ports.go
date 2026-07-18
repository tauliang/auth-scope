package mission

import "context"

type IdentityStore interface {
	SaveAgentIdentity(AgentIdentity) error
	GetAgentIdentity(string) (AgentIdentity, error)
	UpdateAgentIdentity(AgentIdentity) error
	ListAgentIdentities() ([]AgentIdentity, error)
	SaveAgentNonce(AgentNonce) error
}

type MissionStore interface {
	SaveProposal(MissionProposal) error
	GetProposal(string) (MissionProposal, error)
	ListProposals() ([]MissionProposal, error)
	DeleteProposal(string) error
	SaveMission(Mission) error
	GetMission(string) (Mission, error)
	UpdateMission(Mission) error
	ChildrenOf(string) ([]Mission, error)
	ListMissions() ([]Mission, error)
}

type GovernanceStore interface {
	SaveExpansionRequest(ExpansionRequest) error
	GetExpansionRequest(string) (ExpansionRequest, error)
	UpdateExpansionRequest(ExpansionRequest) error
	ListExpansionRequests() ([]ExpansionRequest, error)
	SaveEvaluationEvidence(EvaluationEvidence) error
	GetEvaluationEvidence(string) (EvaluationEvidence, error)
	SaveToolContract(ToolContract) error
	GetToolContract(string) (ToolContract, error)
	ListToolContracts() ([]ToolContract, error)
}

type ProjectionStore interface {
	SaveProjection(Projection) error
	GetProjection(string) (Projection, error)
	UpdateProjection(Projection) error
	ListProjections() ([]Projection, error)
	SaveMissionLease(MissionLease) error
	GetMissionLease(string) (MissionLease, error)
	UpdateMissionLease(MissionLease) error
	ListMissionLeases() ([]MissionLease, error)
}

type ApprovalStore interface {
	SaveApprovalRule(ApprovalRule) error
	ListApprovalRules() ([]ApprovalRule, error)
	SaveApprovalRecord(ApprovalRecord) error
	ListApprovalRecords(string, string) ([]ApprovalRecord, error)
}

type NegotiationStore interface {
	SaveAuthorityNegotiation(AuthorityNegotiation) error
	GetAuthorityNegotiation(string) (AuthorityNegotiation, error)
}

type ContainmentStore interface {
	SaveContainmentRule(ContainmentRule) error
	GetContainmentRule(string) (ContainmentRule, error)
	UpdateContainmentRule(ContainmentRule) error
	ListContainmentRules() ([]ContainmentRule, error)
}

type ExpansionDecisionStore interface {
	CommitExpansionDecision(context.Context, ExpansionDecisionCommit) error
}

type EventStore interface {
	AppendEvent(Event) error
	Events() []Event
}

type OutboxStore interface {
	PublishOutboxEvents() ([]OutboxEvent, error)
}

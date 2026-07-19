package mission

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeServiceNowStore struct {
	bindings map[string]ServiceNowTicketBinding
}

func newFakeServiceNowStore() *fakeServiceNowStore {
	return &fakeServiceNowStore{bindings: map[string]ServiceNowTicketBinding{}}
}

func (s *fakeServiceNowStore) SaveServiceNowTicketBinding(binding ServiceNowTicketBinding) error {
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *fakeServiceNowStore) GetServiceNowTicketBinding(id string) (ServiceNowTicketBinding, error) {
	binding, ok := s.bindings[id]
	if !ok {
		return ServiceNowTicketBinding{}, ErrNotFound
	}
	return binding, nil
}

func (s *fakeServiceNowStore) GetServiceNowTicketBindingByMissionRefAndSysID(missionRef string, sysID string) (ServiceNowTicketBinding, error) {
	for _, binding := range s.bindings {
		if binding.MissionRef == missionRef && binding.ServiceNowSysID == sysID {
			return binding, nil
		}
	}
	return ServiceNowTicketBinding{}, ErrNotFound
}

func (s *fakeServiceNowStore) ListServiceNowTicketBindings() ([]ServiceNowTicketBinding, error) {
	bindings := make([]ServiceNowTicketBinding, 0, len(s.bindings))
	for _, binding := range s.bindings {
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func (s *fakeServiceNowStore) UpdateServiceNowTicketBinding(binding ServiceNowTicketBinding) error {
	if _, ok := s.bindings[binding.BindingID]; !ok {
		return ErrNotFound
	}
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *fakeServiceNowStore) DeleteServiceNowTicketBinding(id string) error {
	if _, ok := s.bindings[id]; !ok {
		return ErrNotFound
	}
	delete(s.bindings, id)
	return nil
}

func newServiceWithServiceNowStore(snStore ServiceNowStore) (*Service, *MemoryStore) {
	base := NewMemoryStore()
	service := NewServiceWithDependencies(ServiceDependencies{
		Identities:         base,
		Missions:           base,
		Governance:         base,
		Projections:        base,
		Approvals:          base,
		Negotiations:       base,
		Containments:       base,
		ExpansionDecisions: base,
		ProposalApprovals:  base,
		Events:             base,
		GitHub:             base,
		Okta:               base,
		Entra:              base,
		Slack:              base,
		ServiceNow:         snStore,
		GovernanceReads:    base,
		ArtifactKey:        []byte("0123456789abcdef0123456789abcdef"),
	}, fixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)})
	return service, base
}

func TestServiceNowMissionAdapterLifecycleAndEvaluation(t *testing.T) {
	ctx := context.Background()
	snStore := newFakeServiceNowStore()
	service, baseStore := newServiceWithServiceNowStore(snStore)

	req := validProposalRequest()
	req.Intent = Purpose{Objective: "Govern ServiceNow change execution"}
	req.AuthorityRegion = AuthorityRegion{
		Resources:        []ResourceGrant{{Type: "servicenow_ticket", ID: "sys_123", Actions: []string{"update"}}},
		ForbiddenActions: []string{"delete"},
	}
	proposal, err := service.CreateProposal(req)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	mission, err := service.ApproveProposal(proposal.ProposalID, ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}

	binding, err := service.CreateServiceNowTicketBinding(ctx, CreateServiceNowTicketBindingRequest{
		TenantID:        "demo",
		InstanceURL:     "https://acme.service-now.com",
		ServiceNowSysID: "sys_123",
		State:           "new",
		MissionRef:      mission.MissionRef,
		AssignmentGroup: "Change Approvers",
		CallerID:        "agent@example.com",
		RequiredGroups:  []string{"Change Approvers"},
	}, Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateServiceNowTicketBinding: %v", err)
	}
	if binding.ServiceNowSysID != "sys_123" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected ServiceNow binding: %#v", binding)
	}
	if len(baseStore.Events()) == 0 || baseStore.Events()[len(baseStore.Events())-1].Type != "ticket_binding.created" {
		t.Fatalf("expected ServiceNow create event, got %#v", baseStore.Events())
	}

	got, err := service.GetServiceNowTicketBinding(ctx, binding.BindingID)
	if err != nil {
		t.Fatalf("GetServiceNowTicketBinding: %v", err)
	}
	if got.BindingID != binding.BindingID {
		t.Fatalf("unexpected fetched binding: %#v", got)
	}
	list, err := service.ListServiceNowTicketBindings(ctx)
	if err != nil {
		t.Fatalf("ListServiceNowTicketBindings: %v", err)
	}
	if len(list) != 1 || list[0].BindingID != binding.BindingID {
		t.Fatalf("unexpected ServiceNow binding list: %#v", list)
	}

	updated, err := service.UpdateServiceNowTicketStatus(ctx, binding.BindingID, "in_progress")
	if err != nil {
		t.Fatalf("UpdateServiceNowTicketStatus: %v", err)
	}
	if updated.State != "in_progress" || updated.LastResolvedAt.IsZero() {
		t.Fatalf("unexpected status update: %#v", updated)
	}

	resolved, err := service.ResolveServiceNowAuthorityContext(ctx, ResolveServiceNowAuthorityContextRequest{
		TenantID:   "demo",
		MissionRef: mission.MissionRef,
		Subject:    "agent@example.com",
		Groups:     []string{"Change Approvers"},
		Context:    map[string]any{"finance.close.status": "open"},
		Evaluation: &ServiceNowEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              ServiceNowActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: ServiceNowEvaluationAction{
				Type:      "ticket_change",
				Resource:  ServiceNowEvaluationActionResource{Type: "servicenow_ticket", ID: "sys_123"},
				Operation: "update",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveServiceNowAuthorityContext: %v", err)
	}
	if !resolved.Accepted || resolved.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected authority context response: %#v", resolved)
	}

	if err := service.DeleteServiceNowTicketBinding(ctx, binding.BindingID); err != nil {
		t.Fatalf("DeleteServiceNowTicketBinding: %v", err)
	}
	if _, err := service.GetServiceNowTicketBinding(ctx, binding.BindingID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected deleted binding lookup to return ErrNotFound, got %v", err)
	}
}

func TestServiceNowMissionAdapterConversions(t *testing.T) {
	principal := serviceNowPrincipal(Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"})
	if principal.Subject != "alice@example.com" || principal.Issuer != "https://idp.example.com" {
		t.Fatalf("unexpected principal conversion: %#v", principal)
	}

	actor := missionActorFromServiceNow(ServiceNowActor{AgentInstanceID: "inst_123", ClientID: "agent", KeyThumbprint: "thumb"})
	if actor.AgentInstanceID != "inst_123" || actor.ClientID != "agent" || actor.KeyThumbprint != "thumb" {
		t.Fatalf("unexpected actor conversion: %#v", actor)
	}

	resource := actionResourceFromServiceNow(ServiceNowEvaluationActionResource{Type: "servicenow_ticket", ID: "sys_123"})
	if resource.Type != "servicenow_ticket" || resource.ID != "sys_123" {
		t.Fatalf("unexpected resource conversion: %#v", resource)
	}
}

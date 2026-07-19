package mission

import (
	"errors"
	"testing"
)

func TestOktaAppBindingResolvesAuthorityContextAndEvaluatesAction(t *testing.T) {
	service := testService()
	mission := approveOktaMission(t, service)

	binding, err := service.CreateOktaAppBinding(CreateOktaAppBindingRequest{
		Issuer:          "https://acme.okta.com/oauth2/default/",
		ClientID:        "0oaabc123client",
		AppID:           "0oaapp123",
		AppLabel:        "Auth Scope Console",
		MissionRef:      mission.MissionRef,
		RequiredGroups:  []string{"Mission Operators"},
		AdminGroups:     []string{"Mission Admins"},
		AllowedSubjects: []string{"00u1agent"},
		Metadata:        map[string]string{"environment": "demo"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateOktaAppBinding: %v", err)
	}
	if binding.Issuer != "https://acme.okta.com/oauth2/default" || binding.AuthorizationServerID != "default" {
		t.Fatalf("unexpected issuer normalization: %#v", binding)
	}
	if binding.DiscoveryURL != "https://acme.okta.com/oauth2/default/.well-known/openid-configuration" {
		t.Fatalf("unexpected discovery URL: %s", binding.DiscoveryURL)
	}
	if binding.JWKSURI != "https://acme.okta.com/oauth2/default/v1/keys" {
		t.Fatalf("unexpected jwks URI: %s", binding.JWKSURI)
	}
	if _, err := service.CreateOktaAppBinding(CreateOktaAppBindingRequest{
		Issuer:     "https://acme.okta.com/oauth2/default",
		ClientID:   "0oaabc123client",
		MissionRef: mission.MissionRef,
	}, Principal{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate binding err = %v, want ErrConflict", err)
	}

	resp, err := service.ResolveOktaAuthorityContext(ResolveOktaAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://acme.okta.com/oauth2/default",
			"cid":    "0oaabc123client",
			"sub":    "00u1agent",
			"groups": []any{"Mission Operators", "Mission Admins"},
			"scp":    []any{"openid", "groups"},
		},
		Context: map[string]any{"risk": "low", "reversible": true},
		Evaluation: &OktaEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              OktaActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: OktaEvaluationAction{
				Type:      "tool_call",
				Resource:  OktaEvaluationActionResource{Type: "drive_folder", ID: "board"},
				Operation: "read",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveOktaAuthorityContext: %v", err)
	}
	if !resp.Accepted || !resp.Admin || resp.BindingID != binding.BindingID || resp.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected accepted response: %#v", resp)
	}
	if resp.Context["okta.client_id"] != "0oaabc123client" || resp.Context["risk"] != "low" {
		t.Fatalf("unexpected context: %#v", resp.Context)
	}
	if resp.Evaluation == nil || resp.Evaluation.Decision != string(DecisionAllow) || resp.Evaluation.DecisionArtifact == "" {
		t.Fatalf("unexpected evaluation: %#v", resp.Evaluation)
	}
	updated, err := service.okta.GetOktaAppBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetOktaAppBinding: %v", err)
	}
	if updated.LastSubject != "00u1agent" || updated.LastResolutionStatus != OktaResolutionStatusAccepted {
		t.Fatalf("binding resolution fields = %#v", updated)
	}
}

func TestOktaAuthorityContextDeniesMissingRequiredGroup(t *testing.T) {
	service := testService()
	mission := approveOktaMission(t, service)
	if _, err := service.CreateOktaAppBinding(CreateOktaAppBindingRequest{
		Issuer:         "https://acme.okta.com",
		ClientID:       "0oaabc123client",
		MissionRef:     mission.MissionRef,
		RequiredGroups: []string{"Mission Operators"},
	}, Principal{}); err != nil {
		t.Fatalf("CreateOktaAppBinding: %v", err)
	}

	resp, err := service.ResolveOktaAuthorityContext(ResolveOktaAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://acme.okta.com",
			"aud":    "0oaabc123client",
			"sub":    "00u1agent",
			"groups": []any{"Everyone"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveOktaAuthorityContext: %v", err)
	}
	if resp.Accepted || resp.Status != OktaResolutionStatusDenied || resp.ReasonCodes[0] != "okta_required_group_missing" {
		t.Fatalf("unexpected denial: %#v", resp)
	}
}

func approveOktaMission(t *testing.T, service *Service) ApproveProposalResponse {
	t.Helper()
	req := validProposalRequest()
	req.Intent = Purpose{Objective: "Govern Okta-authenticated agent work"}
	req.Conditions = nil
	req.AuthorityRegion = AuthorityRegion{
		Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
		ForbiddenActions: []string{"send_external"},
	}
	proposal, err := service.CreateProposal(req)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	mission, err := service.ApproveProposal(proposal.ProposalID, ApproveProposalRequest{
		Approver: Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}
	return mission
}

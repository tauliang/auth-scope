package mission

import (
	"errors"
	"testing"
)

func TestEntraAppRegistrationResolvesAuthorityContextAndEvaluatesAction(t *testing.T) {
	service := testService()
	mission := approveEntraMission(t, service)

	registration, err := service.CreateEntraAppRegistration(CreateEntraAppRegistrationRequest{
		Issuer:          "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
		ClientID:        "00000000-0000-0000-0000-000000000000",
		AppID:           "app_entra_001",
		AppName:         "Auth Scope Console",
		MissionRef:      mission.MissionRef,
		RequiredGroups:  []string{"Mission Operators"},
		AdminGroups:     []string{"Mission Admins"},
		AllowedSubjects: []string{"user@example.onmicrosoft.com"},
		Metadata:        map[string]string{"environment": "demo"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateEntraAppRegistration: %v", err)
	}
	if registration.Issuer != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0" {
		t.Fatalf("unexpected issuer normalization: %#v", registration)
	}
	if registration.DiscoveryURL != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0/.well-known/openid-configuration" {
		t.Fatalf("unexpected discovery URL: %s", registration.DiscoveryURL)
	}
	if registration.JWKSURI != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0/discovery/v2.0/keys" {
		t.Fatalf("unexpected jwks URI: %s", registration.JWKSURI)
	}
	if _, err := service.CreateEntraAppRegistration(CreateEntraAppRegistrationRequest{
		Issuer:     "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
		ClientID:   "00000000-0000-0000-0000-000000000000",
		MissionRef: mission.MissionRef,
	}, Principal{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate registration err = %v, want ErrConflict", err)
	}

	resp, err := service.ResolveEntraAuthorityContext(ResolveEntraAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"appid":  "00000000-0000-0000-0000-000000000000",
			"sub":    "user@example.onmicrosoft.com",
			"groups": []any{"Mission Operators", "Mission Admins"},
			"roles":  []any{"Reader", "Contributor"},
		},
		Context: map[string]any{"risk": "low", "reversible": true},
		Evaluation: &EntraEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              EntraActor{AgentInstanceID: "inst_456", ClientID: "research-agent"},
			Action: EntraEvaluationAction{
				Type:      "tool_call",
				Resource:  EntraEvaluationActionResource{Type: "drive_folder", ID: "board"},
				Operation: "read",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveEntraAuthorityContext: %v", err)
	}
	if !resp.Accepted || !resp.Admin || resp.RegistrationID != registration.RegistrationID || resp.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected accepted response: %#v", resp)
	}
	if resp.Context["entra.client_id"] != "00000000-0000-0000-0000-000000000000" || resp.Context["risk"] != "low" {
		t.Fatalf("unexpected context: %#v", resp.Context)
	}
	if len(resp.Groups) == 0 || len(resp.Roles) == 0 {
		t.Fatalf("unexpected groups/roles: groups=%v, roles=%v", resp.Groups, resp.Roles)
	}
	if resp.Evaluation == nil || resp.Evaluation.Decision != string(DecisionAllow) || resp.Evaluation.DecisionArtifact == "" {
		t.Fatalf("unexpected evaluation: %#v", resp.Evaluation)
	}
	updated, err := service.entra.GetEntraAppRegistration(registration.RegistrationID)
	if err != nil {
		t.Fatalf("GetEntraAppRegistration: %v", err)
	}
	if updated.LastSubject != "user@example.onmicrosoft.com" || updated.LastResolutionStatus != EntraResolutionStatusAccepted {
		t.Fatalf("registration resolution fields = %#v", updated)
	}
}

func TestEntraAuthorityContextDeniesMissingRequiredGroup(t *testing.T) {
	service := testService()
	mission := approveEntraMission(t, service)
	if _, err := service.CreateEntraAppRegistration(CreateEntraAppRegistrationRequest{
		Issuer:         "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
		ClientID:       "00000000-0000-0000-0000-000000000000",
		MissionRef:     mission.MissionRef,
		RequiredGroups: []string{"Mission Operators"},
	}, Principal{}); err != nil {
		t.Fatalf("CreateEntraAppRegistration: %v", err)
	}

	resp, err := service.ResolveEntraAuthorityContext(ResolveEntraAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"aud":    "00000000-0000-0000-0000-000000000000",
			"sub":    "user@example.onmicrosoft.com",
			"groups": []any{"Everyone"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveEntraAuthorityContext: %v", err)
	}
	if resp.Accepted || resp.Status != EntraResolutionStatusDenied || resp.ReasonCodes[0] != "entra_required_group_missing" {
		t.Fatalf("unexpected denial: %#v", resp)
	}
}

func TestEntraAuthorityContextDeniesMissingAllowedSubject(t *testing.T) {
	service := testService()
	mission := approveEntraMission(t, service)
	if _, err := service.CreateEntraAppRegistration(CreateEntraAppRegistrationRequest{
		Issuer:          "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
		ClientID:        "00000000-0000-0000-0000-000000000000",
		MissionRef:      mission.MissionRef,
		RequiredGroups:  []string{"Mission Operators"},
		AllowedSubjects: []string{"allowed@example.onmicrosoft.com"},
	}, Principal{}); err != nil {
		t.Fatalf("CreateEntraAppRegistration: %v", err)
	}

	resp, err := service.ResolveEntraAuthorityContext(ResolveEntraAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"aud":    "00000000-0000-0000-0000-000000000000",
			"sub":    "denied@example.onmicrosoft.com",
			"groups": []any{"Mission Operators"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveEntraAuthorityContext: %v", err)
	}
	if resp.Accepted || resp.Status != EntraResolutionStatusDenied || resp.ReasonCodes[0] != "entra_subject_not_allowed" {
		t.Fatalf("unexpected denial: %#v", resp)
	}
}

func approveEntraMission(t *testing.T, service *Service) ApproveProposalResponse {
	t.Helper()
	req := validProposalRequest()
	req.Intent = Purpose{Objective: "Govern Entra-authenticated agent work"}
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

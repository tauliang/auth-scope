package mission

import (
	"errors"
	"testing"
)

func TestSlackWorkspaceBindingAuthorizesMessageAction(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)

	binding, err := service.CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest{
		WorkspaceID:   "T12345678",
		WorkspaceName: "Acme Corp",
		WorkspaceURL:  "https://acme-corp.slack.com",
		MissionRef:    mission.MissionRef,
		RequiredRoles: []string{"Workspace Admin"},
		AdminRoles:    []string{"Owner"},
		AllowedChannels: []string{"C11111111", "C22222222"},
		AllowedUsers:  []string{"U12345678"},
		AllowedActions: []string{SlackActionTypePostMessage, SlackActionTypeReactMessage},
		Metadata:      map[string]string{"environment": "production"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateSlackWorkspaceBinding: %v", err)
	}
	if binding.WorkspaceID != "T12345678" || binding.WorkspaceName != "Acme Corp" {
		t.Fatalf("unexpected binding: %#v", binding)
	}
	if _, err := service.CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest{
		WorkspaceID: "T12345678",
		WorkspaceURL: "https://acme-corp.slack.com",
		MissionRef:  mission.MissionRef,
	}, Principal{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate binding err = %v, want ErrConflict", err)
	}

	resp, err := service.AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Email:       "user@example.com",
		Roles:       []string{"Workspace Admin", "Owner"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
		Context:     map[string]any{"timestamp": 1234567890},
		Evaluation: &SlackEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              SlackActor{AgentInstanceID: "slack_bot_1", ClientID: "research-agent"},
			Action: SlackMessageAction{
				Type:      "message_event",
				Resource:  SlackActionResource{Type: "message", ID: "msg_123", ChannelID: "C11111111"},
				Operation: "post",
			},
		},
	})
	if err != nil {
		t.Fatalf("AuthorizeSlackMessageAction: %v", err)
	}
	if !resp.Accepted || !resp.Admin || resp.BindingID != binding.BindingID || resp.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected accepted response: %#v", resp)
	}
	if resp.Context["slack.user_id"] != "U12345678" || resp.Context["slack.workspace_id"] != "T12345678" {
		t.Fatalf("unexpected context: %#v", resp.Context)
	}
	if resp.Context["timestamp"] != 1234567890 {
		t.Fatalf("context not preserved: %#v", resp.Context)
	}
	if resp.Evaluation == nil || resp.Evaluation.Decision != string(DecisionAllow) || resp.Evaluation.DecisionArtifact == "" {
		t.Fatalf("unexpected evaluation: %#v", resp.Evaluation)
	}
	updated, err := service.slack.GetSlackWorkspaceBinding(binding.BindingID)
	if err != nil {
		t.Fatalf("GetSlackWorkspaceBinding: %v", err)
	}
	if updated.LastUserID != "U12345678" || updated.LastResolutionStatus != SlackResolutionStatusAccepted {
		t.Fatalf("binding resolution fields = %#v", updated)
	}
}

func TestSlackAuthorizeDeniesMissingRequiredRole(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	if _, err := service.CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest{
		WorkspaceID:   "T12345678",
		WorkspaceURL:  "https://acme-corp.slack.com",
		MissionRef:    mission.MissionRef,
		RequiredRoles: []string{"Workspace Admin"},
	}, Principal{}); err != nil {
		t.Fatalf("CreateSlackWorkspaceBinding: %v", err)
	}

	resp, err := service.AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Email:       "user@example.com",
		Roles:       []string{"Member"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
	})
	if err != nil {
		t.Fatalf("AuthorizeSlackMessageAction: %v", err)
	}
	if resp.Accepted || resp.Status != SlackResolutionStatusDenied || resp.ReasonCodes[0] != "slack_required_role_missing" {
		t.Fatalf("unexpected denial: %#v", resp)
	}
}

func TestSlackAuthorizeDeniesBlockedChannel(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	if _, err := service.CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest{
		WorkspaceID:     "T12345678",
		WorkspaceURL:    "https://acme-corp.slack.com",
		MissionRef:      mission.MissionRef,
		RequiredRoles:   []string{"Workspace Admin"},
		BlockedChannels: []string{"C99999999"},
	}, Principal{}); err != nil {
		t.Fatalf("CreateSlackWorkspaceBinding: %v", err)
	}

	resp, err := service.AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C99999999",
		Action:      SlackActionTypePostMessage,
	})
	if err != nil {
		t.Fatalf("AuthorizeSlackMessageAction: %v", err)
	}
	if resp.Accepted || resp.Status != SlackResolutionStatusDenied || resp.ReasonCodes[0] != "slack_channel_blocked" {
		t.Fatalf("unexpected denial: %#v", resp)
	}
}

func TestSlackAuthorizeDeniesDisallowedAction(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	if _, err := service.CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest{
		WorkspaceID:    "T12345678",
		WorkspaceURL:   "https://acme-corp.slack.com",
		MissionRef:     mission.MissionRef,
		RequiredRoles:  []string{"Workspace Admin"},
		AllowedActions: []string{SlackActionTypePostMessage},
	}, Principal{}); err != nil {
		t.Fatalf("CreateSlackWorkspaceBinding: %v", err)
	}

	resp, err := service.AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypeDeleteMessage,
	})
	if err != nil {
		t.Fatalf("AuthorizeSlackMessageAction: %v", err)
	}
	if resp.Accepted || resp.Status != SlackResolutionStatusDenied || resp.ReasonCodes[0] != "slack_action_not_allowed" {
		t.Fatalf("unexpected denial: %#v", resp)
	}
}

func TestSlackAuthorizeDeniesNotAllowedUser(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	if _, err := service.CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest{
		WorkspaceID:   "T12345678",
		WorkspaceURL:  "https://acme-corp.slack.com",
		MissionRef:    mission.MissionRef,
		RequiredRoles: []string{"Workspace Admin"},
		AllowedUsers:  []string{"U87654321"},
	}, Principal{}); err != nil {
		t.Fatalf("CreateSlackWorkspaceBinding: %v", err)
	}

	resp, err := service.AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
	})
	if err != nil {
		t.Fatalf("AuthorizeSlackMessageAction: %v", err)
	}
	if resp.Accepted || resp.Status != SlackResolutionStatusDenied || resp.ReasonCodes[0] != "slack_user_not_allowed" {
		t.Fatalf("unexpected denial: %#v", resp)
	}
}

func TestSlackAuthorizeWithRoleMatchAll(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	if _, err := service.CreateSlackWorkspaceBinding(CreateSlackWorkspaceBindingRequest{
		WorkspaceID:   "T12345678",
		WorkspaceURL:  "https://acme-corp.slack.com",
		MissionRef:    mission.MissionRef,
		RequiredRoles: []string{"Workspace Admin", "Moderator"},
		RoleMatchMode: SlackRoleMatchAll,
	}, Principal{}); err != nil {
		t.Fatalf("CreateSlackWorkspaceBinding: %v", err)
	}

	// Should deny - missing Moderator role
	resp, err := service.AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
	})
	if err != nil {
		t.Fatalf("AuthorizeSlackMessageAction: %v", err)
	}
	if resp.Accepted || resp.ReasonCodes[0] != "slack_required_role_missing" {
		t.Fatalf("expected denial with missing role: %#v", resp)
	}

	// Should accept - has both roles
	resp, err = service.AuthorizeSlackMessageAction(AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin", "Moderator"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
	})
	if err != nil {
		t.Fatalf("AuthorizeSlackMessageAction: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("expected acceptance with all roles: %#v", resp)
	}
}

func approveSlackMission(t *testing.T, service *Service) ApproveProposalResponse {
	t.Helper()
	req := validProposalRequest()
	req.Intent = Purpose{Objective: "Govern Slack workspace agent activity"}
	req.Conditions = nil
	req.AuthorityRegion = AuthorityRegion{
		Resources:        []ResourceGrant{{Type: "message", ID: "msg_*", Actions: []string{"post", "react"}}},
		ForbiddenActions: []string{"delete_channel"},
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

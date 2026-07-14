package mission

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPISlackIntegrationLifecycle(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	router := withTestAdminAuthorization(NewHandler(service).Routes())

	createBinding := httptest.NewRecorder()
	router.ServeHTTP(createBinding, jsonRequest(http.MethodPost, "/v1/integrations/slack/workspace-bindings", CreateSlackWorkspaceBindingRequest{
		WorkspaceID:     "T12345678",
		WorkspaceName:   "Acme Corp",
		WorkspaceURL:    "https://acme-corp.slack.com",
		MissionRef:      mission.MissionRef,
		RequiredRoles:   []string{"Workspace Admin"},
		AdminRoles:      []string{"Owner"},
		AllowedChannels: []string{"C11111111"},
	}))
	if createBinding.Code != http.StatusCreated {
		t.Fatalf("create binding status = %d body=%s", createBinding.Code, createBinding.Body.String())
	}
	var binding SlackWorkspaceBinding
	decodeTestJSON(t, createBinding.Body.Bytes(), &binding)
	if binding.WorkspaceID != "T12345678" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected binding: %#v", binding)
	}

	listBindings := httptest.NewRecorder()
	router.ServeHTTP(listBindings, httptest.NewRequest(http.MethodGet, "/v1/integrations/slack/workspace-bindings", nil))
	if listBindings.Code != http.StatusOK {
		t.Fatalf("list bindings status = %d body=%s", listBindings.Code, listBindings.Body.String())
	}

	authorizeAccepted := httptest.NewRecorder()
	router.ServeHTTP(authorizeAccepted, jsonRequest(http.MethodPost, "/v1/integrations/slack/message-actions/authorize", AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Email:       "user@example.com",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
		Evaluation: &SlackEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              SlackActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: SlackMessageAction{
				Type:      "message_event",
				Resource:  SlackActionResource{Type: "message", ID: "msg_123", ChannelID: "C11111111"},
				Operation: "post",
			},
		},
	}))
	if authorizeAccepted.Code != http.StatusOK {
		t.Fatalf("authorize status = %d body=%s", authorizeAccepted.Code, authorizeAccepted.Body.String())
	}
	var authorized SlackMessageAuthorizationResponse
	decodeTestJSON(t, authorizeAccepted.Body.Bytes(), &authorized)
	if !authorized.Accepted || authorized.Evaluation == nil || authorized.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("unexpected authorized response: %#v", authorized)
	}

	authorizeDenied := httptest.NewRecorder()
	router.ServeHTTP(authorizeDenied, jsonRequest(http.MethodPost, "/v1/integrations/slack/message-actions/authorize", AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U87654321",
		Roles:       []string{"Member"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypePostMessage,
	}))
	if authorizeDenied.Code != http.StatusOK {
		t.Fatalf("authorize denied status = %d body=%s", authorizeDenied.Code, authorizeDenied.Body.String())
	}
	var deniedResp SlackMessageAuthorizationResponse
	decodeTestJSON(t, authorizeDenied.Body.Bytes(), &deniedResp)
	if deniedResp.Accepted || deniedResp.ReasonCodes[0] != "slack_required_role_missing" {
		t.Fatalf("unexpected denied response: %#v", deniedResp)
	}
}

func TestAPISlackMessageActionDeniedByBlockedChannel(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	router := withTestAdminAuthorization(NewHandler(service).Routes())

	// Create binding with blocked channel
	createBinding := httptest.NewRecorder()
	router.ServeHTTP(createBinding, jsonRequest(http.MethodPost, "/v1/integrations/slack/workspace-bindings", CreateSlackWorkspaceBindingRequest{
		WorkspaceID:     "T12345678",
		WorkspaceURL:    "https://acme-corp.slack.com",
		MissionRef:      mission.MissionRef,
		RequiredRoles:   []string{"Workspace Admin"},
		BlockedChannels: []string{"C99999999"},
	}))
	if createBinding.Code != http.StatusCreated {
		t.Fatalf("create binding status = %d", createBinding.Code)
	}

	// Try to post to blocked channel
	authorize := httptest.NewRecorder()
	router.ServeHTTP(authorize, jsonRequest(http.MethodPost, "/v1/integrations/slack/message-actions/authorize", AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C99999999",
		Action:      SlackActionTypePostMessage,
	}))
	if authorize.Code != http.StatusOK {
		t.Fatalf("authorize status = %d", authorize.Code)
	}
	var resp SlackMessageAuthorizationResponse
	decodeTestJSON(t, authorize.Body.Bytes(), &resp)
	if resp.Accepted || resp.ReasonCodes[0] != "slack_channel_blocked" {
		t.Fatalf("expected channel blocked denial: %#v", resp)
	}
}

func TestAPISlackMessageActionDeniedByDisallowedAction(t *testing.T) {
	service := testService()
	mission := approveSlackMission(t, service)
	router := withTestAdminAuthorization(NewHandler(service).Routes())

	// Create binding with allowed actions
	createBinding := httptest.NewRecorder()
	router.ServeHTTP(createBinding, jsonRequest(http.MethodPost, "/v1/integrations/slack/workspace-bindings", CreateSlackWorkspaceBindingRequest{
		WorkspaceID:    "T12345678",
		WorkspaceURL:   "https://acme-corp.slack.com",
		MissionRef:     mission.MissionRef,
		RequiredRoles:  []string{"Workspace Admin"},
		AllowedActions: []string{SlackActionTypePostMessage, SlackActionTypeReactMessage},
	}))
	if createBinding.Code != http.StatusCreated {
		t.Fatalf("create binding status = %d", createBinding.Code)
	}

	// Try disallowed action
	authorize := httptest.NewRecorder()
	router.ServeHTTP(authorize, jsonRequest(http.MethodPost, "/v1/integrations/slack/message-actions/authorize", AuthorizeSlackMessageActionRequest{
		WorkspaceID: "T12345678",
		UserID:      "U12345678",
		Roles:       []string{"Workspace Admin"},
		ChannelID:   "C11111111",
		Action:      SlackActionTypeDeleteMessage,
	}))
	if authorize.Code != http.StatusOK {
		t.Fatalf("authorize status = %d", authorize.Code)
	}
	var resp SlackMessageAuthorizationResponse
	decodeTestJSON(t, authorize.Body.Bytes(), &resp)
	if resp.Accepted || resp.ReasonCodes[0] != "slack_action_not_allowed" {
		t.Fatalf("expected action not allowed denial: %#v", resp)
	}
}

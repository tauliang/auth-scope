package mission

import "testing"

func TestIntegrationActorIdentityBinding(t *testing.T) {
	identity := AgentIdentity{
		Agent: Agent{
			InstanceID:    "inst_123",
			ClientID:      "research-agent",
			KeyThumbprint: "thumb_123",
		},
		KeyThumbprint: "thumb_123",
	}

	githubActorValue := GitHubActor{}
	if err := bindGitHubActorIdentity(&githubActorValue, identity); err != nil {
		t.Fatalf("bindGitHubActorIdentity: %v", err)
	}
	if githubActorValue.AgentInstanceID != "inst_123" || githubActorValue.ClientID != "research-agent" || githubActorValue.KeyThumbprint != "thumb_123" {
		t.Fatalf("unexpected GitHub actor: %#v", githubActorValue)
	}

	oktaActorValue := OktaActor{}
	if err := bindOktaActorIdentity(&oktaActorValue, identity); err != nil {
		t.Fatalf("bindOktaActorIdentity: %v", err)
	}
	if oktaActorValue.AgentInstanceID != "inst_123" || oktaActorValue.ClientID != "research-agent" || oktaActorValue.KeyThumbprint != "thumb_123" {
		t.Fatalf("unexpected Okta actor: %#v", oktaActorValue)
	}

	entraActorValue := EntraActor{}
	if err := bindEntraActorIdentity(&entraActorValue, identity); err != nil {
		t.Fatalf("bindEntraActorIdentity: %v", err)
	}
	if entraActorValue.AgentInstanceID != "inst_123" || entraActorValue.ClientID != "research-agent" || entraActorValue.KeyThumbprint != "thumb_123" {
		t.Fatalf("unexpected Entra actor: %#v", entraActorValue)
	}

	slackActorValue := SlackActor{}
	if err := bindSlackActorIdentity(&slackActorValue, identity); err != nil {
		t.Fatalf("bindSlackActorIdentity: %v", err)
	}
	if slackActorValue.AgentInstanceID != "inst_123" || slackActorValue.ClientID != "research-agent" || slackActorValue.KeyThumbprint != "thumb_123" {
		t.Fatalf("unexpected Slack actor: %#v", slackActorValue)
	}
}

func TestIntegrationActorIdentityBindingRejectsMismatch(t *testing.T) {
	identity := AgentIdentity{
		Agent: Agent{
			InstanceID:    "inst_123",
			ClientID:      "research-agent",
			KeyThumbprint: "thumb_123",
		},
		KeyThumbprint: "thumb_123",
	}

	for name, bind := range map[string]func() error{
		"github": func() error {
			actor := GitHubActor{AgentInstanceID: "other"}
			return bindGitHubActorIdentity(&actor, identity)
		},
		"okta": func() error {
			actor := OktaActor{ClientID: "other"}
			return bindOktaActorIdentity(&actor, identity)
		},
		"entra": func() error {
			actor := EntraActor{AgentInstanceID: "other"}
			return bindEntraActorIdentity(&actor, identity)
		},
		"slack": func() error {
			actor := SlackActor{ClientID: "other"}
			return bindSlackActorIdentity(&actor, identity)
		},
	} {
		if err := bind(); err == nil {
			t.Fatalf("expected %s actor mismatch to be rejected", name)
		}
	}
}

func TestIntegrationPrincipalAdapters(t *testing.T) {
	principal := Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"}

	gh := githubPrincipal(principal)
	if gh.Subject != principal.Subject || gh.Issuer != principal.Issuer {
		t.Fatalf("unexpected GitHub principal: %#v", gh)
	}

	okta := oktaPrincipal(principal)
	if okta.Subject != principal.Subject || okta.Issuer != principal.Issuer {
		t.Fatalf("unexpected Okta principal: %#v", okta)
	}

	entra := entraPrincipal(principal)
	if entra.Subject != principal.Subject || entra.Issuer != principal.Issuer {
		t.Fatalf("unexpected Entra principal: %#v", entra)
	}

	slack := slackPrincipal(principal)
	if slack.UserID != principal.Subject || slack.Email != "" {
		t.Fatalf("unexpected Slack principal: %#v", slack)
	}
}

func TestAuthZENSubjectIdentityBinding(t *testing.T) {
	identity := AgentIdentity{
		Agent: Agent{
			InstanceID:    "inst_123",
			ClientID:      "research-agent",
			KeyThumbprint: "thumb_123",
		},
		KeyThumbprint: "thumb_123",
	}

	subject := AuthZENEntity{}
	if err := bindAuthZENSubjectIdentity(&subject, identity); err != nil {
		t.Fatalf("bindAuthZENSubjectIdentity: %v", err)
	}
	if subject.ID != "inst_123" {
		t.Fatalf("unexpected subject id: %#v", subject)
	}
	if subject.Properties["agent_instance_id"] != "inst_123" ||
		subject.Properties["client_id"] != "research-agent" ||
		subject.Properties["key_thumbprint"] != "thumb_123" {
		t.Fatalf("unexpected subject properties: %#v", subject.Properties)
	}

	mismatchedID := AuthZENEntity{ID: "other"}
	if err := bindAuthZENSubjectIdentity(&mismatchedID, identity); err == nil {
		t.Fatal("expected subject id mismatch to fail")
	}

	mismatchedClient := AuthZENEntity{Properties: map[string]any{"client_id": "other"}}
	if err := bindAuthZENSubjectIdentity(&mismatchedClient, identity); err == nil {
		t.Fatal("expected subject client mismatch to fail")
	}
}

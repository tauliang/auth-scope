package mission

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMultiBearerAdminAuthenticatorUsesTokenBoundPrincipal(t *testing.T) {
	authenticator := NewCredentialAdminAuthenticator([]AdminCredential{
		{Token: "alice-token", Subject: "alice@example.com", Issuer: "https://idp.example.com"},
		{Token: "bob-token", Subject: "bob@example.com", Issuer: "https://idp.example.com"},
		{Token: "", Subject: "ignored@example.com"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/approval-rules", nil)
	req.Header.Set("Authorization", "Bearer bob-token")
	principal, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if principal.Subject != "bob@example.com" {
		t.Fatalf("principal = %#v, want Bob", principal)
	}

	req.Header.Set("Authorization", "bob-token")
	if _, err := authenticator.Authenticate(req); !errors.Is(err, ErrAdminAuthenticationRequired) {
		t.Fatalf("non-bearer authentication err = %v", err)
	}
}

func TestAdminAuthenticatorFromEnvSupportsCredentialSetAndRejectsInvalidConfig(t *testing.T) {
	t.Setenv("AUTH_SCOPE_ADMIN_TOKEN", "")
	t.Setenv("AUTH_SCOPE_ADMIN_CREDENTIALS", `[{"token":"admin-token","subject":"security@example.com","issuer":"issuer"}]`)
	authenticator := AdminAuthenticatorFromEnv()
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	principal, err := authenticator.Authenticate(req)
	if err != nil || principal.Subject != "security@example.com" {
		t.Fatalf("environment credentials principal=%#v err=%v", principal, err)
	}

	t.Setenv("AUTH_SCOPE_ADMIN_CREDENTIALS", "{")
	authenticator = AdminAuthenticatorFromEnv()
	if _, err := authenticator.Authenticate(req); !errors.Is(err, ErrAdminAuthenticationRequired) {
		t.Fatalf("invalid credential config err = %v", err)
	}
}

func TestBearerAdminAuthenticatorRequiresConfiguredIdentity(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	req.Header.Set("Authorization", "Bearer token")
	if _, err := NewBearerAdminAuthenticator("", Principal{Subject: "admin"}).Authenticate(req); !errors.Is(err, ErrAdminAuthenticationRequired) {
		t.Fatalf("empty token err = %v", err)
	}
	if _, err := NewBearerAdminAuthenticator("token", Principal{}).Authenticate(req); !errors.Is(err, ErrAdminAuthenticationRequired) {
		t.Fatalf("empty principal err = %v", err)
	}
}

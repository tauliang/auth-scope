package mission

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
)

const (
	defaultDevelopmentAdminToken       = "auth-scope-dev-admin-token"
	defaultDevelopmentSecondAdminToken = "auth-scope-dev-admin-token-bob"
	defaultDevelopmentAdminSubject     = "alice@example.com"
	defaultDevelopmentAdminIssuer      = "https://idp.example.com"
)

var ErrAdminAuthenticationRequired = errors.New("admin authentication required")

type AdminAuthenticator interface {
	Authenticate(*http.Request) (Principal, error)
}

type BearerAdminAuthenticator struct {
	token     string
	principal Principal
}

type AdminCredential struct {
	Token         string `json:"token"`
	Subject       string `json:"subject"`
	Issuer        string `json:"issuer"`
	TenantSubject string `json:"tenant_subject,omitempty"`
}

type MultiBearerAdminAuthenticator struct {
	principals map[string]Principal
}

func NewBearerAdminAuthenticator(token string, principal Principal) *BearerAdminAuthenticator {
	return &BearerAdminAuthenticator{
		token:     strings.TrimSpace(token),
		principal: principal,
	}
}

func AdminAuthenticatorFromEnv() AdminAuthenticator {
	if raw := strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_CREDENTIALS")); raw != "" {
		var credentials []AdminCredential
		if err := json.Unmarshal([]byte(raw), &credentials); err != nil {
			return NewMultiBearerAdminAuthenticator(nil)
		}
		return NewCredentialAdminAuthenticator(credentials)
	}
	token := strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_TOKEN"))
	principal := Principal{
		Subject: strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_SUBJECT")),
		Issuer:  strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_ISSUER")),
	}
	if principal.Subject == "" {
		principal.Subject = defaultDevelopmentAdminSubject
	}
	if principal.Issuer == "" {
		principal.Issuer = defaultDevelopmentAdminIssuer
	}
	if token == "" {
		return NewMultiBearerAdminAuthenticator(map[string]Principal{
			defaultDevelopmentAdminToken: principal,
			defaultDevelopmentSecondAdminToken: {
				Subject: "bob@example.com",
				Issuer:  defaultDevelopmentAdminIssuer,
			},
		})
	}
	return NewBearerAdminAuthenticator(token, principal)
}

func NewCredentialAdminAuthenticator(credentials []AdminCredential) *MultiBearerAdminAuthenticator {
	principals := make(map[string]Principal, len(credentials))
	for _, credential := range credentials {
		token := strings.TrimSpace(credential.Token)
		subject := strings.TrimSpace(credential.Subject)
		if token == "" || subject == "" {
			continue
		}
		principals[token] = Principal{
			Subject:       subject,
			Issuer:        strings.TrimSpace(credential.Issuer),
			TenantSubject: strings.TrimSpace(credential.TenantSubject),
		}
	}
	return NewMultiBearerAdminAuthenticator(principals)
}

func NewMultiBearerAdminAuthenticator(principals map[string]Principal) *MultiBearerAdminAuthenticator {
	copyOfPrincipals := make(map[string]Principal, len(principals))
	for token, principal := range principals {
		copyOfPrincipals[token] = principal
	}
	return &MultiBearerAdminAuthenticator{principals: copyOfPrincipals}
}

func (a *BearerAdminAuthenticator) Authenticate(r *http.Request) (Principal, error) {
	if a == nil || a.token == "" || a.principal.Subject == "" {
		return Principal{}, ErrAdminAuthenticationRequired
	}
	presented := bearerToken(r)
	if presented == "" || subtle.ConstantTimeCompare([]byte(presented), []byte(a.token)) != 1 {
		return Principal{}, ErrAdminAuthenticationRequired
	}
	return a.principal, nil
}

func (a *MultiBearerAdminAuthenticator) Authenticate(r *http.Request) (Principal, error) {
	if a == nil || len(a.principals) == 0 {
		return Principal{}, ErrAdminAuthenticationRequired
	}
	presented := bearerToken(r)
	for token, principal := range a.principals {
		if presented != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1 {
			return principal, nil
		}
	}
	return Principal{}, ErrAdminAuthenticationRequired
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

type adminPrincipalContextKey struct{}

func withAdminPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, adminPrincipalContextKey{}, principal)
}

func adminPrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(adminPrincipalContextKey{}).(Principal)
	return principal, ok && principal.Subject != ""
}

func authenticatedAdmin(r *http.Request) Principal {
	principal, _ := adminPrincipalFromContext(r.Context())
	return principal
}

func principalActor(principal Principal) map[string]any {
	return map[string]any{
		"subject":        principal.Subject,
		"issuer":         principal.Issuer,
		"tenant_subject": principal.TenantSubject,
	}
}

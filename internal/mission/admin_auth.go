package mission

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
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
	token    string
	identity AdminIdentity
}

type AdminCredential struct {
	Token         string   `json:"token"`
	Subject       string   `json:"subject"`
	Issuer        string   `json:"issuer"`
	TenantSubject string   `json:"tenant_subject,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Roles         []string `json:"roles,omitempty"`
	Permissions   []string `json:"permissions,omitempty"`
}

type MultiBearerAdminAuthenticator struct {
	identities map[string]AdminIdentity
}

func NewBearerAdminAuthenticator(token string, principal Principal) *BearerAdminAuthenticator {
	return &BearerAdminAuthenticator{
		token:    strings.TrimSpace(token),
		identity: legacyAdminIdentity(principal),
	}
}

func AdminAuthenticatorFromEnv() AdminAuthenticator {
	authenticator, err := AdminAuthenticatorFromEnvStrict(false)
	if err != nil {
		return NewMultiBearerAdminAuthenticator(nil)
	}
	return authenticator
}

func AdminAuthenticatorFromEnvStrict(production bool) (AdminAuthenticator, error) {
	if raw := strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_CREDENTIALS")); raw != "" {
		var credentials []AdminCredential
		if err := json.Unmarshal([]byte(raw), &credentials); err != nil {
			return nil, fmt.Errorf("parse AUTH_SCOPE_ADMIN_CREDENTIALS: %w", err)
		}
		authenticator := NewCredentialAdminAuthenticator(credentials)
		if production && len(authenticator.identities) == 0 {
			return nil, errors.New("AUTH_SCOPE_ADMIN_CREDENTIALS must contain at least one valid credential")
		}
		return authenticator, nil
	}
	if authenticator, configured, err := OIDCAdminAuthenticatorFromEnv(); configured || err != nil {
		if err != nil {
			return nil, err
		}
		return authenticator, nil
	}
	token := strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_TOKEN"))
	principal := Principal{
		Subject: strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_SUBJECT")),
		Issuer:  strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_ISSUER")),
	}
	if production {
		if token == "" || principal.Subject == "" || principal.Issuer == "" {
			return nil, errors.New("AUTH_SCOPE_ADMIN_TOKEN, AUTH_SCOPE_ADMIN_SUBJECT, and AUTH_SCOPE_ADMIN_ISSUER are required in production")
		}
		return NewBearerAdminAuthenticator(token, principal), nil
	}
	if principal.Subject == "" {
		principal.Subject = defaultDevelopmentAdminSubject
	}
	if principal.Issuer == "" {
		principal.Issuer = defaultDevelopmentAdminIssuer
	}
	if token == "" {
		return NewMultiBearerAdminAuthenticatorFromIdentities(map[string]AdminIdentity{
			defaultDevelopmentAdminToken: newAdminIdentity(principal, "static", nil, nil, nil, true),
			defaultDevelopmentSecondAdminToken: newAdminIdentity(Principal{
				Subject: "bob@example.com",
				Issuer:  defaultDevelopmentAdminIssuer,
			}, "static", nil, nil, nil, true),
		}), nil
	}
	return NewBearerAdminAuthenticator(token, principal), nil
}

func NewCredentialAdminAuthenticator(credentials []AdminCredential) *MultiBearerAdminAuthenticator {
	identities := make(map[string]AdminIdentity, len(credentials))
	for _, credential := range credentials {
		token := strings.TrimSpace(credential.Token)
		subject := strings.TrimSpace(credential.Subject)
		if token == "" || subject == "" {
			continue
		}
		principal := Principal{
			Subject:       subject,
			Issuer:        strings.TrimSpace(credential.Issuer),
			TenantSubject: strings.TrimSpace(credential.TenantSubject),
		}
		identities[token] = newAdminIdentity(principal, "static", credential.Groups, credential.Roles, credential.Permissions, true)
	}
	return NewMultiBearerAdminAuthenticatorFromIdentities(identities)
}

func NewMultiBearerAdminAuthenticator(principals map[string]Principal) *MultiBearerAdminAuthenticator {
	identities := make(map[string]AdminIdentity, len(principals))
	for token, principal := range principals {
		identities[token] = legacyAdminIdentity(principal)
	}
	return NewMultiBearerAdminAuthenticatorFromIdentities(identities)
}

func NewMultiBearerAdminAuthenticatorFromIdentities(identities map[string]AdminIdentity) *MultiBearerAdminAuthenticator {
	copyOfIdentities := make(map[string]AdminIdentity, len(identities))
	for token, identity := range identities {
		copyOfIdentities[token] = newAdminIdentity(identity.Principal, identity.Provider, identity.Groups, identity.Roles, identity.Permissions, true)
	}
	return &MultiBearerAdminAuthenticator{identities: copyOfIdentities}
}

func (a *BearerAdminAuthenticator) Authenticate(r *http.Request) (Principal, error) {
	identity, err := a.AuthenticateAdmin(r)
	if err != nil {
		return Principal{}, err
	}
	return identity.Principal, nil
}

func (a *BearerAdminAuthenticator) AuthenticateAdmin(r *http.Request) (AdminIdentity, error) {
	if a == nil || a.token == "" || a.identity.Principal.Subject == "" {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	presented := bearerToken(r)
	if presented == "" || subtle.ConstantTimeCompare([]byte(presented), []byte(a.token)) != 1 {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	return a.identity, nil
}

func (a *MultiBearerAdminAuthenticator) Authenticate(r *http.Request) (Principal, error) {
	identity, err := a.AuthenticateAdmin(r)
	if err != nil {
		return Principal{}, err
	}
	return identity.Principal, nil
}

func (a *MultiBearerAdminAuthenticator) AuthenticateAdmin(r *http.Request) (AdminIdentity, error) {
	if a == nil || len(a.identities) == 0 {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	presented := bearerToken(r)
	for token, identity := range a.identities {
		if presented != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1 {
			return identity, nil
		}
	}
	return AdminIdentity{}, ErrAdminAuthenticationRequired
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

type adminPrincipalContextKey struct{}
type adminIdentityContextKey struct{}

func withAdminPrincipal(ctx context.Context, principal Principal) context.Context {
	return withAdminIdentity(ctx, legacyAdminIdentity(principal))
}

func withAdminIdentity(ctx context.Context, identity AdminIdentity) context.Context {
	ctx = context.WithValue(ctx, adminIdentityContextKey{}, identity)
	return context.WithValue(ctx, adminPrincipalContextKey{}, identity.Principal)
}

func adminPrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(adminPrincipalContextKey{}).(Principal)
	return principal, ok && principal.Subject != ""
}

func authenticatedAdmin(r *http.Request) Principal {
	principal, _ := adminPrincipalFromContext(r.Context())
	return principal
}

func adminIdentityFromContext(ctx context.Context) (AdminIdentity, bool) {
	identity, ok := ctx.Value(adminIdentityContextKey{}).(AdminIdentity)
	return identity, ok && identity.Principal.Subject != ""
}

func authenticatedAdminIdentity(r *http.Request) AdminIdentity {
	identity, ok := adminIdentityFromContext(r.Context())
	if ok {
		return identity
	}
	return legacyAdminIdentity(authenticatedAdmin(r))
}

func principalActor(principal Principal) map[string]any {
	return map[string]any{
		"subject":        principal.Subject,
		"issuer":         principal.Issuer,
		"tenant_subject": principal.TenantSubject,
	}
}

type adminIdentityAuthenticator interface {
	AuthenticateAdmin(*http.Request) (AdminIdentity, error)
}

func authenticateAdminIdentity(authenticator AdminAuthenticator, r *http.Request) (AdminIdentity, error) {
	if authenticator == nil {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	if identityAuthenticator, ok := authenticator.(adminIdentityAuthenticator); ok {
		return identityAuthenticator.AuthenticateAdmin(r)
	}
	principal, err := authenticator.Authenticate(r)
	if err != nil {
		return AdminIdentity{}, err
	}
	return legacyAdminIdentity(principal), nil
}

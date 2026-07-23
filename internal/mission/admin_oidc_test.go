package mission

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOIDCAdminAuthenticatorVerifiesJWKSAndMapsGroupsToRoles(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	key := testAdminRSAKey(t)
	authenticator, err := NewOIDCAdminAuthenticator(OIDCAdminConfig{
		Issuer:   "https://idp.example.com",
		Audience: "auth-scope-admin",
		JWKSJSON: testAdminJWKS(t, key, "admin-key"),
		GroupRoleMapping: map[string][]string{
			"AuthScope Operators":   {string(AdminRoleOperator)},
			"AuthScope Integrators": {string(AdminRoleIntegrationAdmin)},
		},
		Clock: fixedClock{now: now},
	})
	if err != nil {
		t.Fatalf("NewOIDCAdminAuthenticator: %v", err)
	}
	token := testAdminJWT(t, key, "admin-key", map[string]any{
		"iss":            "https://idp.example.com",
		"aud":            []string{"auth-scope-admin"},
		"sub":            "sre@example.com",
		"tenant_subject": "tenant-a",
		"groups":         []string{"AuthScope Operators", "AuthScope Integrators"},
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Add(-time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := authenticator.AuthenticateAdmin(req)
	if err != nil {
		t.Fatalf("AuthenticateAdmin: %v", err)
	}
	if identity.Provider != "oidc" || identity.Principal.Subject != "sre@example.com" || identity.Principal.TenantSubject != "tenant-a" {
		t.Fatalf("identity principal = %#v", identity)
	}
	if !containsString(identity.Roles, string(AdminRoleOperator)) || !containsString(identity.Roles, string(AdminRoleIntegrationAdmin)) {
		t.Fatalf("roles = %#v", identity.Roles)
	}
	for _, permission := range []string{
		string(AdminPermissionOperate),
		string(AdminPermissionManageGovernance),
		string(AdminPermissionManageIntegrations),
	} {
		if !containsString(identity.Permissions, permission) {
			t.Fatalf("permissions = %#v, want %s", identity.Permissions, permission)
		}
	}
}

func TestOIDCAdminAuthenticatorRejectsExpiredOrWrongAudienceToken(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	key := testAdminRSAKey(t)
	authenticator, err := NewOIDCAdminAuthenticator(OIDCAdminConfig{
		Issuer:           "https://idp.example.com",
		Audience:         "auth-scope-admin",
		JWKSJSON:         testAdminJWKS(t, key, "admin-key"),
		GroupRoleMapping: map[string][]string{"AuthScope Auditors": {string(AdminRoleAuditor)}},
		Clock:            fixedClock{now: now},
	})
	if err != nil {
		t.Fatalf("NewOIDCAdminAuthenticator: %v", err)
	}

	for _, claims := range []map[string]any{
		{
			"iss":    "https://idp.example.com",
			"aud":    "other-api",
			"sub":    "auditor@example.com",
			"groups": []string{"AuthScope Auditors"},
			"exp":    now.Add(time.Hour).Unix(),
		},
		{
			"iss":    "https://idp.example.com",
			"aud":    "auth-scope-admin",
			"sub":    "auditor@example.com",
			"groups": []string{"AuthScope Auditors"},
			"exp":    now.Add(-time.Second).Unix(),
		},
	} {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/session", nil)
		req.Header.Set("Authorization", "Bearer "+testAdminJWT(t, key, "admin-key", claims))
		if _, err := authenticator.AuthenticateAdmin(req); !errors.Is(err, ErrAdminAuthenticationRequired) {
			t.Fatalf("AuthenticateAdmin err = %v, want ErrAdminAuthenticationRequired", err)
		}
	}
}

func TestProductionConfigurationAcceptsOIDCAdminAuthenticator(t *testing.T) {
	now := time.Now().UTC()
	key := testAdminRSAKey(t)
	t.Setenv("AUTH_SCOPE_ADMIN_CREDENTIALS", "")
	t.Setenv("AUTH_SCOPE_ADMIN_TOKEN", "")
	t.Setenv("AUTH_SCOPE_ADMIN_SUBJECT", "")
	t.Setenv("AUTH_SCOPE_ADMIN_ISSUER", "")
	t.Setenv("AUTH_SCOPE_ADMIN_OIDC_ISSUER", "https://idp.example.com")
	t.Setenv("AUTH_SCOPE_ADMIN_OIDC_AUDIENCE", "auth-scope-admin")
	t.Setenv("AUTH_SCOPE_ADMIN_OIDC_JWKS", testAdminJWKS(t, key, "admin-key"))
	t.Setenv("AUTH_SCOPE_ADMIN_GROUP_ROLE_MAPPINGS", `{"AuthScope Approvers":["approver"]}`)

	authenticator, err := AdminAuthenticatorFromEnvStrict(true)
	if err != nil {
		t.Fatalf("AdminAuthenticatorFromEnvStrict: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/session", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminJWT(t, key, "admin-key", map[string]any{
		"iss":    "https://idp.example.com",
		"aud":    "auth-scope-admin",
		"sub":    "approver@example.com",
		"groups": []string{"AuthScope Approvers"},
		"exp":    now.Add(time.Hour).Unix(),
	}))
	identity, err := authenticator.(adminIdentityAuthenticator).AuthenticateAdmin(req)
	if err != nil {
		t.Fatalf("AuthenticateAdmin: %v", err)
	}
	if !containsString(identity.Roles, string(AdminRoleApprover)) || !containsString(identity.Permissions, string(AdminPermissionApprove)) {
		t.Fatalf("identity = %#v", identity)
	}
}

func testAdminRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key
}

func testAdminJWKS(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	jwks := map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": kid,
			"alg": "RS256",
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}},
	}
	data, err := json.Marshal(jwks)
	if err != nil {
		t.Fatalf("Marshal JWKS: %v", err)
	}
	return string(data)
}

func testAdminJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "RS256", "kid": kid, "typ": "JWT"}
	encodedHeader := testBase64URLJSON(t, header)
	encodedClaims := testBase64URLJSON(t, claims)
	unsigned := encodedHeader + "." + encodedClaims
	sum := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func testBase64URLJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal JSON: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

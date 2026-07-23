package mission

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultOIDCSubjectClaim     = "sub"
	defaultOIDCGroupsClaim      = "groups"
	defaultOIDCRolesClaim       = "roles"
	defaultOIDCPermissionsClaim = "permissions"
	defaultOIDCTenantClaim      = "tenant_subject"
)

type OIDCAdminConfig struct {
	Issuer           string
	Audience         string
	JWKSJSON         string
	SubjectClaim     string
	GroupsClaim      string
	RolesClaim       string
	PermissionsClaim string
	TenantClaim      string
	GroupRoleMapping map[string][]string
	Clock            Clock
}

type OIDCAdminAuthenticator struct {
	issuer           string
	audience         string
	subjectClaim     string
	groupsClaim      string
	rolesClaim       string
	permissionsClaim string
	tenantClaim      string
	groupRoleMapping map[string][]string
	keys             map[string]*rsa.PublicKey
	clock            Clock
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid,omitempty"`
	Algorithm string `json:"alg,omitempty"`
	Use       string `json:"use,omitempty"`
	N         string `json:"n"`
	E         string `json:"e"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid,omitempty"`
	Type      string `json:"typ,omitempty"`
}

func OIDCAdminAuthenticatorFromEnv() (*OIDCAdminAuthenticator, bool, error) {
	config := OIDCAdminConfig{
		Issuer:           strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_ISSUER")),
		Audience:         strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_AUDIENCE")),
		JWKSJSON:         strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_JWKS")),
		SubjectClaim:     strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_SUBJECT_CLAIM")),
		GroupsClaim:      strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_GROUPS_CLAIM")),
		RolesClaim:       strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_ROLES_CLAIM")),
		PermissionsClaim: strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_PERMISSIONS_CLAIM")),
		TenantClaim:      strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_OIDC_TENANT_CLAIM")),
	}
	mappingRaw := strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_GROUP_ROLE_MAPPINGS"))
	if mappingRaw == "" {
		mappingRaw = strings.TrimSpace(os.Getenv("AUTH_SCOPE_ADMIN_ROLE_MAPPINGS"))
	}
	if mappingRaw != "" {
		mapping, err := parseAdminGroupRoleMapping(mappingRaw)
		if err != nil {
			return nil, true, fmt.Errorf("parse admin group role mappings: %w", err)
		}
		config.GroupRoleMapping = mapping
	}
	configured := config.Issuer != "" || config.Audience != "" || config.JWKSJSON != ""
	if !configured {
		return nil, false, nil
	}
	authenticator, err := NewOIDCAdminAuthenticator(config)
	return authenticator, true, err
}

func NewOIDCAdminAuthenticator(config OIDCAdminConfig) (*OIDCAdminAuthenticator, error) {
	issuer := strings.TrimSpace(config.Issuer)
	audience := strings.TrimSpace(config.Audience)
	jwksJSON := strings.TrimSpace(config.JWKSJSON)
	if issuer == "" {
		return nil, errors.New("AUTH_SCOPE_ADMIN_OIDC_ISSUER is required when OIDC admin auth is configured")
	}
	if audience == "" {
		return nil, errors.New("AUTH_SCOPE_ADMIN_OIDC_AUDIENCE is required when OIDC admin auth is configured")
	}
	if jwksJSON == "" {
		return nil, errors.New("AUTH_SCOPE_ADMIN_OIDC_JWKS is required when OIDC admin auth is configured")
	}
	keys, err := parseAdminJWKS(jwksJSON)
	if err != nil {
		return nil, err
	}
	clock := config.Clock
	if clock == nil {
		clock = SystemClock{}
	}
	return &OIDCAdminAuthenticator{
		issuer:           issuer,
		audience:         audience,
		subjectClaim:     defaultString(config.SubjectClaim, defaultOIDCSubjectClaim),
		groupsClaim:      defaultString(config.GroupsClaim, defaultOIDCGroupsClaim),
		rolesClaim:       defaultString(config.RolesClaim, defaultOIDCRolesClaim),
		permissionsClaim: defaultString(config.PermissionsClaim, defaultOIDCPermissionsClaim),
		tenantClaim:      defaultString(config.TenantClaim, defaultOIDCTenantClaim),
		groupRoleMapping: normalizeAdminGroupRoleMapping(config.GroupRoleMapping),
		keys:             keys,
		clock:            clock,
	}, nil
}

func (a *OIDCAdminAuthenticator) Authenticate(r *http.Request) (Principal, error) {
	identity, err := a.AuthenticateAdmin(r)
	if err != nil {
		return Principal{}, err
	}
	return identity.Principal, nil
}

func (a *OIDCAdminAuthenticator) AuthenticateAdmin(r *http.Request) (AdminIdentity, error) {
	if a == nil {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	token := bearerToken(r)
	if token == "" {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	claims, err := a.verifyJWT(token)
	if err != nil {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	if stringClaim(claims, "iss") != a.issuer {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	if !audienceMatches(claims["aud"], a.audience) {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	if err := validateJWTTimeClaims(claims, a.clock.Now()); err != nil {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	subject := stringClaim(claims, a.subjectClaim)
	if subject == "" {
		return AdminIdentity{}, ErrAdminAuthenticationRequired
	}
	tenant := stringClaim(claims, a.tenantClaim)
	if tenant == "" && a.tenantClaim == defaultOIDCTenantClaim {
		tenant = stringClaim(claims, "tenant")
	}
	groups := stringListClaim(claims, a.groupsClaim)
	roles := append(stringListClaim(claims, a.rolesClaim), rolesForAdminGroups(groups, a.groupRoleMapping)...)
	permissions := stringListClaim(claims, a.permissionsClaim)
	return newAdminIdentity(Principal{
		Subject:       subject,
		Issuer:        a.issuer,
		TenantSubject: tenant,
	}, "oidc", groups, roles, permissions, false), nil
}

func (a *OIDCAdminAuthenticator) verifyJWT(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("jwt must have three parts")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}
	if header.Algorithm != "RS256" {
		return nil, fmt.Errorf("unsupported jwt algorithm %q", header.Algorithm)
	}
	key, err := a.keyForHeader(header)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	signed := []byte(parts[0] + "." + parts[1])
	sum := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], signature); err != nil {
		return nil, err
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (a *OIDCAdminAuthenticator) keyForHeader(header jwtHeader) (*rsa.PublicKey, error) {
	if header.KeyID != "" {
		key, ok := a.keys[header.KeyID]
		if !ok {
			return nil, fmt.Errorf("jwks key %q not found", header.KeyID)
		}
		return key, nil
	}
	if len(a.keys) == 1 {
		for _, key := range a.keys {
			return key, nil
		}
	}
	return nil, errors.New("jwt kid is required when jwks has multiple keys")
}

func parseAdminJWKS(raw string) (map[string]*rsa.PublicKey, error) {
	var document jwksDocument
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return nil, fmt.Errorf("parse AUTH_SCOPE_ADMIN_OIDC_JWKS: %w", err)
	}
	if len(document.Keys) == 0 {
		return nil, errors.New("AUTH_SCOPE_ADMIN_OIDC_JWKS must contain at least one key")
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for index, key := range document.Keys {
		if key.KeyType != "RSA" {
			continue
		}
		publicKey, err := rsaPublicKeyFromJWK(key)
		if err != nil {
			return nil, err
		}
		keyID := strings.TrimSpace(key.KeyID)
		if keyID == "" {
			keyID = fmt.Sprintf("key-%d", index)
		}
		keys[keyID] = publicKey
	}
	if len(keys) == 0 {
		return nil, errors.New("AUTH_SCOPE_ADMIN_OIDC_JWKS must contain at least one RSA key")
	}
	return keys, nil
}

func rsaPublicKeyFromJWK(key jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("decode jwk n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("decode jwk e: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("jwk e is invalid")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func validateJWTTimeClaims(claims map[string]any, now time.Time) error {
	exp, ok := numericDateClaim(claims, "exp")
	if !ok {
		return errors.New("exp is required")
	}
	if !now.Before(exp) {
		return errors.New("jwt is expired")
	}
	if nbf, ok := numericDateClaim(claims, "nbf"); ok && now.Before(nbf) {
		return errors.New("jwt is not active")
	}
	if iat, ok := numericDateClaim(claims, "iat"); ok && now.Add(5*time.Minute).Before(iat) {
		return errors.New("jwt was issued in the future")
	}
	return nil
}

func numericDateClaim(claims map[string]any, name string) (time.Time, bool) {
	value, ok := claims[name]
	if !ok {
		return time.Time{}, false
	}
	switch typed := value.(type) {
	case float64:
		return time.Unix(int64(typed), 0), true
	case json.Number:
		seconds, err := typed.Int64()
		return time.Unix(seconds, 0), err == nil
	default:
		return time.Time{}, false
	}
}

func audienceMatches(value any, expected string) bool {
	switch typed := value.(type) {
	case string:
		return typed == expected
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text == expected {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if item == expected {
				return true
			}
		}
	}
	return false
}

func stringClaim(claims map[string]any, name string) string {
	value, ok := claims[name]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func stringListClaim(claims map[string]any, name string) []string {
	value, ok := claims[name]
	if !ok {
		return nil
	}
	return stringListValue(value)
}

func stringListValue(value any) []string {
	switch typed := value.(type) {
	case string:
		return cleanSortedStrings(strings.Split(typed, ","))
	case []string:
		return cleanSortedStrings(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return cleanSortedStrings(values)
	default:
		return nil
	}
}

func parseAdminGroupRoleMapping(raw string) (map[string][]string, error) {
	var flexible map[string]any
	if err := json.Unmarshal([]byte(raw), &flexible); err != nil {
		return nil, err
	}
	mapping := make(map[string][]string, len(flexible))
	for group, roles := range flexible {
		cleanGroup := strings.TrimSpace(group)
		if cleanGroup == "" {
			continue
		}
		mapping[cleanGroup] = stringListValue(roles)
	}
	return mapping, nil
}

func normalizeAdminGroupRoleMapping(mapping map[string][]string) map[string][]string {
	normalized := make(map[string][]string, len(mapping))
	for group, roles := range mapping {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		normalized[group] = normalizeAdminRoles(roles)
	}
	return normalized
}

func rolesForAdminGroups(groups []string, mapping map[string][]string) []string {
	roles := make([]string, 0)
	for _, group := range groups {
		roles = append(roles, mapping[strings.TrimSpace(group)]...)
	}
	return normalizeAdminRoles(roles)
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

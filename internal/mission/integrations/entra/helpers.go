package entra

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeIssuer normalizes and validates an Azure Entra issuer URL
func NormalizeIssuer(issuer string) (string, error) {
	if issuer == "" {
		return "", fmt.Errorf("issuer is required")
	}
	issues := strings.TrimSpace(issuer)
	u, err := url.Parse(issues)
	if err != nil {
		return "", fmt.Errorf("invalid issuer URL: %w", err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("issuer must use https scheme")
	}
	// Normalize by removing trailing slash
	normalized := strings.TrimSuffix(issues, "/")
	return normalized, nil
}

// DiscoveryURL returns the standard Entra discovery endpoint for an issuer
func DiscoveryURL(issuer string) string {
	return issuer + "/.well-known/openid-configuration"
}

// JWKSURI returns the standard Entra JWKS endpoint for an issuer
func JWKSURI(issuer string) string {
	return issuer + "/discovery/v2.0/keys"
}

// ExtractClaimContext extracts claims from a resolution request and registration
func ExtractClaimContext(req ResolveAuthorityContextRequest, reg AppRegistration) (issuer, clientID, subject string, groups, roles []string, err error) {
	issuer = firstString(req.Issuer, StringClaim(req.Claims, "iss"))
	clientID = firstString(req.ClientID, StringClaim(req.Claims, "appid"), StringClaim(req.Claims, "client_id"))
	subject = firstString(req.Subject, StringClaim(req.Claims, reg.SubjectClaim))

	if subject == "" {
		return "", "", "", nil, nil, fmt.Errorf("subject is required")
	}

	groups = firstStringList(req.Groups, StringListClaim(req.Claims, reg.GroupClaim))
	roles = firstStringList(req.Roles, StringListClaim(req.Claims, reg.RolesClaim))

	return
}

// StringClaim extracts a string claim from claims map
func StringClaim(claims map[string]any, key string) string {
	if claims == nil {
		return ""
	}
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// StringListClaim extracts a string list claim from claims map
func StringListClaim(claims map[string]any, key string) []string {
	if claims == nil {
		return nil
	}
	if v, ok := claims[key]; ok {
		switch val := v.(type) {
		case []string:
			return val
		case []any:
			var result []string
			for _, item := range val {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

// AudienceClaim extracts the audience as client ID
func AudienceClaim(claims map[string]any) string {
	if claims == nil {
		return ""
	}
	if v, ok := claims["aud"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// firstString returns the first non-empty string from a list
func firstString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// firstStringList returns the first non-empty string list from a list of lists
func firstStringList(lists ...[]string) []string {
	for _, list := range lists {
		if len(list) > 0 {
			return list
		}
	}
	return nil
}

// ContainsString checks if a list contains a string
func ContainsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// GroupRequirementSatisfied checks if group requirements are met
func GroupRequirementSatisfied(required []string, actual []string, mode string) bool {
	if len(required) == 0 {
		return true
	}
	if mode == GroupMatchAll {
		for _, req := range required {
			if !ContainsString(actual, req) {
				return false
			}
		}
		return true
	}
	// GroupMatchAny or default
	for _, req := range required {
		if ContainsString(actual, req) {
			return true
		}
	}
	return false
}

// HasAny checks if any item in needles exists in haystack
func HasAny(haystack []string, needles []string) bool {
	for _, needle := range needles {
		if ContainsString(haystack, needle) {
			return true
		}
	}
	return false
}

// CleanStringList removes empty strings and trims whitespace
func CleanStringList(list []string) []string {
	var result []string
	for _, item := range list {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// CloneStringMap creates a copy of a string map
func CloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// CloneContext creates a copy of a context map
func CloneContext(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

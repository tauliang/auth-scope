package okta

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func NormalizeIssuer(issuer string) (string, error) {
	issuer = strings.TrimSpace(issuer)
	issuer = strings.TrimRight(issuer, "/")
	if issuer == "" {
		return "", fmt.Errorf("issuer is required")
	}
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("issuer must be an absolute URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("issuer must use http or https")
	}
	return issuer, nil
}

func DiscoveryURL(issuer string) string {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return ""
	}
	return issuer + "/.well-known/openid-configuration"
}

func JWKSURI(issuer string) string {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Host == "" {
		return ""
	}
	if strings.HasPrefix(parsed.Path, "/oauth2/") {
		return issuer + "/v1/keys"
	}
	return parsed.Scheme + "://" + parsed.Host + "/oauth2/v1/keys"
}

func AuthorizationServerID(issuer string) string {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(issuer), "/"))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 2 && parts[0] == "oauth2" && parts[1] != "" {
		return parts[1]
	}
	return "org"
}

func ExtractClaimContext(req ResolveAuthorityContextRequest, binding AppBinding) (issuer string, clientID string, subject string, groups []string, scopes []string, err error) {
	issuer = firstString(req.Issuer, StringClaim(req.Claims, "iss"))
	issuer, err = NormalizeIssuer(issuer)
	if err != nil {
		return "", "", "", nil, nil, err
	}
	clientID = firstString(req.ClientID, StringClaim(req.Claims, "cid"), StringClaim(req.Claims, "client_id"), AudienceClaim(req.Claims))
	if strings.TrimSpace(clientID) == "" {
		return "", "", "", nil, nil, fmt.Errorf("client_id is required")
	}
	subjectClaim := binding.SubjectClaim
	if subjectClaim == "" {
		subjectClaim = "sub"
	}
	subject = firstString(req.Subject, StringClaim(req.Claims, subjectClaim), StringClaim(req.Claims, "sub"))
	if strings.TrimSpace(subject) == "" {
		return "", "", "", nil, nil, fmt.Errorf("subject is required")
	}
	groupClaim := binding.GroupClaim
	if groupClaim == "" {
		groupClaim = "groups"
	}
	groups = CleanStringList(req.Groups)
	if len(groups) == 0 {
		groups = StringsClaim(req.Claims, groupClaim)
	}
	scopeClaim := binding.ScopeClaim
	if scopeClaim == "" {
		scopeClaim = "scp"
	}
	scopes = CleanStringList(req.Scopes)
	if len(scopes) == 0 {
		scopes = StringsClaim(req.Claims, scopeClaim)
	}
	if len(scopes) == 0 {
		scopes = strings.Fields(StringClaim(req.Claims, "scope"))
	}
	return issuer, strings.TrimSpace(clientID), strings.TrimSpace(subject), groups, scopes, nil
}

func StringClaim(claims map[string]any, key string) string {
	if len(claims) == 0 || key == "" {
		return ""
	}
	switch value := claims[key].(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func AudienceClaim(claims map[string]any) string {
	if len(claims) == 0 {
		return ""
	}
	switch value := claims["aud"].(type) {
	case string:
		return strings.TrimSpace(value)
	case []string:
		if len(value) > 0 {
			return strings.TrimSpace(value[0])
		}
	case []any:
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func StringsClaim(claims map[string]any, key string) []string {
	if len(claims) == 0 || key == "" {
		return nil
	}
	switch value := claims[key].(type) {
	case []string:
		return CleanStringList(value)
	case []any:
		values := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return CleanStringList(values)
	case string:
		if strings.Contains(value, " ") {
			return CleanStringList(strings.Fields(value))
		}
		return CleanStringList([]string{value})
	default:
		return nil
	}
}

func GroupRequirementSatisfied(required []string, groups []string, mode string) bool {
	required = CleanStringList(required)
	if len(required) == 0 {
		return true
	}
	groupSet := stringSet(groups)
	switch mode {
	case GroupMatchAll:
		for _, group := range required {
			if !groupSet[group] {
				return false
			}
		}
		return true
	default:
		for _, group := range required {
			if groupSet[group] {
				return true
			}
		}
		return false
	}
}

func HasAny(values []string, candidates []string) bool {
	valueSet := stringSet(values)
	for _, candidate := range candidates {
		if valueSet[strings.TrimSpace(candidate)] {
			return true
		}
	}
	return false
}

func ContainsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	return slices.Contains(CleanStringList(values), target)
}

func CleanStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func CloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func CloneContext(values map[string]any) map[string]any {
	context := map[string]any{}
	for key, value := range values {
		context[key] = value
	}
	return context
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range CleanStringList(values) {
		set[value] = true
	}
	return set
}

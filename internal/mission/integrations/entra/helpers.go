package entra

import (
	"fmt"
	"net/url"
	"strings"
)

func NormalizeIssuer(issuer string) (string, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return "", fmt.Errorf("issuer is required")
	}
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("issuer must be an absolute URL")
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("issuer must use https scheme")
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
	path := strings.TrimSuffix(parsed.Path, "/v2.0")
	return parsed.Scheme + "://" + parsed.Host + path + "/discovery/v2.0/keys"
}

func ExtractClaimContext(req ResolveAuthorityContextRequest, reg AppRegistration) (issuer, clientID, subject string, groups, roles []string, err error) {
	issuer = firstString(req.Issuer, StringClaim(req.Claims, "iss"))
	issuer, err = NormalizeIssuer(issuer)
	if err != nil {
		return "", "", "", nil, nil, err
	}
	clientID = firstString(req.ClientID, StringClaim(req.Claims, "appid"), StringClaim(req.Claims, "azp"), StringClaim(req.Claims, "client_id"), AudienceClaim(req.Claims))
	if strings.TrimSpace(clientID) == "" {
		return "", "", "", nil, nil, fmt.Errorf("client_id is required")
	}
	subjectClaim := reg.SubjectClaim
	if subjectClaim == "" {
		subjectClaim = "sub"
	}
	subject = firstString(req.Subject, StringClaim(req.Claims, subjectClaim), StringClaim(req.Claims, "oid"), StringClaim(req.Claims, "sub"))
	if strings.TrimSpace(subject) == "" {
		return "", "", "", nil, nil, fmt.Errorf("subject is required")
	}
	groupClaim := reg.GroupClaim
	if groupClaim == "" {
		groupClaim = "groups"
	}
	rolesClaim := reg.RolesClaim
	if rolesClaim == "" {
		rolesClaim = "roles"
	}
	groups = CleanStringList(req.Groups)
	if len(groups) == 0 {
		groups = StringListClaim(req.Claims, groupClaim)
	}
	roles = CleanStringList(req.Roles)
	if len(roles) == 0 {
		roles = StringListClaim(req.Claims, rolesClaim)
	}
	return issuer, strings.TrimSpace(clientID), strings.TrimSpace(subject), groups, roles, nil
}

func StringClaim(claims map[string]any, key string) string {
	if len(claims) == 0 || key == "" {
		return ""
	}
	if value, ok := claims[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func StringListClaim(claims map[string]any, key string) []string {
	if len(claims) == 0 || key == "" {
		return nil
	}
	if v, ok := claims[key]; ok {
		switch val := v.(type) {
		case []string:
			return CleanStringList(val)
		case []any:
			var result []string
			for _, item := range val {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return CleanStringList(result)
		case string:
			if strings.Contains(val, " ") {
				return CleanStringList(strings.Fields(val))
			}
			return CleanStringList([]string{val})
		}
	}
	return nil
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

func firstString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func ContainsString(list []string, s string) bool {
	s = strings.TrimSpace(s)
	for _, item := range list {
		if strings.TrimSpace(item) == s {
			return true
		}
	}
	return false
}

func GroupRequirementSatisfied(required []string, actual []string, mode string) bool {
	required = CleanStringList(required)
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

func HasAny(haystack []string, needles []string) bool {
	for _, needle := range needles {
		if ContainsString(haystack, needle) {
			return true
		}
	}
	return false
}

func CleanStringList(list []string) []string {
	var result []string
	for _, item := range list {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

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

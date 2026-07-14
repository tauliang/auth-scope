package salesforce

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func NormalizeInstanceURL(instanceURL string) (string, error) {
	instanceURL = strings.TrimSpace(instanceURL)
	if instanceURL == "" {
		return "", fmt.Errorf("instance_url is required")
	}
	parsed, err := url.Parse(instanceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("instance_url must be an absolute URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("instance_url must use http or https")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("instance_url must not include credentials")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func NormalizeObjectAPIName(objectAPIName string) string {
	return strings.TrimSpace(objectAPIName)
}

func ExtractUserContext(req AuthorizeRecordActionRequest, binding OrgBinding) (userID string, subject string, username string, email string, profile string, permissionSets []string, err error) {
	subjectClaim := firstString(binding.SubjectClaim, "sub")
	usernameClaim := firstString(binding.UsernameClaim, "username")
	emailClaim := firstString(binding.EmailClaim, "email")
	profileClaim := firstString(binding.ProfileClaim, "profile")
	permissionSetsClaim := firstString(binding.PermissionSetsClaim, "permission_sets")

	userID = firstString(req.UserID, StringClaim(req.Claims, "user_id"), StringClaim(req.Claims, "userId"))
	subject = firstString(req.Subject, StringClaim(req.Claims, subjectClaim), userID)
	username = firstString(req.Username, StringClaim(req.Claims, usernameClaim), StringClaim(req.Claims, "preferred_username"))
	email = firstString(req.Email, StringClaim(req.Claims, emailClaim))
	profile = firstString(req.Profile, StringClaim(req.Claims, profileClaim), StringClaim(req.Claims, "profile_name"))
	permissionSets = CleanStringList(req.PermissionSets)
	if len(permissionSets) == 0 {
		permissionSets = StringsClaim(req.Claims, permissionSetsClaim)
	}
	if subject == "" && userID == "" && username == "" && email == "" {
		return "", "", "", "", "", nil, fmt.Errorf("subject, user_id, username, or email is required")
	}
	return userID, subject, username, email, profile, permissionSets, nil
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
		value = strings.ReplaceAll(value, ",", " ")
		return CleanStringList(strings.Fields(value))
	default:
		return nil
	}
}

func PermissionSetRequirementSatisfied(required []string, permissionSets []string, mode string) bool {
	required = CleanStringList(required)
	if len(required) == 0 {
		return true
	}
	set := stringSet(permissionSets)
	switch mode {
	case PermissionMatchAll:
		for _, permissionSet := range required {
			if !set[strings.ToLower(permissionSet)] {
				return false
			}
		}
		return true
	default:
		for _, permissionSet := range required {
			if set[strings.ToLower(permissionSet)] {
				return true
			}
		}
		return false
	}
}

func HasAny(values []string, candidates []string) bool {
	valueSet := stringSet(values)
	for _, candidate := range candidates {
		if valueSet[strings.ToLower(strings.TrimSpace(candidate))] {
			return true
		}
	}
	return false
}

func ContainsString(values []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	return slices.Contains(cleanLowerStringList(values), target)
}

func ContainsAnyIdentity(allowed []string, candidates ...string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range candidates {
		if ContainsString(allowed, candidate) {
			return true
		}
	}
	return false
}

func CleanStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, value)
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range CleanStringList(values) {
		set[strings.ToLower(value)] = true
	}
	return set
}

func cleanLowerStringList(values []string) []string {
	cleaned := CleanStringList(values)
	for i := range cleaned {
		cleaned[i] = strings.ToLower(cleaned[i])
	}
	return cleaned
}

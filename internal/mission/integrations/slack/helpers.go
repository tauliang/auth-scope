package slack

import (
	"fmt"
	"slices"
	"strings"
)

func ExtractUserContext(req AuthorizeMessageActionRequest, binding WorkspaceBinding) (userID string, email string, roles []string, err error) {
	userID = strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = StringClaim(req.Claims, "sub")
	}
	if userID == "" {
		userID = StringClaim(req.Claims, "user_id")
	}
	if userID == "" {
		return "", "", nil, fmt.Errorf("user_id is required")
	}

	email = strings.TrimSpace(req.Email)
	if email == "" {
		email = StringClaim(req.Claims, "email")
	}

	rollClaim := binding.RoleClaim
	if rollClaim == "" {
		rollClaim = "roles"
	}
	roles = CleanStringList(req.Roles)
	if len(roles) == 0 {
		roles = StringsClaim(req.Claims, rollClaim)
	}

	return userID, email, roles, nil
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
		if strings.Contains(value, " ") {
			return CleanStringList(strings.Fields(value))
		}
		return CleanStringList([]string{value})
	default:
		return nil
	}
}

func RoleRequirementSatisfied(required []string, roles []string, mode string) bool {
	required = CleanStringList(required)
	if len(required) == 0 {
		return true
	}
	roleSet := stringSet(roles)
	switch mode {
	case RoleMatchAll:
		for _, role := range required {
			if !roleSet[role] {
				return false
			}
		}
		return true
	default:
		for _, role := range required {
			if roleSet[role] {
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

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range CleanStringList(values) {
		set[value] = true
	}
	return set
}

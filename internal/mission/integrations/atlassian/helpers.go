package atlassian

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func NormalizeSiteURL(siteURL string) (string, error) {
	siteURL = strings.TrimSpace(siteURL)
	if siteURL == "" {
		return "", fmt.Errorf("site_url is required")
	}
	parsed, err := url.Parse(siteURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("site_url must be an absolute URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("site_url must use http or https")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("site_url must not include credentials")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func NormalizeProjectKey(projectKey string) string {
	return strings.ToUpper(strings.TrimSpace(projectKey))
}

func NormalizeSpaceKey(spaceKey string) string {
	return strings.ToUpper(strings.TrimSpace(spaceKey))
}

func ProjectKeyFromIssueKey(issueKey string) string {
	issueKey = strings.ToUpper(strings.TrimSpace(issueKey))
	if idx := strings.Index(issueKey, "-"); idx > 0 {
		return issueKey[:idx]
	}
	return ""
}

func ExtractJiraUserContext(req AuthorizeJiraIssueActionRequest, binding SiteBinding) (accountID string, subject string, email string, groups []string, err error) {
	return extractUserContext(userContextInput{
		AccountID:    req.AccountID,
		Subject:      req.Subject,
		Email:        req.Email,
		Groups:       req.Groups,
		Claims:       req.Claims,
		SubjectClaim: binding.SubjectClaim,
		EmailClaim:   binding.EmailClaim,
		GroupClaim:   binding.GroupClaim,
	})
}

func ExtractConfluenceUserContext(req AuthorizeConfluencePageActionRequest, binding SiteBinding) (accountID string, subject string, email string, groups []string, err error) {
	return extractUserContext(userContextInput{
		AccountID:    req.AccountID,
		Subject:      req.Subject,
		Email:        req.Email,
		Groups:       req.Groups,
		Claims:       req.Claims,
		SubjectClaim: binding.SubjectClaim,
		EmailClaim:   binding.EmailClaim,
		GroupClaim:   binding.GroupClaim,
	})
}

type userContextInput struct {
	AccountID    string
	Subject      string
	Email        string
	Groups       []string
	Claims       map[string]any
	SubjectClaim string
	EmailClaim   string
	GroupClaim   string
}

func extractUserContext(input userContextInput) (accountID string, subject string, email string, groups []string, err error) {
	subjectClaim := firstString(input.SubjectClaim, "sub")
	emailClaim := firstString(input.EmailClaim, "email")
	groupClaim := firstString(input.GroupClaim, "groups")

	accountID = firstString(input.AccountID, StringClaim(input.Claims, "account_id"), StringClaim(input.Claims, "accountId"))
	subject = firstString(input.Subject, StringClaim(input.Claims, subjectClaim), accountID)
	email = firstString(input.Email, StringClaim(input.Claims, emailClaim))
	groups = CleanStringList(input.Groups)
	if len(groups) == 0 {
		groups = StringsClaim(input.Claims, groupClaim)
	}
	if subject == "" && email == "" && accountID == "" {
		return "", "", "", nil, fmt.Errorf("subject, account_id, or email is required")
	}
	return accountID, subject, email, groups, nil
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

func GroupRequirementSatisfied(required []string, groups []string, mode string) bool {
	required = CleanStringList(required)
	if len(required) == 0 {
		return true
	}
	groupSet := stringSet(groups)
	switch mode {
	case GroupMatchAll:
		for _, group := range required {
			if !groupSet[strings.ToLower(group)] {
				return false
			}
		}
		return true
	default:
		for _, group := range required {
			if groupSet[strings.ToLower(group)] {
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

func CleanKeyList(values []string) []string {
	cleaned := CleanStringList(values)
	for i := range cleaned {
		cleaned[i] = strings.ToUpper(cleaned[i])
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

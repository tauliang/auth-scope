package mission

import (
	"sort"
	"strings"
)

type AdminRole string

const (
	AdminRoleOwner            AdminRole = "owner"
	AdminRoleApprover         AdminRole = "approver"
	AdminRoleAuditor          AdminRole = "auditor"
	AdminRoleOperator         AdminRole = "operator"
	AdminRoleIntegrationAdmin AdminRole = "integration-admin"
)

type AdminPermission string

const (
	AdminPermissionRead               AdminPermission = "admin:read"
	AdminPermissionApprove            AdminPermission = "missions:approve"
	AdminPermissionOperate            AdminPermission = "missions:operate"
	AdminPermissionManageGovernance   AdminPermission = "governance:manage"
	AdminPermissionManageIntegrations AdminPermission = "integrations:manage"
)

var allAdminPermissions = []AdminPermission{
	AdminPermissionRead,
	AdminPermissionApprove,
	AdminPermissionOperate,
	AdminPermissionManageGovernance,
	AdminPermissionManageIntegrations,
}

type AdminIdentity struct {
	Principal   Principal `json:"principal"`
	Provider    string    `json:"provider,omitempty"`
	Groups      []string  `json:"groups,omitempty"`
	Roles       []string  `json:"roles,omitempty"`
	Permissions []string  `json:"permissions,omitempty"`
}

func newAdminIdentity(principal Principal, provider string, groups []string, roles []string, permissions []string, legacyOwner bool) AdminIdentity {
	identity := AdminIdentity{
		Principal: principal,
		Provider:  strings.TrimSpace(provider),
		Groups:    cleanSortedStrings(groups),
		Roles:     normalizeAdminRoles(roles),
	}
	identity.Permissions = normalizeAdminPermissions(permissions)
	if legacyOwner && len(identity.Roles) == 0 && len(identity.Permissions) == 0 {
		identity.Roles = []string{string(AdminRoleOwner)}
	}
	identity.Permissions = normalizeAdminPermissions(append(identity.Permissions, rolePermissionStrings(identity.Roles)...))
	return identity
}

func legacyAdminIdentity(principal Principal) AdminIdentity {
	return newAdminIdentity(principal, "static", nil, nil, nil, true)
}

func AdminCapabilitiesForIdentity(identity AdminIdentity) map[string]bool {
	return map[string]bool{
		"approve":      adminIdentityHasPermission(identity, AdminPermissionApprove),
		"audit":        adminIdentityHasPermission(identity, AdminPermissionRead),
		"containment":  adminIdentityHasPermission(identity, AdminPermissionManageGovernance),
		"governance":   adminIdentityHasPermission(identity, AdminPermissionManageGovernance),
		"integrations": adminIdentityHasPermission(identity, AdminPermissionManageIntegrations),
		"operate":      adminIdentityHasPermission(identity, AdminPermissionOperate),
		"revoke":       adminIdentityHasPermission(identity, AdminPermissionOperate),
	}
}

func adminIdentityHasPermission(identity AdminIdentity, permission AdminPermission) bool {
	if permission == "" {
		return true
	}
	for _, current := range identity.Permissions {
		if current == string(permission) {
			return true
		}
	}
	return false
}

func normalizeAdminRoles(roles []string) []string {
	normalized := make([]string, 0, len(roles))
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "", "none":
			continue
		case "admin", "administrator", "security-admin", "super-admin":
			normalized = append(normalized, string(AdminRoleOwner))
		case string(AdminRoleOwner):
			normalized = append(normalized, string(AdminRoleOwner))
		case string(AdminRoleApprover):
			normalized = append(normalized, string(AdminRoleApprover))
		case string(AdminRoleAuditor):
			normalized = append(normalized, string(AdminRoleAuditor))
		case string(AdminRoleOperator):
			normalized = append(normalized, string(AdminRoleOperator))
		case string(AdminRoleIntegrationAdmin), "integration_admin", "integration admin":
			normalized = append(normalized, string(AdminRoleIntegrationAdmin))
		default:
			normalized = append(normalized, strings.ToLower(strings.TrimSpace(role)))
		}
	}
	return cleanSortedStrings(normalized)
}

func normalizeAdminPermissions(permissions []string) []string {
	normalized := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		value := strings.ToLower(strings.TrimSpace(permission))
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return cleanSortedStrings(normalized)
}

func rolePermissionStrings(roles []string) []string {
	permissions := make([]string, 0, len(roles)*len(allAdminPermissions))
	for _, role := range roles {
		switch AdminRole(strings.ToLower(strings.TrimSpace(role))) {
		case AdminRoleOwner:
			for _, permission := range allAdminPermissions {
				permissions = append(permissions, string(permission))
			}
		case AdminRoleApprover:
			permissions = append(permissions, string(AdminPermissionRead), string(AdminPermissionApprove))
		case AdminRoleAuditor:
			permissions = append(permissions, string(AdminPermissionRead))
		case AdminRoleOperator:
			permissions = append(permissions, string(AdminPermissionRead), string(AdminPermissionOperate), string(AdminPermissionManageGovernance))
		case AdminRoleIntegrationAdmin:
			permissions = append(permissions, string(AdminPermissionRead), string(AdminPermissionManageIntegrations))
		}
	}
	return permissions
}

func cleanSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	sort.Strings(cleaned)
	return cleaned
}

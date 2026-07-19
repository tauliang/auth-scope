package slack

import "testing"

func TestExtractUserContextUsesClaimsAndConfiguredRoleClaim(t *testing.T) {
	userID, email, roles, err := ExtractUserContext(AuthorizeMessageActionRequest{
		Claims: map[string]any{
			"sub":         " U12345678 ",
			"email":       " user@example.com ",
			"slack_roles": []any{" Workspace Admin ", "Owner", ""},
		},
	}, WorkspaceBinding{RoleClaim: "slack_roles"})
	if err != nil {
		t.Fatalf("ExtractUserContext: %v", err)
	}
	if userID != "U12345678" || email != "user@example.com" {
		t.Fatalf("identity = %q %q", userID, email)
	}
	if len(roles) != 2 || roles[0] != "Workspace Admin" || roles[1] != "Owner" {
		t.Fatalf("roles = %#v", roles)
	}
}

func TestRoleRequirementSatisfiedModes(t *testing.T) {
	if !RoleRequirementSatisfied([]string{"Admin"}, []string{"Admin"}, RoleMatchAny) {
		t.Fatal("expected any match to satisfy")
	}
	if RoleRequirementSatisfied([]string{"Admin", "Owner"}, []string{"Admin"}, RoleMatchAll) {
		t.Fatal("expected all match to require every role")
	}
	if !RoleRequirementSatisfied([]string{"Admin", "Owner"}, []string{"Admin", "Owner"}, RoleMatchAll) {
		t.Fatal("expected all roles to satisfy all mode")
	}
}

func TestStringsClaimCleansSpaceDelimitedValues(t *testing.T) {
	got := StringsClaim(map[string]any{"roles": "Admin Owner"}, "roles")
	if len(got) != 2 || got[0] != "Admin" || got[1] != "Owner" {
		t.Fatalf("StringsClaim = %#v", got)
	}
}

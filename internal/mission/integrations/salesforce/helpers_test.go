package salesforce

import "testing"

func TestSalesforceNormalizeAndClaimHelpers(t *testing.T) {
	instanceURL, err := NormalizeInstanceURL(" HTTPS://ACME.my.salesforce.com/services/oauth2?token=true#frag ")
	if err != nil {
		t.Fatalf("NormalizeInstanceURL: %v", err)
	}
	if instanceURL != "https://acme.my.salesforce.com" {
		t.Fatalf("instanceURL = %q, want normalized root URL", instanceURL)
	}
	for _, bad := range []string{"", "acme.my.salesforce.com", "ftp://acme.my.salesforce.com", "https://user:pass@acme.my.salesforce.com"} {
		if _, err := NormalizeInstanceURL(bad); err == nil {
			t.Fatalf("NormalizeInstanceURL(%q) succeeded, want validation error", bad)
		}
	}
	if got := NormalizeObjectAPIName(" Account "); got != "Account" {
		t.Fatalf("object API name = %q, want Account", got)
	}
	permissionSets := StringsClaim(map[string]any{"permission_sets": "CRM_Agent, Deal Desk"}, "permission_sets")
	if len(permissionSets) != 3 || permissionSets[0] != "CRM_Agent" {
		t.Fatalf("permission sets = %#v, want comma/space split", permissionSets)
	}
	if permissionSets = StringsClaim(map[string]any{"permission_sets": []any{"CRM_Agent", " Deal Desk "}}, "permission_sets"); len(permissionSets) != 2 || permissionSets[1] != "Deal Desk" {
		t.Fatalf("permission sets = %#v, want cleaned []any values", permissionSets)
	}
	if StringsClaim(map[string]any{"permission_sets": 42}, "permission_sets") != nil {
		t.Fatal("expected unsupported claim to return nil")
	}
	if !PermissionSetRequirementSatisfied([]string{"crm_agent"}, []string{"CRM_Agent"}, PermissionMatchAny) {
		t.Fatal("expected case-insensitive permission set match")
	}
	if PermissionSetRequirementSatisfied([]string{"crm_agent", "security"}, []string{"CRM_Agent"}, PermissionMatchAll) {
		t.Fatal("unexpected all-mode permission set match")
	}
	if PermissionSetRequirementSatisfied([]string{"security"}, []string{"CRM_Agent"}, PermissionMatchAny) {
		t.Fatal("unexpected any-mode permission set match")
	}
	if HasAny([]string{"CRM_Agent"}, []string{"Security"}) {
		t.Fatal("unexpected candidate match")
	}
	if !ContainsAnyIdentity([]string{"agent@example.com"}, "005xx", "subject", "agent@example.com") {
		t.Fatal("expected allowed identity to match email candidate")
	}
	if !ContainsAnyIdentity(nil, "anyone") {
		t.Fatal("empty allowed identity list should allow any candidate")
	}
	if ContainsAnyIdentity([]string{"agent@example.com"}, "other@example.com") {
		t.Fatal("unexpected identity match")
	}
	if _, _, _, _, _, _, err := ExtractUserContext(AuthorizeRecordActionRequest{}, OrgBinding{}); err == nil {
		t.Fatal("expected missing identity context error")
	}
	if CloneStringMap(nil) != nil {
		t.Fatal("expected nil string map clone")
	}
	if firstString(" ", "\t") != "" {
		t.Fatal("expected blank firstString result")
	}
}

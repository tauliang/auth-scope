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
	permissionSets := StringsClaim(map[string]any{"permission_sets": "CRM_Agent, Deal Desk"}, "permission_sets")
	if len(permissionSets) != 3 || permissionSets[0] != "CRM_Agent" {
		t.Fatalf("permission sets = %#v, want comma/space split", permissionSets)
	}
	if !PermissionSetRequirementSatisfied([]string{"crm_agent"}, []string{"CRM_Agent"}, PermissionMatchAny) {
		t.Fatal("expected case-insensitive permission set match")
	}
	if !ContainsAnyIdentity([]string{"agent@example.com"}, "005xx", "subject", "agent@example.com") {
		t.Fatal("expected allowed identity to match email candidate")
	}
}

package atlassian

import "testing"

func TestAtlassianNormalizeAndClaimHelpers(t *testing.T) {
	siteURL, err := NormalizeSiteURL(" HTTPS://ACME.atlassian.net/wiki/spaces/ENG?draft=true#page ")
	if err != nil {
		t.Fatalf("NormalizeSiteURL: %v", err)
	}
	if siteURL != "https://acme.atlassian.net" {
		t.Fatalf("siteURL = %q, want normalized URL", siteURL)
	}
	if got := NormalizeProjectKey(" fin "); got != "FIN" {
		t.Fatalf("project key = %q, want FIN", got)
	}
	if got := ProjectKeyFromIssueKey("fin-123"); got != "FIN" {
		t.Fatalf("project from issue = %q, want FIN", got)
	}
	groups := StringsClaim(map[string]any{"groups": []any{"Ops", " Finance "}}, "groups")
	if len(groups) != 2 || groups[1] != "Finance" {
		t.Fatalf("groups = %#v, want cleaned groups", groups)
	}
	if !GroupRequirementSatisfied([]string{"ops"}, []string{"Ops"}, GroupMatchAny) {
		t.Fatal("expected case-insensitive group match")
	}
	if !ContainsAnyIdentity([]string{"agent@example.com"}, "acc_123", "subject", "agent@example.com") {
		t.Fatal("expected allowed identity to match email candidate")
	}
}

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
	if got := ProjectKeyFromIssueKey("no-ticket-number"); got != "NO" {
		t.Fatalf("project from issue = %q, want NO", got)
	}
	if got := ProjectKeyFromIssueKey("badissuekey"); got != "" {
		t.Fatalf("project from malformed issue = %q, want empty", got)
	}
	for _, bad := range []string{"", "acme.atlassian.net", "ftp://acme.atlassian.net", "https://user:pass@acme.atlassian.net"} {
		if _, err := NormalizeSiteURL(bad); err == nil {
			t.Fatalf("NormalizeSiteURL(%q) succeeded, want validation error", bad)
		}
	}
	groups := StringsClaim(map[string]any{"groups": []any{"Ops", " Finance "}}, "groups")
	if len(groups) != 2 || groups[1] != "Finance" {
		t.Fatalf("groups = %#v, want cleaned groups", groups)
	}
	if groups = StringsClaim(map[string]any{"groups": "Ops Finance"}, "groups"); len(groups) != 2 || groups[0] != "Ops" {
		t.Fatalf("string groups = %#v, want split groups", groups)
	}
	if StringsClaim(map[string]any{"groups": 42}, "groups") != nil {
		t.Fatal("expected unsupported claim to return nil")
	}
	if !GroupRequirementSatisfied([]string{"ops"}, []string{"Ops"}, GroupMatchAny) {
		t.Fatal("expected case-insensitive group match")
	}
	if GroupRequirementSatisfied([]string{"ops", "security"}, []string{"Ops"}, GroupMatchAll) {
		t.Fatal("unexpected all-mode group match")
	}
	if GroupRequirementSatisfied([]string{"security"}, []string{"Ops"}, GroupMatchAny) {
		t.Fatal("unexpected any-mode group match")
	}
	if HasAny([]string{"Ops"}, []string{"Security"}) {
		t.Fatal("unexpected candidate match")
	}
	if !ContainsAnyIdentity([]string{"agent@example.com"}, "acc_123", "subject", "agent@example.com") {
		t.Fatal("expected allowed identity to match email candidate")
	}
	if !ContainsAnyIdentity(nil, "anyone") {
		t.Fatal("empty allowed identity list should allow any candidate")
	}
	if ContainsAnyIdentity([]string{"agent@example.com"}, "other@example.com") {
		t.Fatal("unexpected identity match")
	}
	if _, _, _, _, err := ExtractJiraUserContext(AuthorizeJiraIssueActionRequest{}, SiteBinding{}); err == nil {
		t.Fatal("expected missing identity context error")
	}
	if CloneStringMap(nil) != nil {
		t.Fatal("expected nil string map clone")
	}
	if firstString(" ", "\t") != "" {
		t.Fatal("expected blank firstString result")
	}
}

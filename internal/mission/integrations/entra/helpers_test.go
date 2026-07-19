package entra

import "testing"

func TestIssuerMetadataURLs(t *testing.T) {
	issuer, err := NormalizeIssuer("https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0/")
	if err != nil {
		t.Fatalf("NormalizeIssuer: %v", err)
	}
	if issuer != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0" {
		t.Fatalf("issuer = %q", issuer)
	}
	if DiscoveryURL(issuer) != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0/.well-known/openid-configuration" {
		t.Fatalf("discovery URL = %q", DiscoveryURL(issuer))
	}
	if JWKSURI(issuer) != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/discovery/v2.0/keys" {
		t.Fatalf("jwks URI = %q", JWKSURI(issuer))
	}
}

func TestExtractClaimContextAndGroupMatching(t *testing.T) {
	registration := AppRegistration{
		GroupClaim:     "groups",
		SubjectClaim:   "sub",
		RolesClaim:     "roles",
		GroupMatchMode: GroupMatchAll,
	}
	issuer, clientID, subject, groups, roles, err := ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
			"azp":    "00000000-0000-0000-0000-000000000000",
			"sub":    "user@example.onmicrosoft.com",
			"groups": []any{"Mission Operators", "Mission Admins"},
			"roles":  []any{"Reader", "Contributor"},
		},
	}, registration)
	if err != nil {
		t.Fatalf("ExtractClaimContext: %v", err)
	}
	if issuer != "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0" ||
		clientID != "00000000-0000-0000-0000-000000000000" ||
		subject != "user@example.onmicrosoft.com" {
		t.Fatalf("unexpected identity context: issuer=%q client=%q subject=%q", issuer, clientID, subject)
	}
	if !GroupRequirementSatisfied([]string{"Mission Operators", "Mission Admins"}, groups, GroupMatchAll) {
		t.Fatalf("expected all required groups to match: %#v", groups)
	}
	if GroupRequirementSatisfied([]string{"Mission Operators", "Security"}, groups, GroupMatchAll) {
		t.Fatalf("expected missing group to fail all mode: %#v", groups)
	}
	if !HasAny(groups, []string{"Security", "Mission Admins"}) {
		t.Fatalf("expected admin group intersection: %#v", groups)
	}
	if len(roles) != 2 || roles[1] != "Contributor" {
		t.Fatalf("unexpected roles: %#v", roles)
	}
}

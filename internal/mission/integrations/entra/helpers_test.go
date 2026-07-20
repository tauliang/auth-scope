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
	if DiscoveryURL(" ") != "" {
		t.Fatal("expected empty discovery URL for blank issuer")
	}
	if JWKSURI("://bad") != "" {
		t.Fatal("expected empty JWKS URI for malformed issuer")
	}
	for _, bad := range []string{"", "login.microsoftonline.com/tenant/v2.0", "http://login.microsoftonline.com/tenant/v2.0"} {
		if _, err := NormalizeIssuer(bad); err == nil {
			t.Fatalf("NormalizeIssuer(%q) succeeded, want validation error", bad)
		}
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

func TestEntraClaimHelperEdgeCases(t *testing.T) {
	registration := AppRegistration{}
	_, _, _, _, _, err := ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://login.microsoftonline.com/tenant/v2.0", "sub": "user"},
	}, registration)
	if err == nil {
		t.Fatal("expected missing client_id error")
	}
	_, _, _, _, _, err = ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://login.microsoftonline.com/tenant/v2.0", "aud": []any{"client"}},
	}, registration)
	if err == nil {
		t.Fatal("expected missing subject error")
	}

	_, clientID, subject, groups, roles, err := ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://login.microsoftonline.com/tenant/v2.0",
			"aud":    []string{"client"},
			"oid":    "oid-user",
			"groups": "Mission Operators",
			"roles":  "Reader Contributor",
		},
	}, registration)
	if err != nil {
		t.Fatalf("ExtractClaimContext defaults: %v", err)
	}
	if clientID != "client" || subject != "oid-user" || len(groups) != 2 || len(roles) != 2 {
		t.Fatalf("context client=%q subject=%q groups=%#v roles=%#v", clientID, subject, groups, roles)
	}
	if AudienceClaim(map[string]any{"aud": []any{"", "client-from-any"}}) != "client-from-any" {
		t.Fatal("expected audience from non-empty []any entry")
	}
	if AudienceClaim(map[string]any{"aud": []string{}}) != "" {
		t.Fatal("expected empty audience from empty slice")
	}
	if StringListClaim(map[string]any{"groups": 42}, "groups") != nil {
		t.Fatal("expected unsupported list claim to return nil")
	}
	if GroupRequirementSatisfied([]string{"Admin"}, []string{"Member"}, GroupMatchAny) {
		t.Fatal("unexpected group match")
	}
	if HasAny([]string{"Member"}, []string{"Admin"}) {
		t.Fatal("unexpected intersection")
	}
	if ContainsString([]string{" Member "}, "Admin") {
		t.Fatal("unexpected contains match")
	}
	if firstString(" ", "\t") != "" {
		t.Fatal("expected blank firstString result")
	}
}

package okta

import "testing"

func TestIssuerMetadataURLs(t *testing.T) {
	issuer, err := NormalizeIssuer("https://acme.okta.com/oauth2/default/")
	if err != nil {
		t.Fatalf("NormalizeIssuer custom: %v", err)
	}
	if issuer != "https://acme.okta.com/oauth2/default" {
		t.Fatalf("issuer = %q", issuer)
	}
	if AuthorizationServerID(issuer) != "default" {
		t.Fatalf("authorization server id = %q", AuthorizationServerID(issuer))
	}
	if DiscoveryURL(issuer) != "https://acme.okta.com/oauth2/default/.well-known/openid-configuration" {
		t.Fatalf("custom discovery URL = %q", DiscoveryURL(issuer))
	}
	if JWKSURI(issuer) != "https://acme.okta.com/oauth2/default/v1/keys" {
		t.Fatalf("custom jwks URI = %q", JWKSURI(issuer))
	}

	orgIssuer := "https://acme.okta.com"
	if AuthorizationServerID(orgIssuer) != "org" {
		t.Fatalf("org authorization server id = %q", AuthorizationServerID(orgIssuer))
	}
	if JWKSURI(orgIssuer) != "https://acme.okta.com/oauth2/v1/keys" {
		t.Fatalf("org jwks URI = %q", JWKSURI(orgIssuer))
	}
	if DiscoveryURL(" ") != "" {
		t.Fatal("expected empty discovery URL for blank issuer")
	}
	if JWKSURI("://bad") != "" {
		t.Fatal("expected empty JWKS URI for malformed issuer")
	}
	if AuthorizationServerID("://bad") != "" {
		t.Fatal("expected empty authorization server id for malformed issuer")
	}
	for _, bad := range []string{"", "acme.okta.com", "ftp://acme.okta.com"} {
		if _, err := NormalizeIssuer(bad); err == nil {
			t.Fatalf("NormalizeIssuer(%q) succeeded, want validation error", bad)
		}
	}
}

func TestExtractClaimContextAndGroupMatching(t *testing.T) {
	binding := AppBinding{
		GroupClaim:     "groups",
		SubjectClaim:   "sub",
		ScopeClaim:     "scp",
		GroupMatchMode: GroupMatchAll,
	}
	issuer, clientID, subject, groups, scopes, err := ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://acme.okta.com/oauth2/default",
			"cid":    "0oaabc123client",
			"sub":    "00u1agent",
			"groups": []any{"Mission Operators", "Mission Admins"},
			"scp":    []any{"openid", "groups"},
		},
	}, binding)
	if err != nil {
		t.Fatalf("ExtractClaimContext: %v", err)
	}
	if issuer != "https://acme.okta.com/oauth2/default" || clientID != "0oaabc123client" || subject != "00u1agent" {
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
	if len(scopes) != 2 || scopes[1] != "groups" {
		t.Fatalf("unexpected scopes: %#v", scopes)
	}
}

func TestOktaClaimHelperEdgeCases(t *testing.T) {
	binding := AppBinding{}
	_, _, _, _, _, err := ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://acme.okta.com", "sub": "00u1agent"},
	}, binding)
	if err == nil {
		t.Fatal("expected missing client_id error")
	}
	_, _, _, _, _, err = ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{"iss": "https://acme.okta.com", "aud": []any{"0oaabc123client"}},
	}, binding)
	if err == nil {
		t.Fatal("expected missing subject error")
	}

	issuer, clientID, subject, groups, scopes, err := ExtractClaimContext(ResolveAuthorityContextRequest{
		Claims: map[string]any{
			"iss":    "https://acme.okta.com",
			"aud":    []string{"0oaabc123client"},
			"sub":    "00u1agent",
			"groups": "Mission Operators",
			"scope":  "openid profile",
		},
	}, binding)
	if err != nil {
		t.Fatalf("ExtractClaimContext defaults: %v", err)
	}
	if issuer != "https://acme.okta.com" || clientID != "0oaabc123client" || subject != "00u1agent" {
		t.Fatalf("identity context = %q %q %q", issuer, clientID, subject)
	}
	if len(groups) != 2 || groups[1] != "Operators" || len(scopes) != 2 || scopes[1] != "profile" {
		t.Fatalf("groups/scopes = %#v/%#v, want split strings", groups, scopes)
	}
	if AudienceClaim(map[string]any{"aud": []any{"", "client-from-any"}}) != "client-from-any" {
		t.Fatal("expected audience from non-empty []any entry")
	}
	if AudienceClaim(map[string]any{"aud": []string{}}) != "" {
		t.Fatal("expected empty audience from empty slice")
	}
	if StringsClaim(map[string]any{"groups": 42}, "groups") != nil {
		t.Fatal("expected unsupported string-list claim to return nil")
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
	if CloneStringMap(nil) != nil {
		t.Fatal("expected nil string map clone")
	}
	if firstString(" ", "\t") != "" {
		t.Fatal("expected blank firstString result")
	}
}

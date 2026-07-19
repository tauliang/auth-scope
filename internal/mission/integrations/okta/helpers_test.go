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

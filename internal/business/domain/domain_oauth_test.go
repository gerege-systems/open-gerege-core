// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "testing"

func testClient() OAuthClient {
	return OAuthClient{
		ClientID:                "ring-dgov-mn",
		TokenEndpointAuthMethod: AuthMethodBasic,
		GrantTypes:              []string{GrantAuthorizationCode, GrantRefreshToken},
		Scopes:                  []string{"openid", "profile", "email", "svc:eid-proxy"},
		RedirectURIs:            []string{"https://ring.dgov.mn/sso/callback"},
		PostLogoutRedirectURIs:  []string{"https://ring.dgov.mn/"},
	}
}

// Redirect URI-г ЯГ тулгах нь энэ системийн хамгийн эмзэг шалгалт — сул тулгалт
// нь authorization code-ыг халдагч руу урсгана.
func TestMatchRedirectURIIsExact(t *testing.T) {
	c := testClient()

	if !c.MatchRedirectURI("https://ring.dgov.mn/sso/callback") {
		t.Fatal("the exact registered URI must match")
	}

	attacks := []string{
		"https://ring.dgov.mn/sso/callback/../../evil",
		"https://ring.dgov.mn/sso/callback/evil",      // суффикс
		"https://ring.dgov.mn/sso/callback?next=evil", // query нэмсэн
		"https://ring.dgov.mn/sso/callback#x",         // fragment
		"https://ring.dgov.mn/sso/callbac",            // префикс
		"https://ring.dgov.mn.evil.mn/sso/callback",   // хостын суффикс
		"https://evil.mn/sso/callback",                // өөр хост
		"http://ring.dgov.mn/sso/callback",            // scheme бууруулсан
		"https://ring.dgov.mn:443/sso/callback",       // порт нэмсэн
		"https://RING.dgov.mn/sso/callback",           // том үсэг
		"https://ring.dgov.mn/sso/callback/",          // сүүл зураас
		" https://ring.dgov.mn/sso/callback",          // урд зай
		"",                                            // хоосон
	}
	for _, a := range attacks {
		if c.MatchRedirectURI(a) {
			t.Fatalf("redirect_uri %q must NOT match the registered URI", a)
		}
	}
}

func TestMatchPostLogoutRedirectURIIsExact(t *testing.T) {
	c := testClient()
	if !c.MatchPostLogoutRedirectURI("https://ring.dgov.mn/") {
		t.Fatal("the exact registered post-logout URI must match")
	}
	if c.MatchPostLogoutRedirectURI("https://ring.dgov.mn/evil") {
		t.Fatal("post-logout redirect must not accept a suffix")
	}
}

func TestFilterAllowedScopesCannotEscalate(t *testing.T) {
	c := testClient()

	got := c.FilterAllowedScopes([]string{"openid", "svc:eid-org-proxy", "profile", "admin"})
	want := []string{"openid", "profile"}
	if len(got) != len(want) {
		t.Fatalf("FilterAllowedScopes = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FilterAllowedScopes = %v; want %v (request order preserved)", got, want)
		}
	}
}

func TestFilterAllowedScopesDeduplicates(t *testing.T) {
	c := testClient()
	got := c.FilterAllowedScopes([]string{"openid", "openid", "", "profile"})
	if len(got) != 2 {
		t.Fatalf("FilterAllowedScopes = %v; want the duplicate and the empty entry dropped", got)
	}
}

func TestIsPublicFollowsAuthMethod(t *testing.T) {
	c := testClient()
	if c.IsPublic() {
		t.Fatal("client_secret_basic must not be treated as a public client")
	}
	c.TokenEndpointAuthMethod = AuthMethodNone
	if !c.IsPublic() {
		t.Fatal("token_endpoint_auth_method=none is a public client and requires PKCE")
	}
}

func TestHasGrant(t *testing.T) {
	c := testClient()
	if !c.HasGrant(GrantAuthorizationCode) || !c.HasGrant(GrantRefreshToken) {
		t.Fatal("registered grants must be reported")
	}
	if c.HasGrant(GrantClientCredentials) {
		t.Fatal("an unregistered grant must never be allowed")
	}
}

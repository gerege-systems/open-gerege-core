// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package adminapi

import "testing"

func TestValidateRedirectURI(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		private bool
		ok      bool
	}{
		{"https ok", "https://sso.dgov.mn/sso/callback", false, true},
		{"http loopback ok", "http://127.0.0.1:8080/cb", false, true},
		{"http non-loopback rejected", "http://sso.dgov.mn/cb", false, false},
		{"custom scheme rejected when web", "geregetemp://oauth2/callback", false, false},
		{"custom scheme ok when public (RFC 8252)", "geregetemp://oauth2/callback", true, true},
		{"bare private scheme rejected", "geregetemp:", true, false},
		{"dangerous scheme rejected even when public", "javascript://alert", true, false},
		{"relative rejected", "/sso/callback", true, false},
		{"empty rejected", "", true, false},
		{"fragment rejected", "https://sso.dgov.mn/cb#x", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateRedirectURI(c.raw, c.private)
			if c.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestValidateAdminClient(t *testing.T) {
	// confidential web client — https redirect required
	if err := validateAdminClient(adminClientBody{Name: "app", RedirectURIs: []string{"https://sso.dgov.mn/cb"}}, true); err != nil {
		t.Fatalf("valid confidential client rejected: %v", err)
	}
	// public native client — custom scheme allowed
	if err := validateAdminClient(adminClientBody{Name: "ios", Public: true, RedirectURIs: []string{"geregetemp://oauth2/callback"}}, true); err != nil {
		t.Fatalf("valid public native client rejected: %v", err)
	}
	// create without redirect (authorization_code implied) — rejected
	if err := validateAdminClient(adminClientBody{Name: "x"}, true); err == nil {
		t.Fatal("expected missing-redirect error")
	}
	// unsupported grant — rejected
	if err := validateAdminClient(adminClientBody{Name: "x", RedirectURIs: []string{"https://a/b"}, GrantTypes: []string{"password"}}, true); err == nil {
		t.Fatal("expected unsupported-grant error")
	}
	// bad client_id charset — rejected
	if err := validateAdminClient(adminClientBody{ClientID: "bad id!", Name: "x", RedirectURIs: []string{"https://a/b"}}, true); err == nil {
		t.Fatal("expected client_id charset error")
	}
}

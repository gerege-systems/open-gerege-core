// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Token endpoint (ExchangeFull / Refresh) unit тест: refresh_token + expires_in
// задлалт, refresh grant, Hydra эргүүлээгүй үед хуучин refresh_token хадгалах.
package oidc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTokenServer(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "client-id", "secret", "https://app/cb", "openid offline_access")
}

func TestExchangeFull(t *testing.T) {
	c := newTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %s", r.FormValue("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"acc","id_token":"idt","refresh_token":"ref","expires_in":3600}`))
	})
	tok, err := c.ExchangeFull(context.Background(), "code-xyz")
	if err != nil {
		t.Fatalf("ExchangeFull: %v", err)
	}
	if tok.AccessToken != "acc" || tok.IDToken != "idt" || tok.RefreshToken != "ref" || tok.ExpiresIn != 3600 {
		t.Errorf("tokens = %+v", tok)
	}
}

func TestRefresh(t *testing.T) {
	c := newTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" || r.FormValue("refresh_token") != "old-ref" {
			t.Errorf("form = %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-acc","refresh_token":"new-ref","expires_in":1800}`))
	})
	tok, err := c.Refresh(context.Background(), "old-ref")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tok.AccessToken != "new-acc" || tok.RefreshToken != "new-ref" || tok.ExpiresIn != 1800 {
		t.Errorf("tokens = %+v", tok)
	}
}

func TestRefreshKeepsOldTokenWhenNotRotated(t *testing.T) {
	c := newTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// refresh_token буцаагаагүй — хуучныг хадгална.
		_, _ = w.Write([]byte(`{"access_token":"new-acc","expires_in":1800}`))
	})
	tok, err := c.Refresh(context.Background(), "keep-me")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tok.RefreshToken != "keep-me" {
		t.Errorf("refresh token = %q, want keep-me", tok.RefreshToken)
	}
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ssoeidauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gerege-systems/open-gerege-core/pkg/eid"
)

// newStub нь token + proxy endpoint-уудыг нэг серверт үйлчлэх client үүсгэнэ.
func newStub(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{
		BaseURL:      srv.URL + "/rp/eid-auth",
		TokenURL:     srv.URL + "/oauth2/token",
		ClientID:     "platform",
		ClientSecret: "s3cret",
	}), srv
}

func writeToken(w http.ResponseWriter, expiresIn int) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "app-token", "expires_in": expiresIn,
	})
}

func TestConfiguredRequiresEveryField(t *testing.T) {
	full := Config{BaseURL: "https://sso/rp/eid-auth", TokenURL: "https://sso/oauth2/token", ClientID: "a", ClientSecret: "b"}
	if !full.Configured() {
		t.Fatal("бүтэн тохиргоо Configured() үнэн байх ёстой")
	}
	for name, cfg := range map[string]Config{
		"base алга":   {TokenURL: full.TokenURL, ClientID: "a", ClientSecret: "b"},
		"token алга":  {BaseURL: full.BaseURL, ClientID: "a", ClientSecret: "b"},
		"client алга": {BaseURL: full.BaseURL, TokenURL: full.TokenURL, ClientSecret: "b"},
		"secret алга": {BaseURL: full.BaseURL, TokenURL: full.TokenURL, ClientID: "a"},
	} {
		if cfg.Configured() {
			t.Errorf("%s: Configured() худал байх ёстой", name)
		}
	}
}

func TestInitiateSendsAppTokenAndParsesStart(t *testing.T) {
	var gotAuth, gotBody string
	c, _ := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			// Нууц нь body-д биш, Basic-аар очих ёстой.
			id, secret, ok := r.BasicAuth()
			if !ok || id != "platform" || secret != "s3cret" {
				t.Errorf("client creds Basic-аар ирсэнгүй: ok=%v id=%q", ok, id)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("form: %v", err)
			}
			if got := r.PostForm.Get("grant_type"); got != "client_credentials" {
				t.Errorf("grant_type=%q", got)
			}
			if got := r.PostForm.Get("scope"); got != "svc:eid-auth" {
				t.Errorf("scope=%q", got)
			}
			writeToken(w, 300)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"session_id": "sess-1", "verification_code": "4821", "expires_at": "2026-08-05T00:00:00Z",
		}})
	})

	res, err := c.Initiate(context.Background(), "УБ99887766", "display", "")
	if err != nil {
		t.Fatalf("Initiate: %v", err)
	}
	if gotAuth != "Bearer app-token" {
		t.Errorf("Authorization=%q", gotAuth)
	}
	if !strings.Contains(gotBody, "УБ99887766") {
		t.Errorf("national_id дамжсангүй: %s", gotBody)
	}
	if res.SessionID != "sess-1" || res.VerificationCode != "4821" {
		t.Errorf("start буруу задарлаа: %+v", res)
	}
}

func TestSessionParsesIdentityOnComplete(t *testing.T) {
	c, _ := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			writeToken(w, 300)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"state": eid.StateComplete,
			"identity": map[string]any{
				"civil_id": "ab1234567", "given_name": "Бат", "surname": "Дорж", "kyc_level": "HIGH",
			},
		}})
	})

	res, err := c.Session(context.Background(), "sess-1", 25000)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if res.State != eid.StateComplete || res.Identity == nil {
		t.Fatalf("identity ирсэнгүй: %+v", res)
	}
	if res.Identity.CivilID != "ab1234567" || res.Identity.GivenName != "Бат" {
		t.Errorf("identity буруу: %+v", res.Identity)
	}
}

func TestTokenIsCachedAndRefreshedOn401(t *testing.T) {
	var tokenCalls, proxyCalls int32
	c, _ := newStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			atomic.AddInt32(&tokenCalls, 1)
			writeToken(w, 300)
			return
		}
		// Эхний прокси дуудалт 401 — client токеноо шинэчилж ДАХИН оролдоно.
		if atomic.AddInt32(&proxyCalls, 1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"state": "RUNNING"}})
	})

	if _, err := c.Session(context.Background(), "sess-1", 0); err != nil {
		t.Fatalf("Session: %v", err)
	}
	if tokenCalls != 2 {
		t.Errorf("401-ийн дараа токен шинэчлэх ёстой: tokenCalls=%d", tokenCalls)
	}
	if proxyCalls != 2 {
		t.Errorf("дахин оролдох ёстой: proxyCalls=%d", proxyCalls)
	}

	// Хоёр дахь дуудалт кэшлэгдсэн токеныг хэрэглэнэ — token endpoint дахин
	// дуудагдахгүй.
	if _, err := c.Session(context.Background(), "sess-2", 0); err != nil {
		t.Fatalf("Session 2: %v", err)
	}
	if tokenCalls != 2 {
		t.Errorf("токен кэшлэгдээгүй: tokenCalls=%d", tokenCalls)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusForbidden, ErrNotGranted},
		{http.StatusServiceUnavailable, ErrProxyDisabled},
		{http.StatusBadRequest, eid.ErrInitiateRejected},
	}
	for _, tc := range cases {
		c, _ := newStub(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/oauth2/token" {
				writeToken(w, 300)
				return
			}
			w.WriteHeader(tc.status)
		})
		_, err := c.QRInitiate(context.Background(), "d", "", "nonce")
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d: алдаа=%v, хүлээсэн %v", tc.status, err, tc.want)
		}
	}
}

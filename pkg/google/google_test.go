// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Google OAuth client-ийн unit тест: consent URL-ийн бүтэц, id_token payload-оос
// иргэний мэдээлэл задлах, буруу id_token-д алдаа, Configured шалгалт.
package google

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestAuthCodeURL(t *testing.T) {
	c := NewClient("cid-123", "secret")
	u := c.AuthCodeURL("state-xyz", "https://app.mn/api/auth/google/callback")

	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if q.Get("client_id") != "cid-123" {
		t.Errorf("client_id = %s", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://app.mn/api/auth/google/callback" {
		t.Errorf("redirect_uri = %s", q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" || q.Get("state") != "state-xyz" {
		t.Errorf("response_type/state wrong: %v", q)
	}
	if !strings.Contains(q.Get("scope"), "email") || !strings.Contains(q.Get("scope"), "openid") {
		t.Errorf("scope = %s", q.Get("scope"))
	}
}

// makeIDToken нь тестийн (гарын үсэггүй) id_token үүсгэнэ — parseIDToken зөвхөн
// payload-ыг уншдаг тул header/signature нь чухал биш.
func makeIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, _ := json.Marshal(claims)
	enc := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc(payload) + ".sig"
}

func TestParseIDToken(t *testing.T) {
	tok := makeIDToken(t, map[string]any{
		"sub": "google-sub-1", "email": "bat@gmail.com", "email_verified": true, "name": "Бат",
	})
	u, err := parseIDToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if u.Sub != "google-sub-1" || u.Email != "bat@gmail.com" || !u.EmailVerified || u.Name != "Бат" {
		t.Errorf("parsed = %+v", u)
	}
}

func TestParseIDTokenErrors(t *testing.T) {
	if _, err := parseIDToken("not.a.jwt.token"); err == nil {
		t.Error("4 хэсэгтэй → алдаа")
	}
	if _, err := parseIDToken("only.two"); err == nil {
		t.Error("2 хэсэгтэй → алдаа")
	}
	// sub байхгүй payload → алдаа.
	noSub := makeIDToken(t, map[string]any{"email": "x@y.z"})
	if _, err := parseIDToken(noSub); err == nil {
		t.Error("sub-гүй → алдаа")
	}
}

func TestConfigured(t *testing.T) {
	if NewClient("", "").Configured() {
		t.Error("хоосон → тохируулаагүй")
	}
	if NewClient("id", "").Configured() {
		t.Error("secret-гүй → тохируулаагүй")
	}
	if !NewClient("id", "secret").Configured() {
		t.Error("хоёулаа → тохируулсан")
	}
}

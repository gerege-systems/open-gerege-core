// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package sign

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureSignBody нь /v3 sign notification-ийн body-г барьж авах fake сервер.
func captureSignBody(t *testing.T, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		*got = body
		_ = json.NewEncoder(w).Encode(map[string]any{"sessionID": "s", "vc": map[string]any{"value": "1234"}})
	}))
}

// TestInit_SendsFileName нь баримтын нэр /v3 body-д fileName болж дамжиж буйг
// батална — үүнгүй бол нийтийн verify хуудсанд FILE NAME хоосон харагдана.
func TestInit_SendsFileName(t *testing.T) {
	var body map[string]any
	srv := captureSignBody(t, &body)
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	if _, err := u.Init(context.Background(), "УБ12345678", "Бат Болд", "Гэрээ-2026-07.pdf", []byte("%PDF"), "", "", ""); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got, _ := body["fileName"].(string); got != "Гэрээ-2026-07.pdf" {
		t.Fatalf("fileName = %q, want %q", got, "Гэрээ-2026-07.pdf")
	}
}

// TestInitDigest_NoFileName нь баримтгүй (зөвхөн digest) урсгалд fileName талбар
// ОГТ илгээгдэхгүйг батална — хоосон утга илгээвэл сервер таамаглахаа болино.
func TestInitDigest_NoFileName(t *testing.T) {
	var body map[string]any
	srv := captureSignBody(t, &body)
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	digest := strings.Repeat("ab", 32)
	if _, err := u.InitDigest(context.Background(), "УБ12345678", "Бат Болд", digest, "50000 MNT → …1234"); err != nil {
		t.Fatalf("InitDigest: %v", err)
	}
	if _, ok := body["fileName"]; ok {
		t.Fatal("digest урсгалд body-д fileName байх ёсгүй")
	}
}

func TestClampFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Гэрээ.pdf", "Гэрээ.pdf"},
		{`C:\Users\bat\Гэрээ.pdf`, "Гэрээ.pdf"},
		{"/tmp/upload/doc.pdf", "doc.pdf"},
		{"  doc.pdf  ", "doc.pdf"},
		{"do\x00c\nname.pdf", "docname.pdf"},
		{"", ""},
		{"   ", ""},
		{strings.Repeat("я", maxFileNameRunes+40) + ".pdf", strings.Repeat("я", maxFileNameRunes)},
	}
	for _, c := range cases {
		if got := clampFileName(c.in); got != c.want {
			t.Errorf("clampFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

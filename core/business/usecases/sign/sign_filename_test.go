// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package sign

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// captureSignBody нь /v3 sign notification-ийн body-г барьж авах fake сервер
// болон барьсан body-г уншигч функцийг буцаана.
func captureSignBody(t *testing.T) (srv *httptest.Server, capturedBody func() map[string]any) {
	t.Helper()
	var (
		mu   sync.Mutex
		body map[string]any
	)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		mu.Lock()
		body = got
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"sessionID": "s", "vc": map[string]any{"value": "1234"}})
	}))
	return srv, func() map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return body
	}
}

// TestInit_SendsFileName нь баримтын нэр /v3 body-д fileName болж дамжиж буйг
// батална — үүнгүй бол нийтийн verify хуудсанд FILE NAME хоосон харагдана.
func TestInit_SendsFileName(t *testing.T) {
	srv, body := captureSignBody(t)
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	if _, err := u.Init(context.Background(), "УБ12345678", "Бат Болд", "Гэрээ-2026-07.pdf", []byte("%PDF"), "", "", ""); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got, _ := body()["fileName"].(string); got != "Гэрээ-2026-07.pdf" {
		t.Fatalf("fileName = %q, want %q", got, "Гэрээ-2026-07.pdf")
	}
}

// TestInitDigest_NoFileName — RP нэр өгөөгүй бол fileName талбар ОГТ
// илгээгдэхгүй. Хоосон мөр илгээвэл сервер interaction текстээс таамаглахаа
// болино, тиймээс "байхгүй" ба "хоосон" хоёрыг ялгах нь чухал.
func TestInitDigest_NoFileName(t *testing.T) {
	srv, body := captureSignBody(t)
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	digest := strings.Repeat("ab", 32)
	if _, err := u.InitDigest(context.Background(), "УБ12345678", "Бат Болд", digest, "50000 MNT → …1234", ""); err != nil {
		t.Fatalf("InitDigest: %v", err)
	}
	if _, ok := body()["fileName"]; ok {
		t.Fatal("нэр өгөөгүй үед body-д fileName байх ёсгүй")
	}
}

// TestInitDigest_WithDocumentName — RP нэр өгвөл ТЭР ЧИГЭЭР нь дамжина.
//
// Өмнө нь digest урсгал ямагт хоосон илгээдэг байсан тул нийтийн verify хуудас
// «—» харуулдаг байв: RP-ийн ерөнхий interaction текст («Gerege — баримтад
// гарын үсэг») нь серверийн нөөц аргын хайдаг «зурах: » тэмдэглэгээг
// агуулдаггүй.
func TestInitDigest_WithDocumentName(t *testing.T) {
	srv, body := captureSignBody(t)
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	digest := strings.Repeat("ab", 32)
	if _, err := u.InitDigest(context.Background(), "УБ12345678", "Бат Болд", digest,
		"50000 MNT → …1234", "Шилжүүлэг_2026-07-30"); err != nil {
		t.Fatalf("InitDigest: %v", err)
	}
	if got, _ := body()["fileName"].(string); got != "Шилжүүлэг_2026-07-30" {
		t.Fatalf("fileName = %q, want %q", got, "Шилжүүлэг_2026-07-30")
	}
}

// TestInitDigest_DocumentNameClamped — нэр нь PDF урсгалтай ижил цэвэрлэгээнд
// орно (зам таслах, урт хязгаарлах).
func TestInitDigest_DocumentNameClamped(t *testing.T) {
	srv, body := captureSignBody(t)
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	digest := strings.Repeat("ab", 32)
	if _, err := u.InitDigest(context.Background(), "УБ12345678", "Бат Болд", digest,
		"текст", "/tmp/upload/Гэрээ.pdf"); err != nil {
		t.Fatalf("InitDigest: %v", err)
	}
	if got, _ := body()["fileName"].(string); got != "Гэрээ.pdf" {
		t.Fatalf("fileName = %q, want %q", got, "Гэрээ.pdf")
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

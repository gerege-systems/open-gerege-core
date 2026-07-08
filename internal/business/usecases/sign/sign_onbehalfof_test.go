// Gerege Template Version 27.0
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

	"template/internal/apperror"
)

// memCache — sign usecase-ийн cache интерфэйсийн санах ойн хэрэгжилт (тест).
type memCache struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemCache() *memCache { return &memCache{m: map[string]string{}} }

func (c *memCache) Set(_ context.Context, k string, v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v.(string)
	return nil
}

func (c *memCache) Get(_ context.Context, k string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	if !ok {
		return "", apperror.NotFound("not found")
	}
	return v, nil
}

func newTestUsecase(t *testing.T, baseURL string) *usecase {
	t.Helper()
	id, err := newSelfSignedSigner()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return &usecase{
		cache:  newMemCache(),
		cfg:    Config{V3BaseURL: baseURL, RPUUID: "rp-uuid", RPName: "gerege.mn", APISecret: "rp_sk_test"},
		client: http.DefaultClient,
		signer: id,
	}
}

// TestInit_OnBehalfOf нь байгууллагын нэрийн өмнөөс зурах үед /v3 body-д onBehalfOf
// талбар яг дамжиж буйг, дараа нь poll-оос байгууллагын нэр state-д хадгалагдаж буйг батална.
func TestInit_OnBehalfOf(t *testing.T) {
	const wantOrg = "NTRMN-1234567"
	var gotOnBehalfOf string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v3/signature/notification/etsi/"):
			var body map[string]any
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			if v, ok := body["onBehalfOf"].(string); ok {
				gotOnBehalfOf = v
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessionID": "v3-sess-1",
				"vc":        map[string]any{"value": "1234"},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v3/session/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"state":  "COMPLETE",
				"result": map[string]any{"endResult": "OK"},
				"onBehalfOf": map[string]any{
					"orgEtsi": wantOrg,
					"orgName": "Гэрэгэ Системс ХХК",
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	ctx := context.Background()

	res, err := u.Init(ctx, "УБ12345678", "Бат Болд", "doc.pdf", []byte("%PDF-1.4 test"), wantOrg, "", "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if gotOnBehalfOf != wantOrg {
		t.Fatalf("onBehalfOf body = %q, want %q", gotOnBehalfOf, wantOrg)
	}

	state, err := u.Poll(ctx, "УБ12345678", res.SessionID)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if state != "completed" {
		t.Fatalf("state = %q, want completed", state)
	}
	st, err := u.loadState(ctx, res.SessionID)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if st.OnBehalfOfOrg != wantOrg {
		t.Fatalf("state OnBehalfOfOrg = %q, want %q", st.OnBehalfOfOrg, wantOrg)
	}
	if st.OnBehalfOfOrgName != "Гэрэгэ Системс ХХК" {
		t.Fatalf("state OnBehalfOfOrgName = %q, want authoritative poll name", st.OnBehalfOfOrgName)
	}
}

// TestInit_PersonalSigning нь onBehalfOf хоосон үед body-д уг талбар БАЙХГҮЙг батална.
func TestInit_PersonalSigning(t *testing.T) {
	hasOnBehalfOf := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, hasOnBehalfOf = body["onBehalfOf"]
		_ = json.NewEncoder(w).Encode(map[string]any{"sessionID": "s", "vc": map[string]any{"value": "9999"}})
	}))
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	if _, err := u.Init(context.Background(), "УБ12345678", "Бат Болд", "d.pdf", []byte("%PDF"), "", "", ""); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if hasOnBehalfOf {
		t.Fatal("хувь хүний гарын үсэгт body-д onBehalfOf байх ёсгүй")
	}
}

// TestInit_NotRepresentative_403 нь /v3 403 буцаахад Forbidden (5xx биш) болж буйг батална.
func TestInit_NotRepresentative_403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"not represented"}`))
	}))
	defer srv.Close()

	u := newTestUsecase(t, srv.URL)
	_, err := u.Init(context.Background(), "УБ12345678", "Бат Болд", "d.pdf", []byte("%PDF"), "NTRMN-7654321", "", "")
	if err == nil {
		t.Fatal("алдаа хүлээв (403)")
	}
	de, ok := err.(*apperror.DomainError)
	if !ok {
		t.Fatalf("DomainError хүлээв, авсан: %T (%v)", err, err)
	}
	if de.Type != apperror.ErrTypeForbidden {
		t.Fatalf("Forbidden хүлээв, авсан: %v", de.Type)
	}
}

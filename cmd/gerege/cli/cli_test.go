// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package cli

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testEnv(t *testing.T, handler http.HandlerFunc) Env {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return Env{
		BaseURL:      srv.URL,
		Token:        "test-token",
		HTTP:         &http.Client{Timeout: 5 * time.Second},
		ScaffoldRoot: t.TempDir(),
	}
}

func TestListModules(t *testing.T) {
	var gotPath, gotAuth string
	env := testEnv(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": true,
			"data": []map[string]any{
				{"id": "gov", "kind": "business", "enabled": true, "name": "Гов"},
				{"id": "gspace", "kind": "business", "enabled": false},
			},
		})
	})
	var out bytes.Buffer
	if err := RunWithEnv([]string{"modules", "list"}, &out, env); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/platform/admin/modules" {
		t.Fatalf("path: %s", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("auth: %s", gotAuth)
	}
	if !strings.Contains(out.String(), "gov") || !strings.Contains(out.String(), "унтраалттай") {
		t.Fatalf("гаралт дутуу:\n%s", out.String())
	}

	// Token-гүй үед нийтийн endpoint рүү очно.
	env.Token = ""
	if err := RunWithEnv([]string{"modules", "list"}, &out, env); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/platform/modules" {
		t.Fatalf("нийтийн path: %s", gotPath)
	}
}

func TestToggleModule(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	env := testEnv(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(map[string]any{"status": true, "message": "Модуль унтарлаа"})
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, body.String()
		_, _ = w.Write(b)
	})
	var out bytes.Buffer
	if err := RunWithEnv([]string{"modules", "disable", "gspace"}, &out, env); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut || gotPath != "/api/v1/platform/admin/modules/gspace" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"enabled":false`) {
		t.Fatalf("body: %s", gotBody)
	}

	// Token-гүйгээр toggle хориотой.
	env.Token = ""
	if err := RunWithEnv([]string{"modules", "enable", "gspace"}, &out, env); err == nil {
		t.Fatal("token-гүй toggle алдаа өгөх ёстой")
	}

	// Backend-ийн алдаа ил гарна.
	envErr := testEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": false, "message": "core модуль"})
	})
	if err := RunWithEnv([]string{"modules", "disable", "auth"}, &out, envErr); err == nil ||
		!strings.Contains(err.Error(), "core модуль") {
		t.Fatalf("backend алдаа дамжаагүй: %v", err)
	}
}

func TestScaffoldModule(t *testing.T) {
	env := Env{ScaffoldRoot: t.TempDir()}
	var out bytes.Buffer
	if err := RunWithEnv([]string{"modules", "new", "ring-pay"}, &out, env); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(env.ScaffoldRoot, "modules", "ringpay", "module.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Үүсгэсэн файл нь хүчинтэй Go эх код байх ёстой.
	if _, err := parser.ParseFile(token.NewFileSet(), path, src, 0); err != nil {
		t.Fatalf("scaffold нь хүчинтэй Go биш: %v", err)
	}
	for _, want := range []string{`package ringpay`, `return "ring-pay"`, "module.Host"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("scaffold-д %q алга", want)
		}
	}
	// Давхар үүсгэхийг хориглоно.
	if err := RunWithEnv([]string{"modules", "new", "ring-pay"}, &out, env); err == nil {
		t.Fatal("байгаа модулийг дарж бичих ёсгүй")
	}
	// Буруу ID.
	if err := RunWithEnv([]string{"modules", "new", "Ring_Pay"}, &out, env); err == nil {
		t.Fatal("буруу ID алдаа өгөх ёстой")
	}
}

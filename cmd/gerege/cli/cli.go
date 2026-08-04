// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package cli — gerege CLI-ийн тестлэгдэх цөм (командын задлан, API client,
// модулийн scaffold). main.go нь нимгэн бүрхүүл.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Env — гадаад орчны хамаарлууд (тестэд солигдоно).
type Env struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	// ScaffoldRoot — `modules new`-ийн бичих сан (default: ажлын сан).
	ScaffoldRoot string
}

func envFromOS() Env {
	base := os.Getenv("GEREGE_API")
	if base == "" {
		base = "http://localhost:8080"
	}
	return Env{
		BaseURL:      strings.TrimRight(base, "/"),
		Token:        os.Getenv("GEREGE_ADMIN_TOKEN"),
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		ScaffoldRoot: ".",
	}
}

// Run нь argv-г задлаад гүйцэтгэнэ (os.Args[1:]).
func Run(args []string, out io.Writer) error {
	return RunWithEnv(args, out, envFromOS())
}

// RunWithEnv — тестэд орчноо тарьж өгөх хувилбар.
func RunWithEnv(args []string, out io.Writer, env Env) error {
	if len(args) < 1 || args[0] != "modules" {
		return fmt.Errorf("хэрэглээ: gerege modules <list|enable|disable|new> …")
	}
	if len(args) < 2 {
		return fmt.Errorf("хэрэглээ: gerege modules <list|enable|disable|new> …")
	}
	switch args[1] {
	case "list":
		return listModules(out, env)
	case "enable", "disable":
		if len(args) < 3 {
			return fmt.Errorf("хэрэглээ: gerege modules %s <id>", args[1])
		}
		return toggleModule(out, env, args[2], args[1] == "enable")
	case "new":
		if len(args) < 3 {
			return fmt.Errorf("хэрэглээ: gerege modules new <id>")
		}
		return scaffoldModule(out, env, args[2])
	default:
		return fmt.Errorf("үл мэдэгдэх команд: modules %s", args[1])
	}
}

type moduleStatus struct {
	ID        string   `json:"id"`
	Name      string   `json:"name,omitempty"`
	Kind      string   `json:"kind"`
	Enabled   bool     `json:"enabled"`
	DependsOn []string `json:"depends_on,omitempty"`
}

type apiResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (e Env) do(method, path string, body any) (*apiResponse, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.BaseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if e.Token != "" {
		req.Header.Set("Authorization", "Bearer "+e.Token)
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out apiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("HTTP %d: хариу задлагдсангүй: %w", resp.StatusCode, err)
	}
	if resp.StatusCode >= 400 || !out.Status {
		return &out, fmt.Errorf("HTTP %d: %s", resp.StatusCode, out.Message)
	}
	return &out, nil
}

func listModules(out io.Writer, env Env) error {
	path := "/api/v1/platform/modules"
	if env.Token != "" {
		path = "/api/v1/platform/admin/modules"
	}
	resp, err := env.do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	var mods []moduleStatus
	if err := json.Unmarshal(resp.Data, &mods); err != nil {
		return err
	}
	for _, m := range mods {
		state := "ИДЭВХТЭЙ"
		if !m.Enabled {
			state = "унтраалттай"
		}
		line := fmt.Sprintf("%-16s %-8s %-12s", m.ID, m.Kind, state)
		if m.Name != "" {
			line += " " + m.Name
		}
		if len(m.DependsOn) > 0 {
			line += fmt.Sprintf("  (хамаарал: %s)", strings.Join(m.DependsOn, ", "))
		}
		fmt.Fprintln(out, line)
	}
	return nil
}

func toggleModule(out io.Writer, env Env, id string, enabled bool) error {
	if env.Token == "" {
		return fmt.Errorf("GEREGE_ADMIN_TOKEN шаардлагатай (админы access token)")
	}
	resp, err := env.do(http.MethodPut, "/api/v1/platform/admin/modules/"+id,
		map[string]bool{"enabled": enabled})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, resp.Message)
	return nil
}

// scaffoldModule нь modules/<id>/module.go skeleton үүсгэж, дараагийн
// алхмуудын чеклист хэвлэнэ. Байгаа файлыг ДАРЖ БИЧИХГҮЙ.
func scaffoldModule(out io.Writer, env Env, id string) error {
	if err := validateModuleID(id); err != nil {
		return err
	}
	pkg := packageName(id)
	dir := filepath.Join(env.ScaffoldRoot, "modules", pkg)
	path := filepath.Join(dir, "module.go")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s аль хэдийн байна", path)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(moduleTemplate(id, pkg)), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "Үүсгэлээ: %s\n\nДараагийн алхмууд:\n", path)
	fmt.Fprintf(out, "  1. kernel/module/builtin.go-д манифест нэм (ID: %q, RoutePrefixes: \"/api/v1/%s/\")\n", id, id)
	fmt.Fprintf(out, "  2. cmd/api/server/server.go platformModules()-д %s.New()-г нэм\n", pkg)
	fmt.Fprintln(out, "  3. core/business/usecases + route + migration-аа бич (docs/MODULES.md §Шинэ модуль)")
	fmt.Fprintln(out, "  4. go test ./core/http/routes -run TestGoldenRoutes -update")
	return nil
}

func validateModuleID(id string) error {
	if id == "" || strings.ToLower(id) != id || strings.ContainsAny(id, " /\\_") {
		return fmt.Errorf("модулийн ID нь жижиг үсэг + цэгтэй зураас байх ёстой (жишээ: ring, core-find)")
	}
	return nil
}

// packageName — "core-find" → "corefind".
func packageName(id string) string {
	return strings.ReplaceAll(id, "-", "")
}

func moduleTemplate(id, pkg string) string {
	return fmt.Sprintf(`// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package %[2]s нь %[1]q business модулийн wiring. Домэйн кодоо
// core/business/usecases/%[2]s дор бичээд энд угсарна — docs/MODULES.md-г үз.
package %[2]s

import (
	"context"

	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — %[1]s модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — kernel/module/builtin.go дахь манифестийн ID.
func (*Module) ID() string { return %[1]q }

// Register нь repo → usecase → route wiring-ээ хийнэ.
func (m *Module) Register(_ context.Context, host module.Host) error {
	// Жишээ:
	//   uc := %[2]s.NewUsecase(%[2]spostgres.NewRepository(host.Pool()))
	//   routes.New%[3]sRoute(host.APIRouter(), uc, host.AuthMiddleware()).Routes()
	_ = host
	return nil
}
`, id, pkg, strings.Title(pkg)) //nolint:staticcheck // ASCII ID тул Title аюулгүй
}

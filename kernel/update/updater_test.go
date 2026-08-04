// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package update

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseAndCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "1.2.3", 0},
		{"v1.2.3", "v1.2.4", -1},
		{"v1.3.0", "v1.2.9", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.2.0-beta.1", "v1.2.0", -1},
		{"v1.2.0-beta.1", "v1.2.0-beta.2", -1},
		{"v1.2.0-rc.1", "v1.2.0-beta.9", 1},
	}
	for _, tc := range cases {
		a, err := ParseVersion(tc.a)
		if err != nil {
			t.Fatalf("%s: %v", tc.a, err)
		}
		b, err := ParseVersion(tc.b)
		if err != nil {
			t.Fatalf("%s: %v", tc.b, err)
		}
		if got := Compare(a, b); got != tc.want {
			t.Errorf("Compare(%s,%s)=%d, хүлээсэн %d", tc.a, tc.b, got, tc.want)
		}
	}
	for _, bad := range []string{"", "v1.2", "1.2.x", "v-1.0.0"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("%q: алдаа гарах ёстой", bad)
		}
	}
}

// newTestUpdater — манифест сервер + version файлтай updater.
func newTestUpdater(t *testing.T, current, manifestJSON string, cfg Config) (*Updater, *string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, manifestJSON)
	}))
	t.Cleanup(srv.Close)

	vf := filepath.Join(t.TempDir(), "VERSION")
	if current != "" {
		if err := os.WriteFile(vf, []byte(current), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg.ManifestURL = srv.URL
	cfg.VersionFile = vf
	if cfg.Channel == "" {
		cfg.Channel = "stable"
	}
	u := New(cfg)
	var ran string
	u.runner = func(_ context.Context, argv []string, env []string) error {
		ran = fmt.Sprintf("%v %v", argv, env)
		return nil
	}
	return u, &ran
}

func TestCheckOnceDecisions(t *testing.T) {
	ctx := context.Background()

	// Шинэ patch → apply.
	u, _ := newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.4"}}`, Config{})
	d, err := u.CheckOnce(ctx)
	if err != nil || d.Action != ActionApply {
		t.Fatalf("patch: action=%s err=%v", d.Action, err)
	}

	// Ижил хувилбар → none.
	u, _ = newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.3"}}`, Config{})
	if d, _ = u.CheckOnce(ctx); d.Action != ActionNone {
		t.Fatalf("same: action=%s", d.Action)
	}

	// Major өсөлт, зөвшөөрөлгүй → blocked-major.
	u, _ = newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v2.0.0"}}`, Config{})
	if d, _ = u.CheckOnce(ctx); d.Action != ActionBlockedMajor {
		t.Fatalf("major: action=%s", d.Action)
	}

	// Major өсөлт, зөвшөөрсөн → apply.
	u, _ = newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v2.0.0"}}`, Config{AllowMajor: true})
	if d, _ = u.CheckOnce(ctx); d.Action != ActionApply {
		t.Fatalf("major allowed: action=%s", d.Action)
	}

	// Цонхны гадна → blocked-window.
	u, _ = newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.4"}}`, Config{Window: "03:00-04:00"})
	u.now = func() time.Time { return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC) }
	if d, _ = u.CheckOnce(ctx); d.Action != ActionBlockedWindow {
		t.Fatalf("window: action=%s", d.Action)
	}
	// Цонхны дотор (шөнө дамнасан) → apply.
	u, _ = newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.4"}}`, Config{Window: "23:00-02:00"})
	u.now = func() time.Time { return time.Date(2026, 8, 4, 1, 0, 0, 0, time.UTC) }
	if d, _ = u.CheckOnce(ctx); d.Action != ActionApply {
		t.Fatalf("overnight window: action=%s", d.Action)
	}

	// Суваг алга → алдаа.
	u, _ = newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.4"}}`, Config{Channel: "beta"})
	if _, err = u.CheckOnce(ctx); err == nil {
		t.Fatal("байхгүй суваг алдаа өгөх ёстой")
	}
}

func TestApplySuccessWritesVersionFile(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer health.Close()

	u, ran := newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.4"}}`, Config{
		ApplyCmd:  []string{"update.sh"},
		HealthURL: health.URL, HealthTimeout: time.Second, HealthInterval: 10 * time.Millisecond,
	})
	d, _ := u.CheckOnce(context.Background())
	if err := u.Apply(context.Background(), d); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if *ran == "" || !strings.Contains(*ran, "GEREGE_TARGET_VERSION=v1.6.4") {
		t.Fatalf("apply script env дутуу: %s", *ran)
	}
	b, _ := os.ReadFile(u.cfg.VersionFile)
	if string(b) != "v1.6.4\n" {
		t.Fatalf("version файл: %q", b)
	}
}

func TestApplyRollsBackOnFailedHealth(t *testing.T) {
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer health.Close()

	u, _ := newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.4"}}`, Config{
		ApplyCmd: []string{"update.sh"}, RollbackCmd: []string{"rollback.sh"},
		HealthURL: health.URL, HealthTimeout: 100 * time.Millisecond, HealthInterval: 10 * time.Millisecond,
	})
	var calls []string
	u.runner = func(_ context.Context, argv []string, _ []string) error {
		calls = append(calls, argv[0])
		return nil
	}
	d, _ := u.CheckOnce(context.Background())
	if err := u.Apply(context.Background(), d); err == nil {
		t.Fatal("health унасан үед Apply алдаа буцаах ёстой")
	}
	if len(calls) != 2 || calls[0] != "update.sh" || calls[1] != "rollback.sh" {
		t.Fatalf("rollback дуудагдаагүй: %v", calls)
	}
	// Version файл ХЭВЭЭР — шинэчлэлт бүртгэгдээгүй.
	b, _ := os.ReadFile(u.cfg.VersionFile)
	if string(b) != "v1.6.3" {
		t.Fatalf("version файл өөрчлөгдсөн байна: %q", b)
	}
}

func TestApplyRollsBackOnScriptFailure(t *testing.T) {
	u, _ := newTestUpdater(t, "v1.6.3", `{"channels":{"stable":"v1.6.4"}}`, Config{
		ApplyCmd: []string{"update.sh"}, RollbackCmd: []string{"rollback.sh"},
	})
	var calls []string
	u.runner = func(_ context.Context, argv []string, _ []string) error {
		calls = append(calls, argv[0])
		if argv[0] == "update.sh" {
			return errors.New("boom")
		}
		return nil
	}
	d, _ := u.CheckOnce(context.Background())
	if err := u.Apply(context.Background(), d); err == nil {
		t.Fatal("script унасан үед Apply алдаа буцаах ёстой")
	}
	if len(calls) != 2 || calls[1] != "rollback.sh" {
		t.Fatalf("rollback дуудагдаагүй: %v", calls)
	}
}

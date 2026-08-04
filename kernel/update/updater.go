// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Manifest — release зарлалын JSON ({"channels":{"stable":"v1.7.0", ...}}).
// Registry талдаа энэ файлыг release pipeline бичиж байршуулна (GitHub Pages,
// S3, plain nginx — дурын статик хост).
type Manifest struct {
	Channels map[string]string `json:"channels"`
}

// Action — CheckOnce-ийн шийдвэр.
type Action string

const (
	// ActionNone — шинэчлэлт алга (эсвэл одоогийнхтой ижил/хуучин).
	ActionNone Action = "none"
	// ActionApply — шинэчлэлт хийж болно, хийнэ.
	ActionApply Action = "apply"
	// ActionBlockedMajor — major өсөлт, админы гар баталгаажуулалт хэрэгтэй.
	ActionBlockedMajor Action = "blocked-major"
	// ActionBlockedWindow — maintenance цонхны гадна тул хойшлогдов.
	ActionBlockedWindow Action = "blocked-window"
)

// Decision — нэг шалгалтын үр дүн.
type Decision struct {
	Action  Action
	Current Version
	Target  Version
}

// Config — updater-ийн тохиргоо (cmd/platformd env-ээс бөглөнө).
type Config struct {
	ManifestURL string
	Channel     string
	// VersionFile — одоо ажиллаж буй хувилбарыг хадгалах файл (амжилттай
	// apply бүрийн дараа шинэчлэгдэнэ).
	VersionFile string
	AllowMajor  bool
	// Window — "03:00-05:00" (локал цаг) maintenance цонх; хоосон = үргэлж.
	// Шөнө дамнасан цонхыг ("23:00-02:00") дэмжинэ.
	Window string

	ApplyCmd    []string
	RollbackCmd []string

	HealthURL      string
	HealthTimeout  time.Duration
	HealthInterval time.Duration
}

// Updater — шийдвэр + apply төлөвийн машин. runner/now нь тестэд солигдоно.
type Updater struct {
	cfg    Config
	client *http.Client
	// runner нь командыг нэмэлт env-тэй ажиллуулна.
	runner func(ctx context.Context, argv []string, extraEnv []string) error
	now    func() time.Time
	// Logf — явцын лог (default: үгүй).
	Logf func(format string, args ...any)
}

// New нь бодит runner/clock-той Updater үүсгэнэ.
func New(cfg Config) *Updater {
	return &Updater{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		runner: func(ctx context.Context, argv []string, extraEnv []string) error {
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // G204: argv нь операторын тохиргоо (ApplyCmd/RollbackCmd), хүсэлтийн оролт биш; shell-гүй exec
			cmd.Env = append(os.Environ(), extraEnv...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
		now:  time.Now,
		Logf: func(string, ...any) {},
	}
}

// CurrentVersion нь version файлыг уншина; байхгүй бол v0.0.0.
func (u *Updater) CurrentVersion() Version {
	b, err := os.ReadFile(u.cfg.VersionFile)
	if err != nil {
		return Version{}
	}
	v, err := ParseVersion(strings.TrimSpace(string(b)))
	if err != nil {
		return Version{}
	}
	return v
}

// FetchManifest нь release манифестийг татна.
func (u *Updater) FetchManifest(ctx context.Context) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.cfg.ManifestURL, http.NoBody)
	if err != nil {
		return Manifest{}, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("update: манифест татах: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // унших талын Close нь алдаа буцаадаггүй
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("update: манифест: HTTP %d", resp.StatusCode)
	}
	var m Manifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("update: манифест задлах: %w", err)
	}
	return m, nil
}

// CheckOnce нь манифест татаж шийдвэр гаргана (юу ч ажиллуулахгүй).
func (u *Updater) CheckOnce(ctx context.Context) (Decision, error) {
	cur := u.CurrentVersion()
	m, err := u.FetchManifest(ctx)
	if err != nil {
		return Decision{Action: ActionNone, Current: cur}, err
	}
	raw, ok := m.Channels[u.cfg.Channel]
	if !ok {
		return Decision{Action: ActionNone, Current: cur},
			fmt.Errorf("update: манифестэд %q суваг алга", u.cfg.Channel)
	}
	target, err := ParseVersion(raw)
	if err != nil {
		return Decision{Action: ActionNone, Current: cur}, err
	}
	d := Decision{Current: cur, Target: target}
	switch {
	case Compare(target, cur) <= 0:
		d.Action = ActionNone
	case target.Major > cur.Major && !u.cfg.AllowMajor:
		d.Action = ActionBlockedMajor
	case !u.inWindow(u.now()):
		d.Action = ActionBlockedWindow
	default:
		d.Action = ActionApply
	}
	return d, nil
}

// inWindow — maintenance цонхны шалгалт (шөнө дамнасан цонхыг дэмжинэ).
func (u *Updater) inWindow(t time.Time) bool {
	w := strings.TrimSpace(u.cfg.Window)
	if w == "" {
		return true
	}
	var fromH, fromM, toH, toM int
	if _, err := fmt.Sscanf(w, "%d:%d-%d:%d", &fromH, &fromM, &toH, &toM); err != nil {
		// Буруу тохиргоо — аюулгүй тал руу: хэзээ ч апплай хийхгүй.
		return false
	}
	mins := t.Hour()*60 + t.Minute()
	from, to := fromH*60+fromM, toH*60+toM
	if from <= to {
		return mins >= from && mins < to
	}
	return mins >= from || mins < to // шөнө дамнасан
}

// Apply нь шинэчлэлтийг гүйцэтгэнэ: apply script → health хүлээлт →
// амжилтад version файл шинэчилнэ; health унавал rollback script ажиллуулаад
// алдаа буцаана. Script-үүдэд target/current нь env-ээр очно.
func (u *Updater) Apply(ctx context.Context, d Decision) error {
	env := []string{
		"GEREGE_TARGET_VERSION=" + d.Target.String(),
		"GEREGE_CURRENT_VERSION=" + d.Current.String(),
	}
	u.Logf("шинэчлэлт эхэллээ: %s → %s", d.Current, d.Target)
	if err := u.runner(ctx, u.cfg.ApplyCmd, env); err != nil {
		u.Logf("apply script унав: %v — rollback", err)
		u.rollback(ctx, env)
		return fmt.Errorf("update: apply унав: %w", err)
	}
	if err := u.waitHealthy(ctx); err != nil {
		u.Logf("health баталгаажсангүй: %v — rollback", err)
		u.rollback(ctx, env)
		return fmt.Errorf("update: health баталгаажсангүй: %w", err)
	}
	if u.cfg.VersionFile != "" {
		if err := os.WriteFile(u.cfg.VersionFile, []byte(d.Target.String()+"\n"), 0o644); err != nil { //nolint:gosec // G306: version файл нууц биш — deploy script-үүд (өөр хэрэглэгчээр) уншдаг
			return fmt.Errorf("update: version файл бичих: %w", err)
		}
	}
	u.Logf("шинэчлэлт амжилттай: %s", d.Target)
	return nil
}

func (u *Updater) rollback(ctx context.Context, env []string) {
	if len(u.cfg.RollbackCmd) == 0 {
		return
	}
	if err := u.runner(ctx, u.cfg.RollbackCmd, env); err != nil {
		u.Logf("АНХААР: rollback ӨӨРӨӨ унав: %v — гар оролцоо хэрэгтэй", err)
	}
}

// waitHealthy нь HealthURL-ыг interval-аар шалгаж 200 хүртэл хүлээнэ.
func (u *Updater) waitHealthy(ctx context.Context) error {
	if u.cfg.HealthURL == "" {
		return nil
	}
	timeout := u.cfg.HealthTimeout
	if timeout == 0 {
		timeout = 3 * time.Minute
	}
	interval := u.cfg.HealthInterval
	if interval == 0 {
		interval = 5 * time.Second
	}
	deadline := u.now().Add(timeout)
	lastErr := fmt.Errorf("шалгалт хийгдээгүй")
	for u.now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.cfg.HealthURL, http.NoBody)
		resp, err := u.client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return lastErr
}

// Run нь интервалаар CheckOnce + Apply давтана (ctx cancel хүртэл).
func (u *Updater) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		d, err := u.CheckOnce(ctx)
		if err != nil {
			u.Logf("шалгалт: %v", err)
		} else {
			switch d.Action {
			case ActionApply:
				if err := u.Apply(ctx, d); err != nil {
					u.Logf("%v", err)
				}
			case ActionBlockedMajor:
				u.Logf("major шинэчлэлт %s хүлээгдэж байна — админ баталгаажуулна (PLATFORMD_ALLOW_MAJOR=true)", d.Target)
			case ActionBlockedWindow:
				u.Logf("шинэчлэлт %s цонх хүлээж байна", d.Target)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// platformd — платформын өөрийгөө шинэчлэгч supervisor (V4.0 Modular
// Platform, Phase 5). API/web стекийн ХАЖУУД (тусдаа процесс/unit) ажиллаж:
//
//  1. release манифестийг (PLATFORMD_MANIFEST_URL) интервалаар шалгана;
//  2. сувгийнхаа (stable|beta) шинэ хувилбарыг илрүүлбэл maintenance
//     цонхонд apply script-ээ ажиллуулна (git checkout + compose build);
//  3. health URL 200 болтол хүлээнэ — болохгүй бол rollback script-ээ
//     ажиллуулж хуучин хувилбарт буцаана;
//  4. амжилтад version файлаа шинэчилнэ.
//
// Major өсөлтийг default-оор апплай хийхгүй (PLATFORMD_ALLOW_MAJOR=true үед л).
// platformd нь платформыг шинэчилдэг болохоос ӨӨРИЙГӨӨ шинэчилдэггүй —
// supervisor нь update урсгалаас гадуур байх нь rollback-ийн баталгаа.
//
// Тохиргоо (env):
//
//	PLATFORMD_MANIFEST_URL    release манифест JSON ({"channels":{"stable":"v1.7.0"}})
//	PLATFORMD_CHANNEL         stable | beta (default: stable)
//	PLATFORMD_VERSION_FILE    одоогийн хувилбарын файл (default: ./VERSION)
//	PLATFORMD_INTERVAL        шалгалтын интервал (default: 5m)
//	PLATFORMD_WINDOW          maintenance цонх "03:00-05:00" (хоосон: үргэлж)
//	PLATFORMD_ALLOW_MAJOR     true бол major өсөлтийг автоматаар апплай
//	PLATFORMD_APPLY_CMD       apply script (default: ./deploy/platformd/update.sh)
//	PLATFORMD_ROLLBACK_CMD    rollback script (default: ./deploy/platformd/rollback.sh)
//	PLATFORMD_HEALTH_URL      health шалгах URL (жишээ: http://localhost:8080/api/)
//	PLATFORMD_HEALTH_TIMEOUT  health хүлээх дээд хугацаа (default: 3m)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gerege-systems/open-gerege-core/kernel/update"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func durationOr(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Printf("platformd: %s=%q буруу duration — default %s", key, v, def) //nolint:gosec // G706: утга нь операторын өөрийн env, %q-аар мултруулсан
	}
	return def
}

func main() {
	manifestURL := os.Getenv("PLATFORMD_MANIFEST_URL")
	if manifestURL == "" {
		log.Fatal("platformd: PLATFORMD_MANIFEST_URL заавал")
	}

	cfg := update.Config{
		ManifestURL:    manifestURL,
		Channel:        envOr("PLATFORMD_CHANNEL", "stable"),
		VersionFile:    envOr("PLATFORMD_VERSION_FILE", "VERSION"),
		AllowMajor:     os.Getenv("PLATFORMD_ALLOW_MAJOR") == "true",
		Window:         os.Getenv("PLATFORMD_WINDOW"),
		ApplyCmd:       []string{envOr("PLATFORMD_APPLY_CMD", "./deploy/platformd/update.sh")},
		RollbackCmd:    []string{envOr("PLATFORMD_ROLLBACK_CMD", "./deploy/platformd/rollback.sh")},
		HealthURL:      os.Getenv("PLATFORMD_HEALTH_URL"),
		HealthTimeout:  durationOr("PLATFORMD_HEALTH_TIMEOUT", 3*time.Minute),
		HealthInterval: 5 * time.Second,
	}
	u := update.New(cfg)
	u.Logf = log.Printf

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	interval := durationOr("PLATFORMD_INTERVAL", 5*time.Minute)
	log.Printf("platformd эхэллээ: суваг=%s интервал=%s одоогийн=%s",
		cfg.Channel, interval, u.CurrentVersion())
	u.Run(ctx, interval)
	log.Print("platformd зогслоо")
}

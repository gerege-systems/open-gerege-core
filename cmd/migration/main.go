// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/public-gerege-core/core/config"
	"github.com/gerege-systems/public-gerege-core/core/constants"
	"github.com/gerege-systems/public-gerege-core/core/datasources/drivers"
	"github.com/gerege-systems/public-gerege-core/core/datasources/migration"
	coremigrations "github.com/gerege-systems/public-gerege-core/migrations"
	"github.com/gerege-systems/public-gerege-core/pkg/logger"
)

// dbConnectAttempts / dbConnectDelay нь эхний DB холболтыг дахин оролдох
// бодлого. Хоосон volume дээр эхлэхэд compose-ийн db healthcheck нь postgres
// TCP-ээ нээхээс өмнөхөн "healthy" гэж мэдээлж болзошгүй (initdb цонх) тул
// migrate нь бүх deploy-г нэг refused холболтоор унагахгүйн тулд хэсэг хугацаанд
// дахин оролдоно. ~6×2s = ~10s буфер; TCP healthcheck-ийн зэрэгцээ давхар хамгаалалт.
const (
	dbConnectAttempts = 6
	dbConnectDelay    = 2 * time.Second
)

// appMigrationsDir нь АППЫН өөрийн migration хавтас (ажлын директороос
// харьцангуй). Суурийн migration-ууд хоёртын файлд шингээгдсэн тул энэ
// хавтас байхгүй байх нь хэвийн — тухайн үед зөвхөн суурь ажиллана.
const appMigrationsDir = "migrations"

var (
	up   bool
	down bool
)

func init() {
	if err := config.InitializeAppConfig(); err != nil {
		logger.Fatal(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
	}
	logger.Info("configuration loaded", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
}

// migrationSources нь суурийн embed FS-ийг, дараа нь байвал аппын хавтсыг
// өгнө. Runner нь бүгдийг нэг дараалалд, дугаараар эрэмбэлдэг тул
// жагсаалтын дараалал эрэмбэд нөлөөлөхгүй — зөвхөн эх сурвалж нэмнэ.
func migrationSources() []fs.FS {
	srcs := []fs.FS{coremigrations.FS}
	if entries, err := os.ReadDir(appMigrationsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				srcs = append(srcs, os.DirFS(appMigrationsDir))
				break
			}
		}
	}
	return srcs
}

func main() {
	flag.BoolVar(&up, "up", false, "apply new tables, columns, or other structures")
	flag.BoolVar(&down, "down", false, "drop tables, columns, or other structures")
	flag.Parse()

	ctx := context.Background()
	pool, err := connectWithRetry(ctx)
	if err != nil {
		logger.Panic(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryMigration})
	}
	defer pool.Close()

	runner := migration.NewFS(pool, migrationSources()...)

	if up {
		// SQL файлууд (өргөтгөлүүд, partial-unique индексүүд,
		// uuid_generate_v4() id анхдагч утга) бүх schema-г бэлддэг. ORM-гүй
		// тул AutoMigrate байхгүй — schema нь зөвхөн *.up.sql-аас гарна.
		if err := runner.Up(ctx); err != nil {
			logger.Fatal(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryMigration})
		}
	}
	if down {
		if err := runner.Down(ctx); err != nil {
			logger.Fatal(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryMigration})
		}
	}
}

// connectWithRetry нь SetupPgxPostgres-г dbConnectAttempts удаа, хооронд нь
// dbConnectDelay хүлээж дахин оролдоно. Эхний амжилттай холболтыг буцаана;
// бүх оролдлого бүтэлгүйтвэл сүүлийн алдааг агуулсан error буцаана.
func connectWithRetry(ctx context.Context) (*pgxpool.Pool, error) {
	var lastErr error
	for attempt := 1; attempt <= dbConnectAttempts; attempt++ {
		pool, err := drivers.SetupPgxPostgres(ctx)
		if err == nil {
			return pool, nil
		}
		lastErr = err
		logger.Warn(
			fmt.Sprintf("db connection attempt %d/%d failed: %v", attempt, dbConnectAttempts, err),
			logger.Fields{constants.LoggerCategory: constants.LoggerCategoryDatabase},
		)
		if attempt < dbConnectAttempts {
			time.Sleep(dbConnectDelay)
		}
	}
	return nil, fmt.Errorf("database unreachable after %d attempts: %w", dbConnectAttempts, lastErr)
}

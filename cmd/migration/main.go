// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/config"
	"template/internal/constants"
	"template/internal/datasources/drivers"
	"template/internal/datasources/migration"
	"template/pkg/logger"
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

// migrationsDir нь модулийн root-оос харьцангуй (make mig-up нь backend/-ээс
// ажилладаг). SQL файлууд нь конвенцийн дагуу backend/migrations/-д байрлана.
const migrationsDir = "migrations"

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

	runner := migration.New(pool, migrationsDir)

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

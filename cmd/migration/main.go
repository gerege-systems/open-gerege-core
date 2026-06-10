// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package main

import (
	"context"
	"flag"

	"template/internal/config"
	"template/internal/constants"
	"template/internal/datasources/drivers"
	"template/internal/datasources/migration"
	"template/pkg/logger"
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
	pool, err := drivers.SetupPgxPostgres(ctx)
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

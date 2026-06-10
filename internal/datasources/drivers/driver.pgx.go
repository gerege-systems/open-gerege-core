// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package drivers нь config утгуудыг холбогдсон өгөгдлийн сангийн
// драйвер болгон хувиргадаг композицийн давхарга юм. Энэ template нь
// jackc/pgx/v5 (pgxpool) ашигладаг — ORM-гүй, түүхий SQL-ийг
// repository давхаргад гараар бичдэг.
package drivers

import (
	"context"
	"fmt"
	"time"

	"template/internal/config"
	"template/internal/constants"
	"template/pkg/logger"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxConfig нь өгөгдлийн сангийн pool-ийн тохиргоог хадгална.
type PgxConfig struct {
	DataSourceName string
	MaxConns       int32
	MinConns       int32
	MaxLifetime    time.Duration
	MaxIdleTime    time.Duration
}

// InitializePgxPool нь холбогдсон *pgxpool.Pool-г буцаана. Уг pool нь
// otelpgx tracer-ээр OpenTelemetry-ээр хэмжигдсэн — Query/Exec бүр
// semantic-convention атрибутаар (db.system, db.statement) тэмдэглэгдсэн
// span гаргадаг. Tracing идэвхгүй үед global provider нь OTel-ийн no-op
// байх тул энэ нь бараг ямар ч зардалгүй. Pool-ийн статистикийг
// pkg/observability-д бүртгэх замаар /metrics-ээр илчилдэг.
func (cfg *PgxConfig) InitializePgxPool(ctx context.Context) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DataSourceName)
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	// Statement бүрт зориулсан OpenTelemetry tracing.
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxLifetime
	}
	if cfg.MaxIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxIdleTime
	}

	logger.Info(fmt.Sprintf("Setting pgx pool max/min conns to %d/%d", poolCfg.MaxConns, poolCfg.MinConns),
		logger.Fields{constants.LoggerCategory: constants.LoggerCategoryDatabase})

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	return pool, nil
}

// SetupPgxPostgres нь config.AppConfig-оос DB_POSTGRE_* түлхүүрүүдийг
// уншиж, Postgres руу чиглэсэн *pgxpool.Pool-г бүтээж ping хийдэг.
func SetupPgxPostgres(ctx context.Context) (*pgxpool.Pool, error) {
	var dsn string
	switch config.AppConfig.Environment {
	case constants.EnvironmentDevelopment:
		dsn = config.AppConfig.DBPostgreDsn
	case constants.EnvironmentProduction:
		dsn = config.AppConfig.DBPostgreURL
	}

	cfg := PgxConfig{
		DataSourceName: dsn,
		MaxConns:       int32(config.AppConfig.DBMaxOpenConns),
		MinConns:       int32(config.AppConfig.DBMaxIdleConns),
		MaxLifetime:    time.Duration(config.AppConfig.DBConnMaxLifeMins) * time.Minute,
		MaxIdleTime:    5 * time.Minute,
	}
	return cfg.InitializePgxPool(ctx)
}

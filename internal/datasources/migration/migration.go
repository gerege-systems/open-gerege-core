// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package migration нь cmd/migration-ийн ард байрлах туршиж болох
// сан юм. cmd/migration/main.go дахь CLI нь энэ package-ийн нимгэн
// бүрхүүл (config ачаалах + flag задлах + pool холбох) тул idempotency /
// advisory-lock / нэг файлд нэг транзакцийн зан төлөвийг binary
// ажиллуулалгүйгээр integration тестэд шалгаж болно.
package migration

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"template/internal/constants"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdvisoryLockID нь pg_advisory_lock-той хамт ашиглагддаг дурын 64-бит
// бүхэл тоо бөгөөд хоёр migration runner нэг файлыг зэрэг хэрэгжүүлэхээс
// сэргийлдэг.
const AdvisoryLockID = 947328461230

// Runner нь schema_migrations хүснэгтэд хэрэгжсэн төлөвийг хянахын
// зэрэгцээ Postgres DB-д SQL migration файлуудыг хэрэгжүүлэх/буцаах
// үйлдлийг гүйцэтгэнэ. Бүх ажил pool-аас авсан НЭГ dedicated холболт
// дээр ажилладаг тул session-scoped advisory lock зөв ажиллана.
type Runner struct {
	pool *pgxpool.Pool
	dir  string
	// log нь тестүүдэд no-op sink сольж тавих боломж олгоно; nil бол
	// төслийн нийтлэг logger руу буцна.
	log func(msg string, fields logger.Fields)
}

// New нь `dir`-ээс migration файлуудыг уншдаг Runner-г бүтээнэ.
func New(pool *pgxpool.Pool, dir string) *Runner {
	return &Runner{pool: pool, dir: dir}
}

// SetLogger нь өгөгдмөл logger sink-г дарж бичнэ.
func (r *Runner) SetLogger(fn func(string, logger.Fields)) { r.log = fn }

// Pool нь migration хийсний дараах төлөвийг шалгах шаардлагатай
// дуудагчдад (жишээ нь schema_migrations-г шууд асуудаг integration
// тест) зориулж pool-г илчилнэ.
func (r *Runner) Pool() *pgxpool.Pool { return r.pool }

func (r *Runner) info(msg string, fields logger.Fields) {
	if r.log != nil {
		r.log(msg, fields)
		return
	}
	logger.Info(msg, fields)
}

// conn нь pool-аас холболт авч, advisory lock-ийн дор fn-г ажиллуулна.
// Lock болон migration-ууд НЭГ холболт дээр ажилладаг тул session-scoped
// lock зөв effect-тэй. Зэрэгцээ runner-уудыг (CI + хөгжүүлэгчийн зөөврийн
// компьютер) дараалалд оруулна.
func (r *Runner) withConnLock(ctx context.Context, fn func(ctx context.Context, conn *pgxpool.Conn) error) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(AdvisoryLockID)); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, int64(AdvisoryLockID)); err != nil {
			logger.Error("failed to release migration advisory lock", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryMigration,
				"error":                  err.Error(),
			})
		}
	}()
	return fn(ctx, conn)
}

// Up нь бүх *.up.sql файлыг лексикографийн дарааллаар хэрэгжүүлнэ.
// schema_migrations-д аль хэдийн байгаа файлуудыг алгасдаг тул дахин
// ажиллуулалт idempotent байна. Файл бүр өөрийн statement болон
// schema_migrations мөрийг нэг транзакцид commit хийнэ.
func (r *Runner) Up(ctx context.Context) error {
	r.info("running migration [up]", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryMigration})
	return r.withConnLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := ensureMigrationsTable(ctx, conn); err != nil {
			return fmt.Errorf("create schema_migrations: %w", err)
		}
		files, err := r.listFiles("up")
		if err != nil {
			return err
		}
		applied, err := loadApplied(ctx, conn)
		if err != nil {
			return err
		}
		for _, file := range files {
			name := filepath.Base(file)
			if applied[name] {
				r.info("skipping already-applied migration", logger.Fields{
					constants.LoggerCategory: constants.LoggerCategoryMigration, constants.LoggerFile: name,
				})
				continue
			}
			r.info("applying migration", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryMigration, constants.LoggerFile: name,
			})
			if err := applyFile(ctx, conn, file, name, true); err != nil {
				return err
			}
		}
		r.info("migration [up] success", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryMigration})
		return nil
	})
}

// Down нь бүх *.down.sql файлыг лексикографийн ЭСРЭГ дарааллаар
// хэрэгжүүлнэ. Амжилттай down бүр тохирох schema_migrations мөрийг
// устгана.
func (r *Runner) Down(ctx context.Context) error {
	r.info("running migration [down]", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryMigration})
	return r.withConnLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := ensureMigrationsTable(ctx, conn); err != nil {
			return fmt.Errorf("create schema_migrations: %w", err)
		}
		files, err := r.listFiles("down")
		if err != nil {
			return err
		}
		slices.Reverse(files)
		for _, file := range files {
			name := filepath.Base(file)
			r.info("reverting migration", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryMigration, constants.LoggerFile: name,
			})
			if err := applyFile(ctx, conn, file, deriveUpName(name), false); err != nil {
				return err
			}
		}
		r.info("migration [down] success", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryMigration})
		return nil
	})
}

func ensureMigrationsTable(ctx context.Context, conn *pgxpool.Conn) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name        TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	return err
}

func (r *Runner) listFiles(action string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(r.dir, fmt.Sprintf("*.%s.sql", action)))
	if err != nil {
		return nil, errors.New("glob migration files")
	}
	// Лексикограф эрэмбэ ашиглаж БОЛОХГҮЙ: "10_" нь "1_"-ээс өмнө ордог
	// ('0' < '_') тул шинэ хоосон DB дээр 10-р migration 1-ээс түрүүлж
	// ажиллана. Файлын нэрний эхний дугаараар тоон эрэмбэлнэ.
	slices.SortFunc(files, func(a, b string) int {
		if c := cmp.Compare(migrationNumber(a), migrationNumber(b)); c != 0 {
			return c
		}
		return cmp.Compare(filepath.Base(a), filepath.Base(b))
	})
	return files, nil
}

// migrationNumber нь "N_name.up.sql" маягийн файлын нэрнээс эхний N
// дугаарыг буцаана; дугааргүй файл хамгийн сүүлд эрэмбэлэгдэнэ.
func migrationNumber(path string) int {
	name := filepath.Base(path)
	i := strings.IndexByte(name, '_')
	if i <= 0 {
		return int(^uint(0) >> 1)
	}
	n, err := strconv.Atoi(name[:i])
	if err != nil {
		return int(^uint(0) >> 1)
	}
	return n
}

func loadApplied(ctx context.Context, conn *pgxpool.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

// applyFile нь migration SQL файлыг schema_migrations-ийн бүртгэлийн
// бичилттэй хамт нэг транзакцид ажиллуулдаг — ингэснээр файлын дунд гацах
// нь хэсэгчилсэн бичлэг үлдээдэггүй.
func applyFile(ctx context.Context, conn *pgxpool.Conn, file, upName string, isUp bool) error {
	// #nosec G304 — файлын замууд нь хүсэлтийн оролтоос биш, хөгжүүлэгчийн
	// хяналт дахь migrations директороос ирдэг.
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, string(data)); err != nil {
		return fmt.Errorf("exec %s: %w", filepath.Base(file), err)
	}

	if isUp {
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(name) VALUES ($1) ON CONFLICT DO NOTHING`, upName); err != nil {
			return fmt.Errorf("record migration %s: %w", upName, err)
		}
	} else {
		if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE name = $1`, upName); err != nil {
			return fmt.Errorf("forget migration %s: %w", upName, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", filepath.Base(file), err)
	}
	return nil
}

// deriveUpName нь "*.down.sql" файлын нэрийг түүний "*.up.sql" хослол
// болгон хувиргадаг бөгөөд migration-ууд schema_migrations-д яг ийм
// байдлаар түлхүүрлэгддэг.
func deriveUpName(downName string) string {
	const suffix = ".down.sql"
	if len(downName) > len(suffix) && downName[len(downName)-len(suffix):] == suffix {
		return downName[:len(downName)-len(suffix)] + ".up.sql"
	}
	return downName
}

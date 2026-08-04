//go:build integration

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package testenv нь integration-тестийн harness-г агуулна —
// testcontainers-go-оор асаагдаж, t.Cleanup-ээр унтраагддаг устгаж
// болох Postgres болон Redis контейнерууд, мөн контейнер бүр дээр шинэ
// schema бэлддэг migration loader.
//
// Бүхэл package нь `integration` build tag-аар хаалттай тул `go build
// ./...` болон өгөгдмөл `go test`-ээс хасагддаг. Production binary-ууд
// хэзээ ч testcontainers-go-г холбодоггүй бөгөөд нэгж тестүүдэд хэзээ ч
// Docker хэрэггүй.
//
// Integration тест ажиллуулахын тулд: `make test-integration` (Docker
// шаардана).
package testenv

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	appmodules "github.com/gerege-systems/open-gerege-core/modules"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartPostgresEmpty нь устгагдах Postgres контейнер асааж, uuid-ossp
// өргөтгөлийг суулгаж, түүн рүү чиглэсэн *pgxpool.Pool-г буцаана. Ямар ч
// migration хэрэгжүүлэгдэхгүй. Тест өөрөө schema өөрчлөлтийг
// удирддаг үед (жишээ нь migration runner-ийн өөрийн integration тест)
// үүнийг ашигла.
func StartPostgresEmpty(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return startPostgres(t, false)
}

// StartPostgres нь устгагдах Postgres контейнер асааж,
// migrations/ дахь бүх .up.sql migration-г лексикографийн
// дарааллаар хэрэгжүүлж, холбогдсон *pgxpool.Pool-г буцаана. Контейнерийг
// t.Cleanup зогсоодог тул тест бүр цэвэр эхлэл авдаг; ижил package дахь
// тестүүдийн хооронд юу ч алддаггүй.
func StartPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return startPostgres(t, true)
}

func startPostgres(t *testing.T, runMigrations bool) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const (
		dbName = "boilerplate_test"
		dbUser = "test"
		dbPass = "test"
	)

	c, err := postgres.Run(ctx,
		// pgvector-тэй image — migration 47 нь `CREATE EXTENSION vector`
		// хийдэг тул extension-гүй image дээр интеграци тест унана.
		// Тестэд collation хамаагүй тул албан ёсны Debian суурьтай
		// image-ийг шууд ашиглана (compose нь alpine дээрээ эмхэтгэдэг).
		"pgvector/pgvector:pg16",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	t.Cleanup(func() {
		// Шинэ ctx ашигла — t.Cleanup нь тестийн ctx дууссаны дараа
		// ажилладаг.
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		if err := testcontainers.TerminateContainer(c, testcontainers.StopContext(stopCtx)); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	// uuid_generate_v4()-г users migration ашигладаг.
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`); err != nil {
		t.Fatalf("install uuid-ossp: %v", err)
	}

	if runMigrations {
		if err := applyMigrations(ctx, pool); err != nil {
			t.Fatalf("apply migrations: %v", err)
		}
	}

	return pool
}

// applyMigrations нь бүх .up.sql файлыг ТООН дарааллаар (файлын нэрний
// эхний дугаар) ажиллуулна — лексикограф эрэмбэ "10_"-ыг "1_"-ээс өмнө
// тавьдаг тул болохгүй (runner-ийн listFiles-тэй ижил дүрэм).
// integration тест нь runner-ийн транзакц / schema_migrations бүртгэлээс
// салангид хэвээр байхын тулд cmd/migration-г дахин ашиглахын оронд
// harness дотор inline байлгасан.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir := migrationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".sql" && len(name) > len(".up.sql") && name[len(name)-len(".up.sql"):] == ".up.sql" {
			files = append(files, name)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		ni, nj := migrationNumber(files[i]), migrationNumber(files[j])
		if ni != nj {
			return ni < nj
		}
		return files[i] < files[j]
	})

	for _, name := range files {
		if err := execSQLFile(ctx, pool, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	// Модулиудын өөрийн migration-ууд (kernel/data/migrate) — глобалуудын
	// ДАРАА, cmd/migration-тай ижил дараалал. Модулийн хүснэгтүүд суурийн
	// хүснэгтүүд рүү заадаг тул эхэлж болохгүй.
	return applyModuleMigrations(ctx, pool)
}

// applyModuleMigrations нь модулиудын embed хийсэн migration-уудыг
// modules.MigrationSources()-ийн ДАРААЛЛААР ажиллуулна.
//
// Хавтасны нэрээр эрэмбэлж БОЛОХГҮЙ: цагаан толгойн дараалал нь users-ийг
// хамгийн сүүлд тавьдаг бөгөөд rbac/assets/superadmin нь users-ийг ALTER
// хийдэг тул шинэ DB босохгүй. Deploy (cmd/migration) болон энэ harness
// хоёр ИЖИЛ жагсаалтыг хэрэглэх нь чухал — эс бөгөөс тест ногоон атлаа
// deploy унана.
func applyModuleMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	for _, src := range appmodules.MigrationSources() {
		fsys := src.Migrations()
		names, err := fs.Glob(fsys, "*.up.sql")
		if err != nil {
			return fmt.Errorf("glob %s migrations: %w", src.ID(), err)
		}
		sort.Slice(names, func(i, j int) bool {
			ni, nj := migrationNumber(names[i]), migrationNumber(names[j])
			if ni != nj {
				return ni < nj
			}
			return names[i] < names[j]
		})
		for _, n := range names {
			data, err := fs.ReadFile(fsys, n)
			if err != nil {
				return fmt.Errorf("read %s/%s: %w", src.ID(), n, err)
			}
			if _, err := pool.Exec(ctx, string(data)); err != nil {
				return fmt.Errorf("exec %s/%s: %w", src.ID(), n, err)
			}
		}
	}
	return nil
}

func execSQLFile(ctx context.Context, pool *pgxpool.Pool, full string) error {
	// #nosec G304 — `full` нь хүсэлтийн оролтоос биш, хөгжүүлэгчийн
	// хяналт дахь migrations директороос бүтээгддэг.
	data, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(full), err)
	}
	if _, err := pool.Exec(ctx, string(data)); err != nil {
		return fmt.Errorf("exec %s: %w", filepath.Base(full), err)
	}
	return nil
}

// migrationNumber нь "N_name.up.sql" нэрнээс эхний N дугаарыг буцаана;
// дугааргүй файл хамгийн сүүлд эрэмбэлэгдэнэ.
func migrationNumber(name string) int {
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

// migrationsDir нь go.mod олдтол энэ эх файлаас дээш алхах замаар
// төслийн үндэс дэх migrations директорыг тодорхойлно — ингэснээр
// package нь internal/ дотор зөөгдөхөд harness ажилласаар байна.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "migrations")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// go.mod олохгүйгээр файлын системийн үндэст хүрсэн —
			// буруу директорыг чимээгүй сонгохын оронд доош чанга
			// бүтэлгүйтэх замыг буцаа.
			return "migrations"
		}
		dir = parent
	}
}

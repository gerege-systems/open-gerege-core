//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package testenv

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AppUserPool нь admin (superuser) pool дээр NOSUPERUSER NOBYPASSRLS role
// үүсгэж, users хүснэгтэд DML эрх олгож, тэр role-оор холбогдсон шинэ pool
// буцаана.
//
// Энэ нь яагаад чухал вэ: Postgres-ийн superuser болон BYPASSRLS role нь RLS
// бодлогуудыг АЛГАСдаг. testcontainers-ийн өгөгдмөл хэрэглэгч superuser тул
// түүгээр RLS-ийг шалгаж болохгүй — бодлогууд хэзээ ч ажиллахгүй. RLS бодит
// хэрэгжилтийг батлахын тулд non-superuser role-оор холбогдох ёстой, яг
// production дахь app_user шиг (docs/SECURITY.md — DB role separation).
func AppUserPool(t *testing.T, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const (
		role = "app_user"
		pass = "app_pass"
	)
	stmts := []string{
		`DROP ROLE IF EXISTS ` + role,
		fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOBYPASSRLS`, role, pass),
		`GRANT USAGE ON SCHEMA public TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON users TO ` + role,
	}
	for _, s := range stmts {
		if _, err := admin.Exec(ctx, s); err != nil {
			t.Fatalf("setup app_user role (%q): %v", s, err)
		}
	}

	cc := admin.Config().ConnConfig
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		role, pass, cc.Host, cc.Port, cc.Database)
	appPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect as app_user: %v", err)
	}
	t.Cleanup(appPool.Close)

	// Sanity: энэ холболт ҮНЭХЭЭР RLS-д захирагдах ёстой (superuser/bypass БИШ).
	var super, bypass bool
	if err := appPool.QueryRow(ctx,
		`SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`,
	).Scan(&super, &bypass); err != nil {
		t.Fatalf("check app_user privileges: %v", err)
	}
	if super || bypass {
		t.Fatalf("app_user must be NOSUPERUSER + NOBYPASSRLS, got super=%v bypass=%v", super, bypass)
	}
	return appPool
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"fmt"

	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation нь Postgres-ийн unique_violation-ийн SQLSTATE код юм.
const pgUniqueViolation = "23505"

// postgreUserRepository нь pgx connection pool-г агуулна. Interface-ийн
// method бүр өөрийн файлд (users_store.go, users_get_by_email.go, ...)
// байрладаг тул нэг query-д хүрэх PR diff-үүд нарийн тодорхой хэвээр
// үлддэг.
//
// GORM-ийн автомат soft-delete байхгүй болсон тул query бүр уншихаасаа
// эсвэл бичихээсээ өмнө `deleted_at IS NULL`-г ИЛ-ээр нэмдэг.
type postgreUserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) repointerface.UserRepository {
	return &postgreUserRepository{pool: pool}
}

// withRLS нь нэг query-г транзакцид боож, тухайн транзакцид зориулж Postgres-ийн
// Row-Level Security session хувьсагчдыг (app.user_id, app.user_role) тогтооно.
// Утгуудыг context-оос (rls.FromContext) уншиж авдаг.
//
// Яагаад транзакц шаардлагатай вэ: set_config-ийн гурав дахь аргумент (is_local)
// нь `true` — энэ нь `SET LOCAL`-той дүйцэх бөгөөд утгыг зөвхөн ИДЭВХТЭЙ
// транзакцийн туршид хадгална. pgx нь холболтын pool ашигладаг тул жирийн `SET`
// нь нэг хүсэлтийн identity-г pool дахь холболтод үлдээж, дараагийн хамааралгүй
// хүсэлт рүү "алдагдуулах" эрсдэлтэй; SET LOCAL транзакц commit/rollback
// хийгдмэгц автоматаар арилдаг тул энэ алдагдлаас сэргийлнэ.
//
// context-д Identity байхгүй бол UserID/Role нь хоосон болж, RLS бодлогууд бүх
// мөрийг хаана — аюулгүй өгөгдмөл (fail-closed).
func (r *postgreUserRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
	id, _ := rls.FromContext(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.user_id',$1,true), set_config('app.user_role',$2,true)`,
		id.UserID, string(id.Role),
	); err != nil {
		return fmt.Errorf("set rls session context: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

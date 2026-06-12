// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package org нь organizations болон organization_memberships хүснэгтүүдийн
// Postgres gateway юм. Хоёр хүснэгт хоёулаа RLS-тэй (migration 14): уншилтууд
// нь дуудагчийн identity-г (app.user_id / app.user_role GUC) транзакцид
// тогтоож, гишүүнчлэлд суурилсан харагдах байдлыг хүндэтгэдэг; харин бичих
// үйлдлүүд (org/membership үүсгэх, дүр солих, хасах) нь "service" GUC дор
// явдаг — учир нь шинэ org-ийг үүсгэх агшинд хэрэглэгч хараахан гишүүн
// болоогүй (тиймээс user policy түүнийг хардаггүй) бөгөөд бизнесийн эрхийг
// (owner/admin) usecase давхарга аль хэдийн шалгасан байдаг.
package org

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

// orgRepository нь pgx connection pool-г агуулна. Interface-ийн method бүр
// өөрийн файлд байрладаг тул нэг query-д хүрэх PR diff-үүд нарийн тодорхой
// хэвээр үлддэг.
type orgRepository struct {
	pool *pgxpool.Pool
}

// NewOrgRepository нь OrgRepository хэрэгжүүлэлтийг буцаана.
func NewOrgRepository(pool *pgxpool.Pool) repointerface.OrgRepository {
	return &orgRepository{pool: pool}
}

// withRLS нь нэг query-г транзакцид боож, тухайн транзакцид зориулж Postgres-ийн
// Row-Level Security session хувьсагчдыг (app.user_id, app.user_role)
// context-оос (rls.FromContext) уншиж тогтооно. users repo-гийн withRLS-тэй
// яг ижил загвар — SET LOCAL (set_config is_local=true) нь pool дахь холболтод
// identity алдагдахаас сэргийлдэг. context-д Identity байхгүй бол хоосон GUC
// тавигдаж RLS бодлогууд бүх мөрийг хаана (fail-closed).
func (r *orgRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
	id, _ := rls.FromContext(ctx)
	return r.runTx(ctx, id.UserID, string(id.Role), fn)
}

// withService нь транзакцийг "service" RLS дүрээр тогтоож гүйцэтгэнэ. Бичих
// үйлдлүүдэд ашиглана — usecase нь бизнесийн эрхийг аль хэдийн шалгасан тул
// энэ нь least-privilege-г зөрчихгүй (rbac repo-гийн CountUsersWithRole-той
// ижил арга барил).
func (r *orgRepository) withService(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return r.runTx(ctx, "", string(rls.RoleService), fn)
}

// runTx нь GUC-уудыг тогтоож, fn-г транзакцид гүйцэтгэдэг хуваалцсан туслах.
func (r *orgRepository) runTx(ctx context.Context, userID, role string, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.user_id',$1,true), set_config('app.user_role',$2,true)`,
		userID, role,
	); err != nil {
		return fmt.Errorf("set rls session context: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

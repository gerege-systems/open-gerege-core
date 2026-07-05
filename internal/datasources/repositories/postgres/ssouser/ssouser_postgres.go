// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ssouser нь Gerege SSO (OIDC)-ээр нэвтэрсэн иргэнийг pairwise subject
// (sso_sub)-ээр users хүснэгтэд upsert хийнэ. eID upsert-ийн адил RLS "service"
// context дор ажиллана (SSO callback нь /v1/sso бүлгийн ServiceRLSContext-тэй).
package ssouser

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/internal/datasources/rls"
)

type ssoUserRepository struct {
	pool *pgxpool.Pool
}

// NewSSOUserRepository нь SSO хэрэглэгчийн repo үүсгэнэ.
func NewSSOUserRepository(pool *pgxpool.Pool) *ssoUserRepository {
	return &ssoUserRepository{pool: pool}
}

// withRLS нь users_postgres.go-ийн загварын дагуу нэг транзакцийн туршид
// app.user_id / app.user_role GUC-уудыг тавьж fn-г ажиллуулна. SSO callback нь
// нэвтрэхээс өмнөх урсгал тул context ихэвчлэн RoleService-тэй ирнэ.
func (r *ssoUserRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
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

// UpsertBySSOSub нь ssoSub (pairwise subject)-ээр иргэнийг олж, байхгүй бол шинэ
// хэрэглэгч (username/email синтетик, role = RoleUser, password NULL) үүсгэнэ;
// байвал нэрийг шинэчилж, идэвхжүүлнэ. Хадгалагдсан мөрийг domain-аар буцаана.
func (r *ssoUserRepository) UpsertBySSOSub(ctx context.Context, ssoSub string, in *domain.User) (domain.User, error) {
	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			INSERT INTO users(id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, sso_sub, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, $6, NULL, true, $7, $8, now())
			ON CONFLICT (sso_sub) WHERE sso_sub IS NOT NULL
			DO UPDATE SET
				first_name    = EXCLUDED.first_name,
				last_name     = EXCLUDED.last_name,
				first_name_en = EXCLUDED.first_name_en,
				last_name_en  = EXCLUDED.last_name_en,
				active        = true,
				updated_at    = now()
			RETURNING `+records.UserColumns+`
		`,
			in.Username, in.FirstName, in.LastName, in.FirstNameEn, in.LastNameEn,
			in.Email, in.RoleID, ssoSub,
		)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err != nil {
		return domain.User{}, err
	}
	if stored.Id == "" {
		return domain.User{}, fmt.Errorf("sso upsert succeeded but RETURNING produced no row")
	}
	return stored.ToV1Domain(), nil
}

// UpsertByCivilID нь SSO иргэнийг civil_id-ээр (eID хэрэглэгчийн тогтвортой
// түлхүүр) тааруулна: тухайн civil_id-тэй хэрэглэгч (eID-ээр урьд бүртгэгдсэн)
// байвал тэр мөрд sso_sub-ыг холбож (role_id/email ХӨНДӨХГҮЙ), нэрийг нөхнө;
// байхгүй бол шинэ хэрэглэгч (username eid_<civilID>, role RoleUser) үүсгэнэ.
// Ингэснээр eID болон SSO нэвтрэлт нэг л данс болно (давхардал үүсэхгүй).
func (r *ssoUserRepository) UpsertByCivilID(ctx context.Context, civilID, nationalID, ssoSub string, in *domain.User) (domain.User, error) {
	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			INSERT INTO users(id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, national_id, civil_id, sso_sub, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NULL, NULL, true, $6, $7, $8, $9, now())
			ON CONFLICT (lower(civil_id)) WHERE civil_id IS NOT NULL
			DO UPDATE SET
				sso_sub    = EXCLUDED.sso_sub,
				first_name = COALESCE(NULLIF(EXCLUDED.first_name, ''), users.first_name),
				last_name  = COALESCE(NULLIF(EXCLUDED.last_name, ''), users.last_name),
				active     = true,
				updated_at = now()
			RETURNING `+records.UserColumns+`
		`,
			in.Username, in.FirstName, in.LastName, in.FirstNameEn, in.LastNameEn,
			in.RoleID, nationalID, civilID, ssoSub,
		)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err != nil {
		return domain.User{}, err
	}
	if stored.Id == "" {
		return domain.User{}, fmt.Errorf("sso civil upsert succeeded but RETURNING produced no row")
	}
	return stored.ToV1Domain(), nil
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ssouser нь dgov SSO (OIDC)-ээр нэвтэрсэн иргэнийг pairwise subject
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
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit returns ErrTxClosed — expected, nothing to handle

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
			INSERT INTO users(id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, sso_sub, google_sub, google_email, google_name, google_picture, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, $6, NULL, true, $7, $8, NULLIF($9,''), NULLIF($10,''), NULLIF($11,''), NULLIF($12,''), now())
			ON CONFLICT (sso_sub) WHERE sso_sub IS NOT NULL
			DO UPDATE SET
				first_name     = EXCLUDED.first_name,
				last_name      = EXCLUDED.last_name,
				first_name_en  = EXCLUDED.first_name_en,
				last_name_en   = EXCLUDED.last_name_en,
				google_sub     = COALESCE(EXCLUDED.google_sub, users.google_sub),
				google_email   = COALESCE(EXCLUDED.google_email, users.google_email),
				google_name    = COALESCE(EXCLUDED.google_name, users.google_name),
				google_picture = COALESCE(EXCLUDED.google_picture, users.google_picture),
				active         = true,
				updated_at     = now()
			RETURNING `+records.UserColumns+`
		`,
			in.Username, in.FirstName, in.LastName, in.FirstNameEn, in.LastNameEn,
			in.Email, in.RoleID, ssoSub, in.GoogleSub, in.GoogleEmail, in.GoogleName, in.GooglePicture,
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
		// Хамгийн эхэнд: админаас урьдчилан бүртгэсэн мөр (national_id-тай, гэхдээ
		// civil_id/sso_sub-гүй) байвал энэ нэвтрэлтэд ХОЛБОНО. Private платформд
		// админ иргэнийг регистрийн дугаар (national_id)-аар урьдчилан бүртгэдэг тул
		// иргэн эхлээд SSO-оор нэвтрэхэд тэр мөрийг олж, civil_id/sso_sub-ыг залгана
		// (шинэ давхардсан мөр үүсгэхгүй). role_id/email хөндөхгүй.
		if nationalID != "" {
			natRows, nErr := tx.Query(ctx, `
				UPDATE users SET
					civil_id       = $2,
					sso_sub        = $3,
					first_name     = COALESCE(NULLIF($4,''), first_name),
					last_name      = COALESCE(NULLIF($5,''), last_name),
					first_name_en  = COALESCE(NULLIF($6,''), first_name_en),
					last_name_en   = COALESCE(NULLIF($7,''), last_name_en),
					active         = true,
					updated_at     = now()
				WHERE lower(national_id) = lower($1)
				  AND (civil_id IS NULL OR civil_id = '')
				  AND (sso_sub  IS NULL OR sso_sub  = '')
				RETURNING `+records.UserColumns+`
			`, nationalID, civilID, ssoSub, in.FirstName, in.LastName, in.FirstNameEn, in.LastNameEn)
			if nErr != nil {
				return nErr
			}
			natPromoted, nScanErr := pgx.CollectRows(natRows, pgx.RowToStructByName[records.Users])
			if nScanErr != nil {
				return nScanErr
			}
			if len(natPromoted) == 1 {
				stored = natPromoted[0]
				return nil
			}
		}

		// Дараа нь: civil_id-гүй байсан ПАЙРВАЙЗ (sso_sub) мөр байвал түүнд civil_id/
		// national_id-ыг нэмж "дэвшүүлнэ". Ингэснээр иргэн урьд SSO-гоор
		// nationalid scope-ГҮЙ нэвтэрч (sso_sub мөр үүсгээд) дараа nationalid-тай
		// эргэж ирэхэд доорх INSERT нь давхардсан sso_sub-д мөргөлдөхгүй.
		promoteRows, pErr := tx.Query(ctx, `
			UPDATE users SET
				civil_id       = $2,
				national_id    = $3,
				first_name     = COALESCE(NULLIF($4,''), first_name),
				last_name      = COALESCE(NULLIF($5,''), last_name),
				first_name_en  = COALESCE(NULLIF($6,''), first_name_en),
				last_name_en   = COALESCE(NULLIF($7,''), last_name_en),
				google_sub     = COALESCE(NULLIF($8,''), google_sub),
				google_email   = COALESCE(NULLIF($9,''), google_email),
				google_name    = COALESCE(NULLIF($10,''), google_name),
				google_picture = COALESCE(NULLIF($11,''), google_picture),
				active         = true,
				updated_at     = now()
			WHERE sso_sub = $1 AND (civil_id IS NULL OR civil_id = '')
			RETURNING `+records.UserColumns+`
		`, ssoSub, civilID, nationalID, in.FirstName, in.LastName, in.FirstNameEn, in.LastNameEn,
			in.GoogleSub, in.GoogleEmail, in.GoogleName, in.GooglePicture)
		if pErr != nil {
			return pErr
		}
		promoted, pScanErr := pgx.CollectRows(promoteRows, pgx.RowToStructByName[records.Users])
		if pScanErr != nil {
			return pScanErr
		}
		if len(promoted) == 1 {
			stored = promoted[0]
			return nil
		}

		// Пайрвайз мөр байхгүй — civil_id-ээр INSERT/merge (шинэ иргэн эсвэл eID-
		// ээр урьд бүртгэгдсэн мөртэй нэгтгэх).
		rows, qErr := tx.Query(ctx, `
			INSERT INTO users(id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, national_id, civil_id, sso_sub, google_sub, google_email, google_name, google_picture, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NULL, NULL, true, $6, $7, $8, $9, NULLIF($10,''), NULLIF($11,''), NULLIF($12,''), NULLIF($13,''), now())
			ON CONFLICT (lower(civil_id)) WHERE civil_id IS NOT NULL
			DO UPDATE SET
				sso_sub        = EXCLUDED.sso_sub,
				first_name     = COALESCE(NULLIF(EXCLUDED.first_name, ''), users.first_name),
				last_name      = COALESCE(NULLIF(EXCLUDED.last_name, ''), users.last_name),
				first_name_en  = COALESCE(NULLIF(EXCLUDED.first_name_en, ''), users.first_name_en),
				last_name_en   = COALESCE(NULLIF(EXCLUDED.last_name_en, ''), users.last_name_en),
				google_sub     = COALESCE(EXCLUDED.google_sub, users.google_sub),
				google_email   = COALESCE(EXCLUDED.google_email, users.google_email),
				google_name    = COALESCE(EXCLUDED.google_name, users.google_name),
				google_picture = COALESCE(EXCLUDED.google_picture, users.google_picture),
				active         = true,
				updated_at     = now()
			RETURNING `+records.UserColumns+`
		`,
			in.Username, in.FirstName, in.LastName, in.FirstNameEn, in.LastNameEn,
			in.RoleID, nationalID, civilID, ssoSub, in.GoogleSub, in.GoogleEmail, in.GoogleName, in.GooglePicture,
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

// AuthorizedByCivilOrNational нь private платформын хандалтын шалгуур: өгсөн
// civil_id ЭСВЭЛ national_id-аар тохирох (устгаагүй) хэрэглэгч байвал true.
// Private горимд урьдчилан бүртгэгдээгүй иргэн eID-ээр баталгаажсан ч нэвтрэхгүй.
func (r *ssoUserRepository) AuthorizedByCivilOrNational(ctx context.Context, civilID, nationalID string) (bool, error) {
	var exists bool
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM users
				WHERE deleted_at IS NULL
				  AND (
				    ($1 <> '' AND lower(civil_id)    = lower($1)) OR
				    ($2 <> '' AND lower(national_id) = lower($2))
				  )
			)`, civilID, nationalID).Scan(&exists)
	})
	if err != nil {
		return false, fmt.Errorf("check platform authorization: %w", err)
	}
	return exists, nil
}

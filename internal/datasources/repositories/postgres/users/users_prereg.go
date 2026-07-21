// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
)

// CreatePreRegistered нь админ иргэнийг РЕГИСТРИЙН ДУГААР (national_id)-аар
// урьдчилан бүртгэнэ (private платформын хандалт): национал дугаар + нэр + role-
// той идэвхтэй мөр үүсгэнэ, гэхдээ password/email/civil_id/sso_sub-гүй. Иргэн
// хожим Government SSO (eID)-ээр эхэлж нэвтэрхэд ssouser upsert нь энэ мөрийг
// national_id-аар олж, civil_id/sso_sub-ыг залгана (давхардал үүсэхгүй).
//
// Тухайн national_id аль хэдийн бүртгэлтэй бол apperror.Conflict.
func (r *postgreUserRepository) CreatePreRegistered(ctx context.Context, in *domain.User) (domain.User, error) {
	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			INSERT INTO users(id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, national_id, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NULL, NULL, true, $6, $7, now())
			RETURNING `+records.UserColumns+`
		`, in.Username, in.FirstName, in.LastName, in.FirstNameEn, in.LastNameEn, in.RoleID, in.NationalID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation — national_id (эсвэл username) давхардсан
				return domain.User{}, apperror.Conflict("энэ регистрийн дугаар аль хэдийн бүртгэлтэй байна")
			case "23503": // foreign_key_violation — role_id байхгүй
				return domain.User{}, apperror.BadRequest("unknown role")
			}
		}
		return domain.User{}, err
	}
	if stored.Id == "" {
		return domain.User{}, fmt.Errorf("pre-register insert succeeded but RETURNING produced no row")
	}
	return stored.ToV1Domain(), nil
}

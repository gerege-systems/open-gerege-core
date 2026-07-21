// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"errors"
	"fmt"

	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"
)

// UpsertSuperAdmin нь superadmin onboarding-ийн ТӨГСГӨЛД (Google + eID + email
// OTP + TOTP бүгд баталгаажсаны дараа) super admin хэрэглэгчийг үүсгэх/ахиулна.
//
// Түлхүүр нь UpsertFromEID-ийн адил civil_id (idx_users_civil_id_active partial
// unique index, migration 13): тухайн иргэн аль хэдийн eID-ээр нэвтэрч байсан
// бол түүний мөрийг ахиулж (role_id, email + email_verified, mfa_enabled,
// totp_secret, Google профайл), эс бөгөөс шинэ идэвхтэй super admin мөр
// оруулна. Нэг round-trip (INSERT … ON CONFLICT … RETURNING).
//
// АНХААР: totp_secret нь usecase давхаргад AES-GCM-ээр аль хэдийн шифрлэгдсэн
// ирнэ — энэ давхаргад ил текст secret ХЭЗЭЭ Ч бичигдэхгүй. Латин нэрийг
// UpsertFromEID-ийн адил COALESCE-оор нэг л удаа авна (хэрэглэгчийн гар
// засварыг дарж бичихгүй). Нууц үг NULL — super admin нь Google + TOTP-оор л
// нэвтэрнэ.
func (r *postgreUserRepository) UpsertSuperAdmin(ctx context.Context, inDom *domain.User, account *domain.SuperadminAccount) (domain.User, error) {
	const (
		repositoryName = "users"
		funcName       = "UpsertSuperAdmin"
		queryName      = "upsertSuperAdmin"
		fileName       = "users_superadmin.go"
	)
	rec := records.FromUsersV1Domain(inDom)

	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		// 1) users мөр — google_sub-аар түлхүүрлэсэн (civil_id/MFA users-д ТАВИХГҮЙ).
		//    Ингэснээр нэг хүн eID-ээр admin, Google-оор super admin байж чадна.
		rows, qErr := tx.Query(ctx, `
			INSERT INTO users(
				id, username, first_name, last_name, first_name_en, last_name_en,
				email, password, active, role_id, kyc_level,
				google_sub, google_email, google_email_verified, google_name, google_picture, google_linked_at,
				created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5,
				$6, NULL, true, $7, $8,
				$9, $10, $11, $12, $13, now(),
				now())
			ON CONFLICT (google_sub) WHERE google_sub IS NOT NULL AND deleted_at IS NULL
			DO UPDATE SET
				first_name            = EXCLUDED.first_name,
				last_name             = EXCLUDED.last_name,
				-- Латин нэрийг НЭГ УДАА (анхны insert-д) л авна; хэрэглэгчийн
				-- гараар засварласан утгыг дарж бичихгүй (UpsertFromEID-ийн адил).
				first_name_en         = COALESCE(users.first_name_en, EXCLUDED.first_name_en),
				last_name_en          = COALESCE(users.last_name_en, EXCLUDED.last_name_en),
				kyc_level             = EXCLUDED.kyc_level,
				email                 = EXCLUDED.email,
				google_email          = EXCLUDED.google_email,
				google_email_verified = EXCLUDED.google_email_verified,
				google_name           = EXCLUDED.google_name,
				google_picture        = EXCLUDED.google_picture,
				google_linked_at      = COALESCE(users.google_linked_at, now()),
				role_id               = EXCLUDED.role_id,
				active                = true,
				updated_at            = now()
			RETURNING `+records.UserColumns+`
		`,
			rec.Username, rec.FirstName, rec.LastName, rec.FirstNameEn, rec.LastNameEn,
			rec.Email, rec.RoleId, rec.KYCLevel,
			rec.GoogleSub, rec.GoogleEmail, rec.GoogleEmailVerified, rec.GoogleName, rec.GooglePicture)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		if scanErr != nil {
			return scanErr
		}
		// 2) superadmin_accounts satellite мөр — ижил транзакцид (атом).
		if _, aErr := tx.Exec(ctx, `
			INSERT INTO superadmin_accounts(
				user_id, civil_id, national_id, email_verified, mfa_enabled, totp_secret, invited_by, onboarded_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, now())
			ON CONFLICT (user_id) DO UPDATE SET
				civil_id       = EXCLUDED.civil_id,
				national_id    = EXCLUDED.national_id,
				email_verified = EXCLUDED.email_verified,
				mfa_enabled    = EXCLUDED.mfa_enabled,
				totp_secret    = EXCLUDED.totp_secret,
				invited_by     = COALESCE(NULLIF(EXCLUDED.invited_by, ''), superadmin_accounts.invited_by),
				onboarded_at   = COALESCE(superadmin_accounts.onboarded_at, now()),
				updated_at     = now()
		`,
			stored.Id, nullStr(account.CivilID), nullStr(account.NationalID),
			account.EmailVerified, account.MFAEnabled, nullStr(account.TOTPSecret), account.InvitedBy); aErr != nil {
			return aErr
		}
		return nil
	})
	if err == nil {
		if stored.Id == "" {
			e := fmt.Errorf("upsert succeeded but RETURNING produced no row")
			logger.ErrorWithContext(ctx, "superadmin upsert returned no row", logger.Fields{
				"repository": repositoryName, "method": funcName, "query": queryName,
				"file": fileName, "error": e.Error(), "table": "users",
			})
			return domain.User{}, apperror.InternalCause(e)
		}
		// Буцаах user-ыг satellite account-ийн MFA утгуудаар hydrate хийнэ (users
		// хүснэгтэд эдгээр багана байхгүй; дуудагч mintSession/response-д ашиглана).
		dom := stored.ToV1Domain()
		dom.EmailVerified = account.EmailVerified
		dom.MFAEnabled = account.MFAEnabled
		dom.TOTPSecret = account.TOTPSecret
		return dom, nil
	}

	// Урьсан и-мэйл / Google account өөр бүртгэлд аль хэдийн эзэмшигдсэн бол
	// (idx_users_email_active / idx_users_google_sub_active) цэвэр 409 болгоно.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		logger.ErrorWithContext(ctx, "superadmin upsert conflict", logger.Fields{
			"repository": repositoryName, "method": funcName, "query": queryName,
			"file": fileName, "constraint": pgErr.ConstraintName, "table": "users",
		})
		return domain.User{}, apperror.Conflict("this email or Google account is already linked to another user")
	}
	logger.ErrorWithContext(ctx, "Failed to upsert super admin", logger.Fields{
		"repository": repositoryName, "method": funcName, "query": queryName,
		"file": fileName, "error": err.Error(), "table": "users",
	})
	return domain.User{}, apperror.InternalCause(fmt.Errorf("upsert super admin: %w", err))
}

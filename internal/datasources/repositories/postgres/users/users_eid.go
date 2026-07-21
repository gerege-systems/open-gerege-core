// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
)

// GetByNationalID нь eID-ийн national_id-ээр (жижиг үсгээр харьцуулж) хэрэглэгч
// хайна. Soft-delete хийгдсэн мөрүүдийг хасна.
func (r *postgreUserRepository) GetByNationalID(ctx context.Context, nationalID string) (domain.User, error) {
	const (
		repositoryName = "users"
		funcName       = "GetByNationalID"
		queryName      = "selectUserByNationalID"
		fileName       = "users_eid.go"
	)

	var stored records.Users
	qerr := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.UserColumns+` FROM users WHERE lower(national_id) = lower($1) AND deleted_at IS NULL`, nationalID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if qerr == nil {
		return stored.ToV1Domain(), nil
	}
	if errors.Is(qerr, pgx.ErrNoRows) {
		return domain.User{}, apperror.NotFound("user not found")
	}
	logger.ErrorWithContext(ctx, "Failed to query user by national_id", logger.Fields{
		"repository": repositoryName, "method": funcName, "query": queryName,
		"file": fileName, "error": qerr.Error(), "table": "users",
	})
	return domain.User{}, qerr
}

// UpsertFromEID нь eID identity-аар хэрэглэгчийг үүсгэх/шинэчлэх. civil_id
// дээрх partial unique index (idx_users_civil_id_active, migration 13)-д
// тулгуурлан ON CONFLICT хийнэ: давхцвал нэр (мн+en), national_id ба
// kyc_level-ийг шинэчилж, идэвхжүүлж, updated_at-г тэмдэглэнэ; эс бөгөөс шинэ
// идэвхтэй мөр оруулна. Public RP-д IdP national_id-г илчлэхгүй тул түлхүүр нь
// civil_id (national_id хоосон байж болзошгүй). civil_id-г жижиг үсгээр,
// national_id хоосон бол NULL-ээр (records.ptrOrNil) хадгална — эс бөгөөс
// lower(national_id) partial unique index олон eID хэрэглэгчид мөргөлдөнө.
// eID хэрэглэгч нууц үг/email-гүй тул эдгээрийг NULL-ээр (nil pointer) хадгална.
func (r *postgreUserRepository) UpsertFromEID(ctx context.Context, inDom *domain.User) (domain.User, error) {
	const (
		repositoryName = "users"
		funcName       = "UpsertFromEID"
		queryName      = "upsertUserFromEID"
		fileName       = "users_eid.go"
	)
	rec := records.FromUsersV1Domain(inDom)

	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			INSERT INTO users(id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, national_id, civil_id, kyc_level, document_number, cert_serial, cert_not_before, cert_not_after, cert_issuer, cert_key_type, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, NULL, NULL, true, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			ON CONFLICT (lower(civil_id)) WHERE civil_id IS NOT NULL
			DO UPDATE SET
				first_name      = EXCLUDED.first_name,
				last_name       = EXCLUDED.last_name,
				-- Латин нэрийг НЭГ УДАА (анхны insert-д) л eID-ээс авна; дараа нь дарж
				-- бичихгүй (COALESCE) — учир нь автомат галиглалт заримдаа буруу тул
				-- хэрэглэгч гараар засах бөгөөд тэр засвар нь дараагийн нэвтрэлтэд хэвээр.
				first_name_en   = COALESCE(users.first_name_en, EXCLUDED.first_name_en),
				last_name_en    = COALESCE(users.last_name_en, EXCLUDED.last_name_en),
				national_id     = EXCLUDED.national_id,
				kyc_level       = EXCLUDED.kyc_level,
				document_number = EXCLUDED.document_number,
				cert_serial     = EXCLUDED.cert_serial,
				cert_not_before = EXCLUDED.cert_not_before,
				cert_not_after  = EXCLUDED.cert_not_after,
				cert_issuer     = EXCLUDED.cert_issuer,
				cert_key_type   = EXCLUDED.cert_key_type,
				active          = true,
				updated_at      = now()
			RETURNING `+records.UserColumns+`
		`,
			rec.Username, rec.FirstName, rec.LastName, rec.FirstNameEn, rec.LastNameEn,
			rec.RoleId, rec.NationalID, rec.CivilID, rec.KYCLevel,
			rec.DocumentNumber, rec.CertSerial, rec.CertNotBefore, rec.CertNotAfter, rec.CertIssuer, rec.CertKeyType,
			rec.CreatedAt)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err == nil {
		if stored.Id == "" {
			e := fmt.Errorf("upsert succeeded but RETURNING produced no row")
			logger.ErrorWithContext(ctx, "eID upsert returned no row", logger.Fields{
				"repository": repositoryName, "method": funcName, "query": queryName,
				"file": fileName, "error": e.Error(), "table": "users",
			})
			return domain.User{}, e
		}
		return stored.ToV1Domain(), nil
	}
	logger.ErrorWithContext(ctx, "Failed to upsert eID user", logger.Fields{
		"repository": repositoryName, "method": funcName, "query": queryName,
		"file": fileName, "error": err.Error(), "table": "users",
	})
	return domain.User{}, err
}

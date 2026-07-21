// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package recovery нь 2FA нөөц кодуудын (user_recovery_codes) Postgres gateway
// юм. Хүснэгт нь per-user тул RLS-тэй (migration 35): query бүр withRLS
// транзакцид app.user_id / app.user_role GUC-ийг context-оос уншиж тавина.
// DB-д зөвхөн SHA-256 hash хадгалагдана — энгийн текст код энд хэзээ ч ирэхгүй.
package recovery

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"
)

type recoveryRepository struct {
	pool *pgxpool.Pool
}

func NewRecoveryCodeRepository(pool *pgxpool.Pool) repointerface.RecoveryCodeRepository {
	return &recoveryRepository{pool: pool}
}

// recoveryColumns нь SELECT-д ашиглах баганууд — pgx.RowToStructByName нь
// `db` tag-аар тааруулдаг тул нэг эх сурвалжид төвлөрүүлэв.
const recoveryColumns = "id, user_id, code_hash, used_at, created_at"

// recoveryRecord нь user_recovery_codes хүснэгтийн pgx record. used_at нь
// nullable (хэрэглэгдээгүй код) тул *time.Time.
type recoveryRecord struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	CodeHash  string     `db:"code_hash"`
	UsedAt    *time.Time `db:"used_at"`
	CreatedAt time.Time  `db:"created_at"`
}

// toDomain нь record-ийг domain entity рүү буулгана.
func (rec recoveryRecord) toDomain() domain.RecoveryCode {
	return domain.RecoveryCode{
		ID:        rec.ID,
		UserID:    rec.UserID,
		CodeHash:  rec.CodeHash,
		UsedAt:    rec.UsedAt,
		CreatedAt: rec.CreatedAt,
	}
}

// withRLS нь users repo-гийн адил — нэг үйлдлийг транзакцид боож, тухайн
// транзакцид зориулж RLS session хувьсагчдыг (SET LOCAL) тогтооно. context-д
// Identity байхгүй бол GUC хоосон болж бодлогууд бүх мөрийг ХААНА (fail-closed).
func (r *recoveryRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
	id, _ := rls.FromContext(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit-ийн дараах rollback нь ErrTxClosed — хүлээгдсэн

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

// Replace нь хэрэглэгчийн хуучин кодуудыг устгаад шинэ hash-уудыг нэг
// транзакцид оруулна — нөөц кодыг дахин үүсгэх нь хуучныг бүрмөсөн хүчингүй
// болгоно.
func (r *recoveryRepository) Replace(ctx context.Context, userID string, hashes []string) error {
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		if _, delErr := tx.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); delErr != nil {
			return delErr
		}
		for _, h := range hashes {
			if _, insErr := tx.Exec(ctx,
				`INSERT INTO user_recovery_codes(user_id, code_hash) VALUES ($1, $2)`, userID, h); insErr != nil {
				return insErr
			}
		}
		return nil
	})
	if err != nil {
		return apperror.InternalCause(fmt.Errorf("replace recovery codes: %w", err))
	}
	return nil
}

// ListActive нь хэрэглэгдээгүй кодуудыг (шинэ нь эхэндээ) буцаана.
func (r *recoveryRepository) ListActive(ctx context.Context, userID string) ([]domain.RecoveryCode, error) {
	var recs []recoveryRecord
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+recoveryColumns+` FROM user_recovery_codes
			  WHERE user_id = $1 AND used_at IS NULL
			  ORDER BY created_at DESC`, userID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		recs, scanErr = pgx.CollectRows(rows, pgx.RowToStructByName[recoveryRecord])
		return scanErr
	})
	if err != nil {
		return nil, apperror.InternalCause(fmt.Errorf("list recovery codes: %w", err))
	}
	out := make([]domain.RecoveryCode, 0, len(recs))
	for _, rec := range recs {
		out = append(out, rec.toDomain())
	}
	return out, nil
}

// Consume нь өгсөн hash-тай, хэрэглэгдээгүй НЭГ кодыг атомаар тэмдэглэнэ.
// `used_at IS NULL` нөхцөл UPDATE дотор байгаа тул зэрэгцээ хоёр хүсэлт нэг
// кодыг хоёуланг нь хэрэглэж чадахгүй (нэг нь 0 мөр → NotFound). Тохирох
// идэвхтэй код байхгүй бол apperror.NotFound.
func (r *recoveryRepository) Consume(ctx context.Context, userID, hash string) error {
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, execErr := tx.Exec(ctx,
			`UPDATE user_recovery_codes
			    SET used_at = now()
			  WHERE id = (
			        SELECT id FROM user_recovery_codes
			         WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
			         LIMIT 1
			        )`, userID, hash)
		if execErr != nil {
			return execErr
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("recovery code not found or already used")
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if _, ok := err.(*apperror.DomainError); ok {
		return err
	}
	return apperror.InternalCause(fmt.Errorf("consume recovery code: %w", err))
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package superadminaccount нь super admin-ы satellite бүртгэлийн (superadmin_accounts)
// Postgres READ gateway юм. Хүснэгт нь эмзэг (TOTP secret) тул RLS-тэй (service/admin
// — migration 37): query бүр withRLS транзакцид app.user_id/app.user_role GUC-ийг
// context-оос уншиж тавина. Бичилтийг UserRepository.UpsertSuperAdmin нь users мөртэй
// нэг транзакцид хийдэг тул энд зөвхөн унших метод байна.
package superadminaccount

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"
)

type superadminAccountRepository struct {
	pool *pgxpool.Pool
}

func NewSuperadminAccountRepository(pool *pgxpool.Pool) repointerface.SuperadminAccountRepository {
	return &superadminAccountRepository{pool: pool}
}

// accountColumns нь SELECT-д ашиглах баганууд — pgx.RowToStructByName `db` tag-аар тааруулна.
const accountColumns = "user_id, civil_id, national_id, email_verified, mfa_enabled, totp_secret, invited_by, onboarded_at, created_at, updated_at"

type accountRecord struct {
	UserID        string     `db:"user_id"`
	CivilID       *string    `db:"civil_id"`
	NationalID    *string    `db:"national_id"`
	EmailVerified bool       `db:"email_verified"`
	MFAEnabled    bool       `db:"mfa_enabled"`
	TOTPSecret    *string    `db:"totp_secret"`
	InvitedBy     string     `db:"invited_by"`
	OnboardedAt   *time.Time `db:"onboarded_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     *time.Time `db:"updated_at"`
}

func (rec accountRecord) toDomain() domain.SuperadminAccount {
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return domain.SuperadminAccount{
		UserID:        rec.UserID,
		CivilID:       deref(rec.CivilID),
		NationalID:    deref(rec.NationalID),
		EmailVerified: rec.EmailVerified,
		MFAEnabled:    rec.MFAEnabled,
		TOTPSecret:    deref(rec.TOTPSecret),
		InvitedBy:     rec.InvitedBy,
		OnboardedAt:   rec.OnboardedAt,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
}

// withRLS нь recovery repo-гийн адил — нэг үйлдлийг транзакцид боож, RLS session
// хувьсагчдыг (SET LOCAL) context-оос тогтооно. Identity байхгүй бол GUC хоосон болж
// бодлогууд бүх мөрийг ХААНА (fail-closed).
func (r *superadminAccountRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
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

// Get нь user_id-аар super admin бүртгэлийг буцаана; байхгүй бол apperror.NotFound.
func (r *superadminAccountRepository) Get(ctx context.Context, userID string) (domain.SuperadminAccount, error) {
	var rec accountRecord
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+accountColumns+` FROM superadmin_accounts WHERE user_id = $1`, userID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		rec, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[accountRecord])
		return scanErr
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SuperadminAccount{}, apperror.NotFound("superadmin account not found")
		}
		return domain.SuperadminAccount{}, apperror.InternalCause(fmt.Errorf("get superadmin account: %w", err))
	}
	return rec.toDomain(), nil
}

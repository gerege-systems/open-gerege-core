// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package superadmininvite нь superadmin урилгын allow-list
// (superadmin_invites) Postgres gateway юм. Хэрэглэгч-тус-бүрийн биш, зөвхөн
// super admin-аар удирдагддаг нийтийн config хүснэгт тул Row-Level Security-д
// хамаарахгүй (plain pool query) — site/theme repo-той ижил зарчим.
package superadmininvite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

// pgUniqueViolation нь Postgres-ийн unique_violation-ийн SQLSTATE код.
const pgUniqueViolation = "23505"

// inviteColumns нь SELECT-д ашиглах баганууд (pgx.RowToStructByName `db` tag).
const inviteColumns = "email, invited_by, created_at, accepted_at"

type inviteRepository struct {
	pool *pgxpool.Pool
}

func NewSuperadminInviteRepository(pool *pgxpool.Pool) repointerface.SuperadminInviteRepository {
	return &inviteRepository{pool: pool}
}

// inviteRecord нь superadmin_invites хүснэгтийн pgx record. accepted_at нь
// nullable (ашиглагдаагүй урилга) тул *time.Time.
type inviteRecord struct {
	Email      string     `db:"email"`
	InvitedBy  string     `db:"invited_by"`
	CreatedAt  time.Time  `db:"created_at"`
	AcceptedAt *time.Time `db:"accepted_at"`
}

func (rec inviteRecord) toDomain() domain.SuperadminInvite {
	return domain.SuperadminInvite{
		Email:      rec.Email,
		InvitedBy:  rec.InvitedBy,
		CreatedAt:  rec.CreatedAt,
		AcceptedAt: rec.AcceptedAt,
	}
}

// Create нь урилга оруулж, оруулсан мөрийг нэг round-trip-д буцаана. Давхардсан
// и-мэйл дээр apperror.Conflict.
func (r *inviteRepository) Create(ctx context.Context, email, invitedBy string) (domain.SuperadminInvite, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO superadmin_invites(email, invited_by) VALUES ($1, $2)
		 RETURNING `+inviteColumns, domain.NormalizeInviteEmail(email), invitedBy)
	if err == nil {
		var rec inviteRecord
		rec, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[inviteRecord])
		if err == nil {
			return rec.toDomain(), nil
		}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.SuperadminInvite{}, apperror.Conflict("this email has already been invited")
	}
	return domain.SuperadminInvite{}, apperror.InternalCause(fmt.Errorf("create superadmin invite: %w", err))
}

// List нь бүх урилгыг шинэ нь эхэндээ буцаана.
func (r *inviteRepository) List(ctx context.Context) ([]domain.SuperadminInvite, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+inviteColumns+` FROM superadmin_invites ORDER BY created_at DESC`)
	if err != nil {
		return nil, apperror.InternalCause(fmt.Errorf("list superadmin invites: %w", err))
	}
	recs, err := pgx.CollectRows(rows, pgx.RowToStructByName[inviteRecord])
	if err != nil {
		return nil, apperror.InternalCause(fmt.Errorf("scan superadmin invites: %w", err))
	}
	out := make([]domain.SuperadminInvite, 0, len(recs))
	for _, rec := range recs {
		out = append(out, rec.toDomain())
	}
	return out, nil
}

// GetByEmail нь и-мэйлээр урилгыг олно; байхгүй бол apperror.NotFound.
func (r *inviteRepository) GetByEmail(ctx context.Context, email string) (domain.SuperadminInvite, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+inviteColumns+` FROM superadmin_invites WHERE email = $1`,
		domain.NormalizeInviteEmail(email))
	if err != nil {
		return domain.SuperadminInvite{}, apperror.InternalCause(fmt.Errorf("get superadmin invite: %w", err))
	}
	rec, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[inviteRecord])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SuperadminInvite{}, apperror.NotFound("superadmin invite not found")
		}
		return domain.SuperadminInvite{}, apperror.InternalCause(fmt.Errorf("scan superadmin invite: %w", err))
	}
	return rec.toDomain(), nil
}

// Delete нь урилгыг цуцална. Байхгүй бол apperror.NotFound.
func (r *inviteRepository) Delete(ctx context.Context, email string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM superadmin_invites WHERE email = $1`,
		domain.NormalizeInviteEmail(email))
	if err != nil {
		return apperror.InternalCause(fmt.Errorf("delete superadmin invite: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("superadmin invite not found")
	}
	return nil
}

// MarkAccepted нь урилгыг ашигласан гэж тэмдэглэнэ (onboarding төгсөхөд).
// accepted_at IS NULL нөхцөл нь дахин тэмдэглэхээс сэргийлнэ; мөр байхгүй /
// аль хэдийн ашигласан бол apperror.NotFound.
func (r *inviteRepository) MarkAccepted(ctx context.Context, email string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE superadmin_invites SET accepted_at = now()
		  WHERE email = $1 AND accepted_at IS NULL`,
		domain.NormalizeInviteEmail(email))
	if err != nil {
		return apperror.InternalCause(fmt.Errorf("mark superadmin invite accepted: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("superadmin invite not found or already accepted")
	}
	return nil
}

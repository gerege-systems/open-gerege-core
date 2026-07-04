// Gerege Template Version 27.0
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

// GetByGoogleSub нь холбогдсон Google account (sub)-аар хэрэглэгчийг хайна.
func (r *postgreUserRepository) GetByGoogleSub(ctx context.Context, sub string) (domain.User, error) {
	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.UserColumns+` FROM users WHERE google_sub = $1 AND deleted_at IS NULL`, sub)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err == nil {
		return stored.ToV1Domain(), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, apperror.NotFound("user not found")
	}
	return domain.User{}, apperror.InternalCause(fmt.Errorf("get user by google_sub: %w", err))
}

// LinkGoogleSub нь userID-тай хэрэглэгчид Google account-ийг холбоно.
func (r *postgreUserRepository) LinkGoogleSub(ctx context.Context, userID, sub string) error {
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, execErr := tx.Exec(ctx,
			`UPDATE users SET google_sub = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
			userID, sub)
		if execErr != nil {
			return execErr
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("user not found")
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if _, ok := err.(*apperror.DomainError); ok {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return apperror.Conflict("this Google account is already linked to another user")
	}
	return apperror.InternalCause(fmt.Errorf("link google_sub: %w", err))
}

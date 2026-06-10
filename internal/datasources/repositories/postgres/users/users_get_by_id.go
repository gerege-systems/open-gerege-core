// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"errors"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
)

func (r *postgreUserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
	const (
		repositoryName = "users"
		funcName       = "GetByID"
		queryName      = "selectUserByID"
		fileName       = "users_get_by_id.go"
	)
	// Soft-delete хийгдсэн мөрүүдийг ИЛ-ээр хас (GORM-ийн автомат scope
	// байхгүй болсон).
	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.UserColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
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
	logger.ErrorWithContext(ctx, "Failed to query user by id", logger.Fields{
		"repository": repositoryName,
		"method":     funcName,
		"query":      queryName,
		"file":       fileName,
		"error":      err.Error(),
		"table":      "users",
		"user_id":    id,
	})
	return domain.User{}, err
}

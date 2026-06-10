// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"errors"

	"template/internal/apperror"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *postgreUserRepository) UpdateRole(ctx context.Context, id string, roleID int) error {
	const (
		repositoryName = "users"
		funcName       = "UpdateRole"
		queryName      = "updateUserRole"
		fileName       = "users_update_role.go"
	)
	// `deleted_at IS NULL` нь UPDATE-г амьд мөрүүдээр хязгаарлана. role_id нь
	// roles(id)-руу FK тул байхгүй role оноох оролдлого 23503 болж гарна.
	var tag pgconn.CommandTag
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		var execErr error
		tag, execErr = tx.Exec(ctx,
			`UPDATE users SET role_id = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL`,
			roleID, id)
		return execErr
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperror.BadRequest("unknown role")
		}
		logger.ErrorWithContext(ctx, "Failed to update user role", logger.Fields{
			"repository": repositoryName, "method": funcName, "query": queryName,
			"file": fileName, "error": err.Error(), "table": "users", "user_id": id, "role_id": roleID,
		})
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("user not found")
	}
	return nil
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"
)

func (r *postgreUserRepository) UpdatePassword(ctx context.Context, inDom *domain.User) error {
	const (
		repositoryName = "users"
		funcName       = "UpdatePassword"
		queryName      = "updateUserPassword"
		fileName       = "users_update_password.go"
	)
	userRecord := records.FromUsersV1Domain(inDom)
	// `deleted_at IS NULL` нь UPDATE-г амьд мөрүүдээр хязгаарлана.
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET password = $1, password_changed_at = $2, updated_at = $3 WHERE id = $4 AND deleted_at IS NULL`,
		userRecord.Password, userRecord.PasswordChangedAt, userRecord.UpdatedAt, userRecord.Id)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to update user password", logger.Fields{
			"repository": repositoryName, "method": funcName, "query": queryName,
			"file": fileName, "error": err.Error(), "table": "users", "user_id": userRecord.Id,
		})
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("user not found")
	}
	return nil
}

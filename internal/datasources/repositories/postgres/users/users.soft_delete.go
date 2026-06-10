// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"time"

	"template/internal/apperror"
	"template/pkg/logger"
)

func (r *postgreUserRepository) SoftDelete(ctx context.Context, id string) error {
	const (
		repositoryName = "users"
		funcName       = "SoftDelete"
		queryName      = "softDeleteUser"
		fileName       = "users.soft_delete.go"
	)
	// Амьд мөр дээрх `deleted_at IS NULL` Where нь үйлдлийг idempotent
	// байлгана — аль хэдийн устгагдсан мөрийг алгасч, RowsAffected == 0
	// гарна.
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL`,
		now, id)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to soft-delete user", logger.Fields{
			"repository": repositoryName, "method": funcName, "query": queryName,
			"file": fileName, "error": err.Error(), "table": "users", "user_id": id,
		})
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("user not found")
	}
	return nil
}

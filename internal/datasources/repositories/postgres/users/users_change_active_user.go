// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"time"

	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
)

func (r *postgreUserRepository) ChangeActiveUser(ctx context.Context, inDom *domain.User) (err error) {
	const (
		repositoryName = "users"
		funcName       = "ChangeActiveUser"
		queryName      = "updateUserActive"
		fileName       = "users_change_active_user.go"
	)
	userRecord := records.FromUsersV1Domain(inDom)

	// `deleted_at IS NULL` нь UPDATE-г амьд мөрүүдээр хязгаарлана.
	err = r.withRLS(ctx, func(tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx,
			`UPDATE users SET active = $1, updated_at = $2 WHERE id = $3 AND deleted_at IS NULL`,
			userRecord.Active, time.Now().UTC(), userRecord.Id)
		return execErr
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to update user active flag", logger.Fields{
			"repository": repositoryName, "method": funcName, "query": queryName,
			"file": fileName, "error": err.Error(), "table": "users", "user_id": userRecord.Id,
		})
	}
	return
}

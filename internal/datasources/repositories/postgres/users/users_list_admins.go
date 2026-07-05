// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"

	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
)

// ListAdmins нь админ түвшний бүх бүртгэлийг (super admin + admin) буцаана.
// role_id өсөх дарааллаар (super admin эхэнд), дараа нь шинээр үүсгэснээр
// эрэмбэлнэ. Утгууд нь $N parameter-ээр холбогддог — хэзээ ч SQL мөр рүү шууд
// залгагддаггүй.
func (r *postgreUserRepository) ListAdmins(ctx context.Context) ([]domain.User, error) {
	const (
		repositoryName = "users"
		funcName       = "ListAdmins"
		queryName      = "selectAdmins"
		fileName       = "users_list_admins.go"
	)

	sql := `SELECT ` + records.UserColumns + `
		FROM users
		WHERE role_id IN ($1, $2) AND deleted_at IS NULL
		ORDER BY role_id ASC, created_at DESC`

	var stored []records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, sql, domain.RoleSuperAdmin, domain.RoleAdmin)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectRows(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to list admins", logger.Fields{
			"repository": repositoryName, "method": funcName, "query": queryName,
			"file": fileName, "error": err.Error(), "table": "users",
		})
		return nil, err
	}
	out := make([]domain.User, 0, len(stored))
	for i := range stored {
		out = append(out, stored[i].ToV1Domain())
	}
	return out, nil
}

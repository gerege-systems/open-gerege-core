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

func (r *postgreUserRepository) GetByEmail(ctx context.Context, inDom *domain.User) (outDomain domain.User, err error) {
	const (
		repositoryName = "users"
		funcName       = "GetByEmail"
		queryName      = "selectUserByEmail"
		fileName       = "users_get_by_email.go"
	)
	userRecord := records.FromUsersV1Domain(inDom)

	// Soft-delete хийгдсэн мөрүүдийг ИЛ-ээр хас — "устгагдсан" хэрэглэгчид
	// audit/сэргээх зорилгоор хадгалагдах боловч нэвтрэх/OTP урсгалуудыг
	// хангах ёсгүй.
	rows, qerr := r.pool.Query(ctx,
		`SELECT `+records.UserColumns+` FROM users WHERE email = $1 AND deleted_at IS NULL`, userRecord.Email)
	if qerr == nil {
		var stored records.Users
		stored, qerr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		if qerr == nil {
			return stored.ToV1Domain(), nil
		}
	}
	if errors.Is(qerr, pgx.ErrNoRows) {
		return domain.User{}, apperror.NotFound("user not found")
	}
	logger.ErrorWithContext(ctx, "Failed to query user by email", logger.Fields{
		"repository": repositoryName,
		"method":     funcName,
		"query":      queryName,
		"file":       fileName,
		"error":      qerr.Error(),
		"table":      "users",
		"email":      userRecord.Email,
	})
	return domain.User{}, qerr
}

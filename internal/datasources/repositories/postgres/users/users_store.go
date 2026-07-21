// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *postgreUserRepository) Store(ctx context.Context, inDom *domain.User) (domain.User, error) {
	const (
		repositoryName = "users"
		funcName       = "Store"
		queryName      = "insertUser"
		fileName       = "users_store.go"
	)
	userRecord := records.FromUsersV1Domain(inDom)

	// INSERT ... RETURNING * — ингэснээр дуудагч хадгалагдсан мөрийг нэг
	// round-trip-д авна. id нь uuid_generate_v4() баганын өгөгдмөл утгаар
	// (SQL migration-уудаар бэлтгэгдсэн) сервер талд үүсгэгддэг.
	var stored records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			INSERT INTO users(id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, $6, $7, false, $8, $9)
			RETURNING `+records.UserColumns+`
		`, userRecord.Username, userRecord.FirstName, userRecord.LastName, userRecord.FirstNameEn, userRecord.LastNameEn, userRecord.Email, userRecord.Password, userRecord.RoleId, userRecord.CreatedAt)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err == nil {
		if stored.Id == "" {
			e := fmt.Errorf("insert succeeded but RETURNING produced no row")
			logger.ErrorWithContext(ctx, "Insert returned no row", logger.Fields{
				"repository": repositoryName, "method": funcName, "query": queryName,
				"file": fileName, "error": e.Error(), "table": "users",
			})
			return domain.User{}, e
		}
		return stored.ToV1Domain(), nil
	}

	// 23505 unique_violation-г 409 Conflict болгон буулгана.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		logger.ErrorWithContext(ctx, "Failed to insert user: unique violation", logger.Fields{
			"repository": repositoryName, "method": funcName, "query": queryName,
			"file": fileName, "error": err.Error(), "table": "users", "email": inDom.Email,
		})
		return domain.User{}, apperror.Conflict("username or email already exists")
	}
	logger.ErrorWithContext(ctx, "Failed to insert user into database", logger.Fields{
		"repository": repositoryName, "method": funcName, "query": queryName,
		"file": fileName, "error": err.Error(), "table": "users",
	})
	return domain.User{}, err
}

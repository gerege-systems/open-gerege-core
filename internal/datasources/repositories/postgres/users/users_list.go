// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"fmt"

	"template/internal/business/domain"
	"template/internal/datasources/records"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
)

// hardLimit нь List хуудасны хэмжээг хязгаарладаг тул буруу ажиллаж
// буй дуудагч бүх хүснэгтийг нэг хүсэлтэд татаж чадахгүй.
const hardLimit = 200

func (r *postgreUserRepository) List(ctx context.Context, filter repointerface.UserListFilter, offset, limit int) ([]domain.User, error) {
	const (
		repositoryName = "users"
		funcName       = "List"
		queryName      = "selectUsersList"
		fileName       = "users_list.go"
	)
	if limit <= 0 || limit > hardLimit {
		limit = hardLimit
	}
	if offset < 0 {
		offset = 0
	}

	// Query-г динамикаар бүтээ — утга бүр $N parameter болж холбогддог,
	// хэзээ ч SQL мөр рүү залгагддаггүй.
	sql := `SELECT ` + records.UserColumns + ` FROM users WHERE 1=1`
	args := make([]any, 0, 4)
	n := 1
	if !filter.IncludeDeleted {
		sql += ` AND deleted_at IS NULL`
	}
	if filter.RoleID != 0 {
		sql += fmt.Sprintf(` AND role_id = $%d`, n)
		args = append(args, filter.RoleID)
		n++
	}
	if filter.ActiveOnly {
		sql += fmt.Sprintf(` AND active = $%d`, n)
		args = append(args, true)
		n++
	}
	sql += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, n, n+1)
	args = append(args, limit, offset)

	var stored []records.Users
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, sql, args...)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectRows(rows, pgx.RowToStructByName[records.Users])
		return scanErr
	})
	if err == nil {
		out := make([]domain.User, 0, len(stored))
		for i := range stored {
			out = append(out, stored[i].ToV1Domain())
		}
		return out, nil
	}
	logger.ErrorWithContext(ctx, "Failed to list users", logger.Fields{
		"repository": repositoryName, "method": funcName, "query": queryName,
		"file": fileName, "error": err.Error(), "table": "users", "limit": limit, "offset": offset,
	})
	return nil, err
}

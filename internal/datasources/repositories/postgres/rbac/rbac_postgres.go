// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package rbac нь roles / permissions / role_permissions хүснэгтүүдийн Postgres
// gateway юм. Эдгээр нь хэрэглэгч-тус-бүрийн биш лавлах өгөгдөл тул Row-Level
// Security-д хамаарахгүй (plain pool query). Цорын ганц онцгой тохиолдол нь
// CountUsersWithRole — RLS-тэй users хүснэгтэд хүрдэг тул "service" GUC-ийг
// транзакцид тогтооно.
package rbac

import (
	"context"
	"errors"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgUniqueViolation = "23505"

type rbacRepository struct {
	pool *pgxpool.Pool
}

func NewRBACRepository(pool *pgxpool.Pool) repointerface.RBACRepository {
	return &rbacRepository{pool: pool}
}

const roleColumns = `id, key, name, description, is_system, created_at, updated_at`

func scanRole(row pgx.Row) (domain.Role, error) {
	var r domain.Role
	err := row.Scan(&r.ID, &r.Key, &r.Name, &r.Description, &r.IsSystem, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func (r *rbacRepository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+roleColumns+` FROM roles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Role, 0, 8)
	for rows.Next() {
		role, scanErr := scanRole(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, role)
	}
	return out, rows.Err()
}

func (r *rbacRepository) GetRole(ctx context.Context, id int) (domain.Role, error) {
	role, err := scanRole(r.pool.QueryRow(ctx, `SELECT `+roleColumns+` FROM roles WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Role{}, apperror.NotFound("role not found")
	}
	return role, err
}

func (r *rbacRepository) CreateRole(ctx context.Context, in *domain.Role) (domain.Role, error) {
	role, err := scanRole(r.pool.QueryRow(ctx,
		`INSERT INTO roles(key, name, description, is_system) VALUES ($1,$2,$3,false) RETURNING `+roleColumns,
		in.Key, in.Name, in.Description))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.Role{}, apperror.Conflict("role key already exists")
	}
	return role, err
}

func (r *rbacRepository) UpdateRole(ctx context.Context, in *domain.Role) (domain.Role, error) {
	// Зөвхөн name/description-г шинэчилнэ; key болон is_system өөрчлөгдөхгүй.
	role, err := scanRole(r.pool.QueryRow(ctx,
		`UPDATE roles SET name = $2, description = $3, updated_at = now() WHERE id = $1 RETURNING `+roleColumns,
		in.ID, in.Name, in.Description))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Role{}, apperror.NotFound("role not found")
	}
	return role, err
}

func (r *rbacRepository) DeleteRole(ctx context.Context, id int) error {
	// Системийн эрхийг (admin/manager/user) устгуулахгүй.
	tag, err := r.pool.Exec(ctx, `DELETE FROM roles WHERE id = $1 AND is_system = false`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.Conflict("role not found or is a system role")
	}
	return nil
}

func (r *rbacRepository) CountUsersWithRole(ctx context.Context, roleID int) (int, error) {
	// users нь RLS-тэй тул "service" GUC-тэй транзакцид тоолно (эс бөгөөс
	// app role 0 хардаг — ашиглагдаж буй role-ийг алдаатай устгуулна).
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `SELECT set_config('app.user_role','service',true)`); err != nil {
		return 0, err
	}
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE role_id = $1 AND deleted_at IS NULL`, roleID).Scan(&n); err != nil {
		return 0, err
	}
	return n, tx.Commit(ctx)
}

func (r *rbacRepository) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, label, category FROM permissions ORDER BY category, key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Permission, 0, 8)
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.Key, &p.Label, &p.Category); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *rbacRepository) GetRolePermissions(ctx context.Context, roleID int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT permission_key FROM role_permissions WHERE role_id = $1 ORDER BY permission_key`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 8)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// SetRolePermissions нь role-ийн эрхийг бүхэлд нь солино (replace) — нэг
// транзакцид хуучныг устгаж, шинийг оруулна. Зөвхөн каталогт байгаа түлхүүрийг
// зөвшөөрнө (FK permissions.key баталгаажуулна).
func (r *rbacRepository) SetRolePermissions(ctx context.Context, roleID int, keys []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	for _, k := range keys {
		if _, err := tx.Exec(ctx,
			`INSERT INTO role_permissions(role_id, permission_key) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
			roleID, k); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
				return apperror.BadRequest("unknown permission key: " + k)
			}
			return err
		}
	}
	return tx.Commit(ctx)
}

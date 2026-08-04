// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package platformmodules нь platform_modules (модулийн идэвхийн төлөв)
// хүснэгтийн Postgres gateway. Нийтийн config тул RLS-гүй (plain pool query);
// бичих эрх нь route давхаргад RequireAdmin-аар хамгаалагдана.
package platformmodules

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository нь platform_modules repo үүсгэнэ.
func NewRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

// ListDisabled нь унтраалттай гэж хадгалагдсан модулийн ID-нуудыг буцаана.
// (Мөргүй буюу enabled=true модулиуд default идэвхтэй тул сонирхолгүй.)
func (r *repository) ListDisabled(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM platform_modules WHERE enabled = false ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query platform modules: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("scan platform modules: %w", err)
	}
	return ids, nil
}

// SetEnabled нь модулийн төлвийг upsert хийнэ.
func (r *repository) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO platform_modules (id, enabled, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now()`,
		id, enabled)
	if err != nil {
		return fmt.Errorf("upsert platform module %s: %w", id, err)
	}
	return nil
}

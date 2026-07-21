// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package theme нь themes хүснэгтийн (landing-ийн нэрлэсэн загварууд) Postgres
// gateway юм. Хэрэглэгч-тус-бүрийн биш нийтийн config тул Row-Level Security-д
// хамаарахгүй (plain pool query).
package theme

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

type themeRepository struct {
	pool *pgxpool.Pool
}

func NewThemeRepository(pool *pgxpool.Pool) repointerface.ThemeRepository {
	return &themeRepository{pool: pool}
}

const themeCols = `id, name, config, is_active, created_at, updated_at`

func (r *themeRepository) ListThemes(ctx context.Context) ([]domain.Theme, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+themeCols+` FROM themes ORDER BY is_active DESC, created_at`)
	if err != nil {
		return nil, fmt.Errorf("list themes: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Theme])
	if err != nil {
		return nil, fmt.Errorf("scan themes: %w", err)
	}
	return list, nil
}

func (r *themeRepository) GetTheme(ctx context.Context, id string) (domain.Theme, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+themeCols+` FROM themes WHERE id = $1`, id)
	if err != nil {
		return domain.Theme{}, fmt.Errorf("query theme: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Theme])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Theme{}, apperror.NotFound("theme not found")
		}
		return domain.Theme{}, fmt.Errorf("scan theme: %w", err)
	}
	return t, nil
}

func (r *themeRepository) GetActiveTheme(ctx context.Context) (domain.Theme, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+themeCols+` FROM themes WHERE is_active LIMIT 1`)
	if err != nil {
		return domain.Theme{}, fmt.Errorf("query active theme: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Theme])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Theme{}, apperror.NotFound("no active theme")
		}
		return domain.Theme{}, fmt.Errorf("scan active theme: %w", err)
	}
	return t, nil
}

func (r *themeRepository) CreateTheme(ctx context.Context, name string, config json.RawMessage) (domain.Theme, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO themes (name, config) VALUES ($1, $2) RETURNING `+themeCols,
		name, []byte(config))
	if err != nil {
		return domain.Theme{}, fmt.Errorf("insert theme: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Theme])
	if err != nil {
		return domain.Theme{}, fmt.Errorf("scan created theme: %w", err)
	}
	return t, nil
}

func (r *themeRepository) UpdateTheme(ctx context.Context, id, name string, config json.RawMessage) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE themes SET name = $2, config = $3, updated_at = now() WHERE id = $1`,
		id, name, []byte(config))
	if err != nil {
		return fmt.Errorf("update theme: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("theme not found")
	}
	return nil
}

func (r *themeRepository) DeleteTheme(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM themes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete theme: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("theme not found")
	}
	return nil
}

// SetActive нь нэг theme-ийг идэвхтэй болгож бусдыг идэвхгүй болгоно. Partial
// unique index-ийн улмаас алхмыг нэг tx-д хийж, эхлээд бусдыг унтраана.
func (r *themeRepository) SetActive(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set-active tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit амжвал no-op

	if _, err := tx.Exec(ctx, `UPDATE themes SET is_active = false WHERE is_active`); err != nil {
		return fmt.Errorf("clear active themes: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE themes SET is_active = true, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("set active theme: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("theme not found")
	}
	return tx.Commit(ctx)
}

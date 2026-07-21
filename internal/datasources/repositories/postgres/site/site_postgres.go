// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package site нь site_appearance (сайтын нийтийн харагдацын default) ганц
// мөрийн Postgres gateway юм. Хэрэглэгч-тус-бүрийн биш нийтийн config тул
// Row-Level Security-д хамаарахгүй (plain pool query).
package site

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

type siteRepository struct {
	pool *pgxpool.Pool
}

func NewSiteRepository(pool *pgxpool.Pool) repointerface.SiteRepository {
	return &siteRepository{pool: pool}
}

func (r *siteRepository) GetAppearance(ctx context.Context) (domain.SiteAppearance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT accent, font, style, theme, updated_at FROM site_appearance WHERE id = 1`)
	if err != nil {
		return domain.SiteAppearance{}, fmt.Errorf("query site appearance: %w", err)
	}
	a, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.SiteAppearance])
	if err != nil {
		return domain.SiteAppearance{}, fmt.Errorf("scan site appearance: %w", err)
	}
	return a, nil
}

func (r *siteRepository) SetAppearance(ctx context.Context, a domain.SiteAppearance) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE site_appearance SET accent = $1, font = $2, style = $3, theme = $4, updated_at = now() WHERE id = 1`,
		a.Accent, a.Font, a.Style, a.Theme)
	if err != nil {
		return fmt.Errorf("update site appearance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Seed мөр байхгүй — migration ажиллаагүй эсэхийг илтгэнэ.
		return apperror.NotFound("site appearance row not found")
	}
	return nil
}

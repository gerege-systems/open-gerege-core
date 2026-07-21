// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package platformsettings нь platform_settings (платформын хандалтын горим)
// ганц мөрийн Postgres gateway. Хэрэглэгч-тус-бүрийн биш нийтийн config тул
// Row-Level Security-д хамаарахгүй (plain pool query). access_mode нь
// 'public' эсвэл 'private'.
package platformsettings

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
)

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository нь platform_settings repo үүсгэнэ.
func NewRepository(pool *pgxpool.Pool) *repository {
	return &repository{pool: pool}
}

// GetAccessMode нь одоогийн хандалтын горимыг ('public'|'private') буцаана.
func (r *repository) GetAccessMode(ctx context.Context) (string, error) {
	rows, err := r.pool.Query(ctx, `SELECT access_mode FROM platform_settings WHERE id = 1`)
	if err != nil {
		return "", fmt.Errorf("query platform settings: %w", err)
	}
	mode, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[string])
	if err != nil {
		return "", fmt.Errorf("scan platform settings: %w", err)
	}
	return mode, nil
}

// SetAccessMode нь хандалтын горимыг шинэчилнэ. Зөвхөн 'public'|'private'.
func (r *repository) SetAccessMode(ctx context.Context, mode string) error {
	if mode != domain.AccessModePublic && mode != domain.AccessModePrivate {
		return apperror.BadRequest("access_mode нь 'public' эсвэл 'private' байх ёстой")
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE platform_settings SET access_mode = $1, updated_at = now() WHERE id = 1`, mode)
	if err != nil {
		return fmt.Errorf("update platform settings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("platform_settings row not found")
	}
	return nil
}

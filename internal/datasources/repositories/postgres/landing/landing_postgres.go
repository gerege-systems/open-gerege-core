// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package landing нь landing_config хүснэгтийн (нүүр хуудасны тохируулдаг
// харагдац) Postgres gateway юм. Хэрэглэгч-тус-бүрийн биш глобал тохиргоо тул
// Row-Level Security-д хамаарахгүй (plain pool query). Ганц мөр (id=1) байх ба
// SetConfig зөвхөн UPDATE хийдэг.
package landing

import (
	"context"
	"encoding/json"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"

	"github.com/jackc/pgx/v5/pgxpool"
)

type landingRepository struct {
	pool *pgxpool.Pool
}

func NewLandingConfigRepository(pool *pgxpool.Pool) repointerface.LandingConfigRepository {
	return &landingRepository{pool: pool}
}

func (r *landingRepository) GetConfig(ctx context.Context) (domain.LandingConfig, error) {
	var out domain.LandingConfig
	err := r.pool.QueryRow(ctx,
		`SELECT config, updated_at FROM landing_config WHERE id = 1`).
		Scan(&out.Config, &out.UpdatedAt)
	if err != nil {
		// pgx нь мөр байхгүй үед pgx.ErrNoRows буцаана; seed хийгдээгүй DB
		// (migration ажиллаагүй) гэсэн үг тул NotFound болгож usecase-д
		// fail-open (өгөгдмөл рүү) шийдүүлнэ.
		return domain.LandingConfig{}, apperror.NotFound("landing config not found")
	}
	return out, nil
}

// SetConfig нь ганц мөрийн (id=1) config-ийг бүхэлд нь солино. Мөр байхгүй бол
// apperror.NotFound (шинэ мөр INSERT хийхгүй — гадаргууг хаалттай байлгана).
func (r *landingRepository) SetConfig(ctx context.Context, config json.RawMessage) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE landing_config SET config = $1, updated_at = now() WHERE id = 1`, []byte(config))
	if err != nil {
		return fmt.Errorf("set landing config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("landing config not found")
	}
	return nil
}

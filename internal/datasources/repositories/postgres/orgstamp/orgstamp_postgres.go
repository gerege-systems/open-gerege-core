// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package orgstamp нь org_stamps хүснэгтийн Postgres gateway — байгууллагын
// тамганы дардасын зургийн URL (Google Drive). Per-org тул RLS-гүй; эрхийн
// шалгалт usecase давхаргад.
package orgstamp

import (
	"context"
	"errors"

	"template/internal/apperror"
	repointerface "template/internal/datasources/repositories/interface"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewOrgStampRepository(pool *pgxpool.Pool) repointerface.OrgStampRepository {
	return &repository{pool: pool}
}

func (r *repository) Get(ctx context.Context, orgRegister string) (string, error) {
	var url string
	err := r.pool.QueryRow(ctx, `SELECT image FROM org_stamps WHERE org_register = $1`, orgRegister).Scan(&url)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", apperror.InternalCause(err)
	}
	return url, nil
}

func (r *repository) Upsert(ctx context.Context, orgRegister, url, uploadedBy string) error {
	var by any
	if uploadedBy != "" {
		by = uploadedBy
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO org_stamps (org_register, image, uploaded_by, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (org_register) DO UPDATE SET image = EXCLUDED.image, uploaded_by = EXCLUDED.uploaded_by, updated_at = now()`,
		orgRegister, url, by)
	if err != nil {
		return apperror.InternalCause(err)
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, orgRegister string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM org_stamps WHERE org_register = $1`, orgRegister)
	if err != nil {
		return apperror.InternalCause(err)
	}
	return nil
}

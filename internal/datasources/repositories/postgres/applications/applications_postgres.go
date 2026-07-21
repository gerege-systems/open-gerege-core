// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package applications нь нэгдсэн Applications overlay-ийн Postgres gateway —
// applications мөр + зөвшөөрсөн gateway service-үүд (application_services).
// OAuth2 client өөрөө oauth_clients-д амьдардаг тул энд зөвхөн client_id + overlay.
// Хэрэглэгч-тус-бүрийн биш config өгөгдөл тул RLS-гүй, plain pool query.
package applications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

type applicationRepository struct {
	pool *pgxpool.Pool
}

func NewApplicationRepository(pool *pgxpool.Pool) repointerface.ApplicationRepository {
	return &applicationRepository{pool: pool}
}

// appSelect нь overlay + зөвшөөрсөн service id-уудыг (array_agg) уншина.
const appSelect = `
	SELECT a.id, a.client_id, a.name, a.app_type, a.tags, a.redirect_uris, a.enabled, a.created_by,
	       COALESCE(array_agg(s.service_id::text) FILTER (WHERE s.service_id IS NOT NULL), '{}') AS service_ids,
	       a.created_at, a.updated_at
	FROM applications a
	LEFT JOIN application_services s ON s.application_id = a.id`

func scanApp(row pgx.Row) (domain.Application, error) {
	var a domain.Application
	err := row.Scan(&a.ID, &a.ClientID, &a.Name, &a.AppType, &a.Tags, &a.RedirectURIs,
		&a.Enabled, &a.CreatedBy, &a.ServiceIDs, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func (r *applicationRepository) List(ctx context.Context) ([]domain.Application, error) {
	rows, err := r.pool.Query(ctx, appSelect+` GROUP BY a.id ORDER BY a.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query applications: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Application, 0)
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *applicationRepository) Get(ctx context.Context, id string) (domain.Application, error) {
	row := r.pool.QueryRow(ctx, appSelect+` WHERE a.id = $1 GROUP BY a.id`, id)
	a, err := scanApp(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Application{}, apperror.NotFound("application not found")
	}
	if err != nil {
		return domain.Application{}, mapErr(err)
	}
	return a, nil
}

func (r *applicationRepository) Create(ctx context.Context, a *domain.Application) (domain.Application, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Application{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO applications (client_id, name, app_type, tags, redirect_uris, enabled, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		a.ClientID, a.Name, a.AppType, arr(a.Tags), arr(a.RedirectURIs), a.Enabled, a.CreatedBy).Scan(&id)
	if err != nil {
		return domain.Application{}, mapErr(err)
	}
	if err := insertServices(ctx, tx, id, a.ServiceIDs); err != nil {
		return domain.Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Application{}, fmt.Errorf("commit: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *applicationRepository) Update(ctx context.Context, a *domain.Application) (domain.Application, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Application{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE applications SET name=$2, app_type=$3, tags=$4, redirect_uris=$5, enabled=$6, updated_at=now()
		 WHERE id=$1`,
		a.ID, a.Name, a.AppType, arr(a.Tags), arr(a.RedirectURIs), a.Enabled)
	if err != nil {
		return domain.Application{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Application{}, apperror.NotFound("application not found")
	}
	if err := replaceServices(ctx, tx, a.ID, a.ServiceIDs); err != nil {
		return domain.Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Application{}, fmt.Errorf("commit: %w", err)
	}
	return r.Get(ctx, a.ID)
}

func (r *applicationRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM applications WHERE id=$1`, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("application not found")
	}
	return nil
}

func (r *applicationRepository) SetServices(ctx context.Context, appID string, serviceIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := replaceServices(ctx, tx, appID, serviceIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *applicationRepository) ServiceScopes(ctx context.Context, serviceIDs []string) ([]string, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT scope FROM gateway_services WHERE id = ANY($1::uuid[]) AND scope <> ''`, serviceIDs)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ServiceIDsForScopes нь өгсөн OAuth scope нэрсэд харгалзах gateway service
// id-уудыг буцаана (ServiceScopes-ийн урвуу). Client-ийн scope-оос апп-ын
// зөвшөөрсөн service-үүдийг сэргээхэд ашиглана.
func (r *applicationRepository) ServiceIDsForScopes(ctx context.Context, scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id::text FROM gateway_services WHERE scope = ANY($1) AND scope <> ''`, scopes)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// replaceServices нь апп-ын service grant-уудыг бүхэлд нь орлуулна (tx дотор).
func replaceServices(ctx context.Context, tx pgx.Tx, appID string, serviceIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM application_services WHERE application_id=$1`, appID); err != nil {
		return mapErr(err)
	}
	return insertServices(ctx, tx, appID, serviceIDs)
}

func insertServices(ctx context.Context, tx pgx.Tx, appID string, serviceIDs []string) error {
	for _, sid := range serviceIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO application_services (application_id, service_id) VALUES ($1, $2::uuid)
			 ON CONFLICT DO NOTHING`, appID, sid); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// arr нь nil slice-ыг хоосон болгоно (text[] NOT NULL багана NULL-ыг зөвшөөрдөггүй).
func arr(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// mapErr нь Postgres алдааг apperror болгоно.
func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation (client_id давхцав)
			return apperror.Conflict("application client_id already exists")
		case "23503": // foreign_key_violation (service байхгүй)
			return apperror.BadRequest("unknown service id")
		case "22P02": // invalid_text_representation (буруу uuid)
			return apperror.BadRequest("invalid id format")
		}
	}
	return apperror.InternalCause(err)
}

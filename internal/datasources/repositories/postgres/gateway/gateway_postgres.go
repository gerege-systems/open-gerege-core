// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gateway нь API Gateway-ийн тохиргоо/телеметр хүснэгтүүдийн Postgres
// gateway юм (services/routes/consumers/api keys/policies/request logs). Эдгээр
// нь хэрэглэгч-тус-бүрийн биш лавлах/тохиргооны өгөгдөл тул Row-Level Security-д
// хамаарахгүй — rbac адаптертай ижил, plain pool query ашиглана.
package gateway

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

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

type gatewayRepository struct {
	pool *pgxpool.Pool
}

func NewGatewayRepository(pool *pgxpool.Pool) repointerface.GatewayRepository {
	return &gatewayRepository{pool: pool}
}

// mapWrite нь бичих үйлдлийн (INSERT/UPDATE) pg алдааг домэйн apperror руу буулгана.
func mapWrite(err error, conflictMsg string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return apperror.Conflict(conflictMsg)
		case pgForeignKeyViolation:
			return apperror.BadRequest("referenced record does not exist")
		}
	}
	return err
}

// ── Services ────────────────────────────────────────────────────────────────

const serviceColumns = `id, name, protocol, host, port, path, retries, connect_timeout_ms, tags, enabled, created_at, updated_at`

func scanService(row pgx.Row) (domain.GatewayService, error) {
	var s domain.GatewayService
	err := row.Scan(&s.ID, &s.Name, &s.Protocol, &s.Host, &s.Port, &s.Path,
		&s.Retries, &s.ConnectTimeout, &s.Tags, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (r *gatewayRepository) ListServices(ctx context.Context) ([]domain.GatewayService, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serviceColumns+` FROM gateway_services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayService, 0, 16)
	for rows.Next() {
		s, scanErr := scanService(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *gatewayRepository) GetService(ctx context.Context, id string) (domain.GatewayService, error) {
	s, err := scanService(r.pool.QueryRow(ctx, `SELECT `+serviceColumns+` FROM gateway_services WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GatewayService{}, apperror.NotFound("service not found")
	}
	return s, err
}

func (r *gatewayRepository) CreateService(ctx context.Context, in *domain.GatewayService) (domain.GatewayService, error) {
	// scope-ыг нэрээс автоматаар гаргана ('svc:'||name) — ингэснээр UI-аар үүсгэсэн
	// service-ийг ч application-д оноож (OAuth scope болгож) болно.
	s, err := scanService(r.pool.QueryRow(ctx,
		`INSERT INTO gateway_services(name, protocol, host, port, path, retries, connect_timeout_ms, tags, enabled, scope)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'svc:'||$1) RETURNING `+serviceColumns,
		in.Name, in.Protocol, in.Host, in.Port, in.Path, in.Retries, in.ConnectTimeout, in.Tags, in.Enabled))
	if err != nil {
		return domain.GatewayService{}, mapWrite(err, "service name already exists")
	}
	return s, nil
}

func (r *gatewayRepository) UpdateService(ctx context.Context, in *domain.GatewayService) (domain.GatewayService, error) {
	s, err := scanService(r.pool.QueryRow(ctx,
		`UPDATE gateway_services SET name=$2, protocol=$3, host=$4, port=$5, path=$6, retries=$7,
		 connect_timeout_ms=$8, tags=$9, enabled=$10, updated_at=now() WHERE id=$1 RETURNING `+serviceColumns,
		in.ID, in.Name, in.Protocol, in.Host, in.Port, in.Path, in.Retries, in.ConnectTimeout, in.Tags, in.Enabled))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GatewayService{}, apperror.NotFound("service not found")
	}
	if err != nil {
		return domain.GatewayService{}, mapWrite(err, "service name already exists")
	}
	return s, nil
}

func (r *gatewayRepository) DeleteService(ctx context.Context, id string) error {
	return r.execDelete(ctx, `DELETE FROM gateway_services WHERE id = $1`, id, "service not found")
}

// ── Telemetry ────────────────────────────────────────────────────────────—

func (r *gatewayRepository) ListRequestLogs(ctx context.Context, limit int) ([]domain.GatewayRequestLog, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, method, path, status, latency_ms, client_ip, created_at
		 FROM gateway_request_logs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayRequestLog, 0, limit)
	for rows.Next() {
		var l domain.GatewayRequestLog
		if err := rows.Scan(&l.ID, &l.Method, &l.Path, &l.Status, &l.LatencyMS, &l.ClientIP, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CreateRequestLog нь бодит /api хүсэлтийг лог-д бичнэ (middleware-ээс).
func (r *gatewayRepository) CreateRequestLog(ctx context.Context, l *domain.GatewayRequestLog) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO gateway_request_logs (method, path, status, latency_ms, client_ip)
		 VALUES ($1,$2,$3,$4,$5)`,
		l.Method, l.Path, l.Status, l.LatencyMS, l.ClientIP)
	return err
}

// Overview нь dashboard-ийн нэгтгэлийг тооцоолно. Тоологдох утгууд (services/
// applications/эрх) нь бүх хугацааных; харин request телеметр нь сүүлийн 24
// цагийнх. Хувь/p95-ийг нэг query-д percentile_cont-оор гаргана.
func (r *gatewayRepository) Overview(ctx context.Context) (domain.GatewayOverview, error) {
	var o domain.GatewayOverview
	if err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM gateway_services),
			(SELECT count(*) FROM applications),
			(SELECT count(*) FROM application_services),
			COALESCE(count(*),0),
			COALESCE(count(*) FILTER (WHERE status >= 500),0),
			COALESCE(count(*) FILTER (WHERE status = 429),0),
			COALESCE(avg(latency_ms),0)::int,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms),0)::int
		FROM gateway_request_logs WHERE created_at >= now() - interval '24 hours'`,
	).Scan(&o.Services, &o.Consumers, &o.ActiveKeys,
		&o.Requests24h, &o.Errors24h, &o.RateLimited24h, &o.AvgLatencyMS, &o.P95LatencyMS); err != nil {
		return domain.GatewayOverview{}, err
	}
	if o.Requests24h > 0 {
		o.ErrorRate = float64(o.Errors24h) / float64(o.Requests24h)
	}

	// Статус ангиллын хуваарилалт (2xx..5xx).
	buckets, err := r.statusBuckets(ctx)
	if err != nil {
		return domain.GatewayOverview{}, err
	}
	o.StatusBuckets = buckets

	// Хамгийн их хүсэлттэй замууд (хүсэлтийн тоогоор).
	top, err := r.topPaths(ctx)
	if err != nil {
		return domain.GatewayOverview{}, err
	}
	o.TopPaths = top
	return o, nil
}

func (r *gatewayRepository) statusBuckets(ctx context.Context) ([]domain.GatewayStatusBucket, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT (status/100)::text || 'xx' AS class, count(*)
		FROM gateway_request_logs WHERE created_at >= now() - interval '24 hours'
		GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayStatusBucket, 0, 4)
	for rows.Next() {
		var b domain.GatewayStatusBucket
		if err := rows.Scan(&b.Class, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *gatewayRepository) topPaths(ctx context.Context) ([]domain.GatewayPathStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT path, count(*) AS n
		FROM gateway_request_logs
		WHERE created_at >= now() - interval '24 hours'
		GROUP BY path ORDER BY n DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayPathStat, 0, 5)
	for rows.Next() {
		var s domain.GatewayPathStat
		if err := rows.Scan(&s.Path, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// execDelete нь нэг мөрийн DELETE-г ажиллуулж, юу ч устгаагүй бол NotFound буцаана.
func (r *gatewayRepository) execDelete(ctx context.Context, sql, id, notFoundMsg string) error {
	tag, err := r.pool.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound(notFoundMsg)
	}
	return nil
}

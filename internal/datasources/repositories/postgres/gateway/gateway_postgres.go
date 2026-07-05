// Gerege Template Version 27.0
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
	s, err := scanService(r.pool.QueryRow(ctx,
		`INSERT INTO gateway_services(name, protocol, host, port, path, retries, connect_timeout_ms, tags, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+serviceColumns,
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

// ── Routes ──────────────────────────────────────────────────────────────────

const routeSelect = `SELECT r.id, r.service_id, s.name, r.name, r.methods, r.paths,
	r.strip_path, r.preserve_host, r.enabled, r.created_at, r.updated_at
	FROM gateway_routes r JOIN gateway_services s ON s.id = r.service_id`

func scanRoute(row pgx.Row) (domain.GatewayRoute, error) {
	var rt domain.GatewayRoute
	err := row.Scan(&rt.ID, &rt.ServiceID, &rt.ServiceName, &rt.Name, &rt.Methods, &rt.Paths,
		&rt.StripPath, &rt.PreserveHost, &rt.Enabled, &rt.CreatedAt, &rt.UpdatedAt)
	return rt, err
}

func (r *gatewayRepository) ListRoutes(ctx context.Context) ([]domain.GatewayRoute, error) {
	rows, err := r.pool.Query(ctx, routeSelect+` ORDER BY r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayRoute, 0, 16)
	for rows.Next() {
		rt, scanErr := scanRoute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, rt)
	}
	return out, rows.Err()
}

func (r *gatewayRepository) GetRoute(ctx context.Context, id string) (domain.GatewayRoute, error) {
	rt, err := scanRoute(r.pool.QueryRow(ctx, routeSelect+` WHERE r.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GatewayRoute{}, apperror.NotFound("route not found")
	}
	return rt, err
}

func (r *gatewayRepository) CreateRoute(ctx context.Context, in *domain.GatewayRoute) (domain.GatewayRoute, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO gateway_routes(service_id, name, methods, paths, strip_path, preserve_host, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		in.ServiceID, in.Name, in.Methods, in.Paths, in.StripPath, in.PreserveHost, in.Enabled).Scan(&id)
	if err != nil {
		return domain.GatewayRoute{}, mapWrite(err, "route already exists")
	}
	return r.GetRoute(ctx, id)
}

func (r *gatewayRepository) UpdateRoute(ctx context.Context, in *domain.GatewayRoute) (domain.GatewayRoute, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gateway_routes SET service_id=$2, name=$3, methods=$4, paths=$5, strip_path=$6,
		 preserve_host=$7, enabled=$8, updated_at=now() WHERE id=$1`,
		in.ID, in.ServiceID, in.Name, in.Methods, in.Paths, in.StripPath, in.PreserveHost, in.Enabled)
	if err != nil {
		return domain.GatewayRoute{}, mapWrite(err, "route already exists")
	}
	if tag.RowsAffected() == 0 {
		return domain.GatewayRoute{}, apperror.NotFound("route not found")
	}
	return r.GetRoute(ctx, in.ID)
}

func (r *gatewayRepository) DeleteRoute(ctx context.Context, id string) error {
	return r.execDelete(ctx, `DELETE FROM gateway_routes WHERE id = $1`, id, "route not found")
}

// ── Consumers ─────────────────────────────────────────────────────────────—

const consumerSelect = `SELECT c.id, c.username, c.custom_id, c.tags, c.enabled, c.created_at, c.updated_at,
	(SELECT count(*) FROM gateway_api_keys k WHERE k.consumer_id = c.id AND NOT k.revoked) AS key_count
	FROM gateway_consumers c`

func scanConsumer(row pgx.Row) (domain.GatewayConsumer, error) {
	var c domain.GatewayConsumer
	err := row.Scan(&c.ID, &c.Username, &c.CustomID, &c.Tags, &c.Enabled, &c.CreatedAt, &c.UpdatedAt, &c.KeyCount)
	return c, err
}

func (r *gatewayRepository) ListConsumers(ctx context.Context) ([]domain.GatewayConsumer, error) {
	rows, err := r.pool.Query(ctx, consumerSelect+` ORDER BY c.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayConsumer, 0, 16)
	for rows.Next() {
		c, scanErr := scanConsumer(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *gatewayRepository) GetConsumer(ctx context.Context, id string) (domain.GatewayConsumer, error) {
	c, err := scanConsumer(r.pool.QueryRow(ctx, consumerSelect+` WHERE c.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GatewayConsumer{}, apperror.NotFound("consumer not found")
	}
	return c, err
}

func (r *gatewayRepository) CreateConsumer(ctx context.Context, in *domain.GatewayConsumer) (domain.GatewayConsumer, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO gateway_consumers(username, custom_id, tags, enabled) VALUES ($1,$2,$3,$4) RETURNING id`,
		in.Username, in.CustomID, in.Tags, in.Enabled).Scan(&id)
	if err != nil {
		return domain.GatewayConsumer{}, mapWrite(err, "consumer username already exists")
	}
	return r.GetConsumer(ctx, id)
}

func (r *gatewayRepository) UpdateConsumer(ctx context.Context, in *domain.GatewayConsumer) (domain.GatewayConsumer, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gateway_consumers SET username=$2, custom_id=$3, tags=$4, enabled=$5, updated_at=now() WHERE id=$1`,
		in.ID, in.Username, in.CustomID, in.Tags, in.Enabled)
	if err != nil {
		return domain.GatewayConsumer{}, mapWrite(err, "consumer username already exists")
	}
	if tag.RowsAffected() == 0 {
		return domain.GatewayConsumer{}, apperror.NotFound("consumer not found")
	}
	return r.GetConsumer(ctx, in.ID)
}

func (r *gatewayRepository) DeleteConsumer(ctx context.Context, id string) error {
	return r.execDelete(ctx, `DELETE FROM gateway_consumers WHERE id = $1`, id, "consumer not found")
}

// ── API keys ─────────────────────────────────────────────────────────────—

const keyColumns = `id, consumer_id, label, key_prefix, key_hash, last_used_at, expires_at, revoked, created_at`

func scanKey(row pgx.Row) (domain.GatewayAPIKey, error) {
	var k domain.GatewayAPIKey
	err := row.Scan(&k.ID, &k.ConsumerID, &k.Label, &k.Prefix, &k.Hash,
		&k.LastUsedAt, &k.ExpiresAt, &k.Revoked, &k.CreatedAt)
	return k, err
}

func (r *gatewayRepository) ListKeys(ctx context.Context, consumerID string) ([]domain.GatewayAPIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+keyColumns+` FROM gateway_api_keys WHERE consumer_id = $1 ORDER BY created_at DESC`, consumerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayAPIKey, 0, 8)
	for rows.Next() {
		k, scanErr := scanKey(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *gatewayRepository) CreateKey(ctx context.Context, in *domain.GatewayAPIKey) (domain.GatewayAPIKey, error) {
	k, err := scanKey(r.pool.QueryRow(ctx,
		`INSERT INTO gateway_api_keys(consumer_id, label, key_prefix, key_hash, expires_at)
		 VALUES ($1,$2,$3,$4,$5) RETURNING `+keyColumns,
		in.ConsumerID, in.Label, in.Prefix, in.Hash, in.ExpiresAt))
	if err != nil {
		return domain.GatewayAPIKey{}, mapWrite(err, "api key already exists")
	}
	// Plaintext-ийг дуудагчаас дамжуулж буцаана (DB-д хадгалагдахгүй).
	k.Plaintext = in.Plaintext
	return k, nil
}

func (r *gatewayRepository) RevokeKey(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE gateway_api_keys SET revoked = true WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("api key not found")
	}
	return nil
}

func (r *gatewayRepository) DeleteKey(ctx context.Context, id string) error {
	return r.execDelete(ctx, `DELETE FROM gateway_api_keys WHERE id = $1`, id, "api key not found")
}

// ── Policies ─────────────────────────────────────────────────────────────—

const policySelect = `SELECT p.id, p.route_id, COALESCE(r.name, ''), p.type, p.config, p.enabled, p.created_at, p.updated_at
	FROM gateway_policies p LEFT JOIN gateway_routes r ON r.id = p.route_id`

func scanPolicy(row pgx.Row) (domain.GatewayPolicy, error) {
	var p domain.GatewayPolicy
	err := row.Scan(&p.ID, &p.RouteID, &p.RouteName, &p.Type, &p.Config, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *gatewayRepository) ListPolicies(ctx context.Context) ([]domain.GatewayPolicy, error) {
	rows, err := r.pool.Query(ctx, policySelect+` ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayPolicy, 0, 16)
	for rows.Next() {
		p, scanErr := scanPolicy(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *gatewayRepository) GetPolicy(ctx context.Context, id string) (domain.GatewayPolicy, error) {
	p, err := scanPolicy(r.pool.QueryRow(ctx, policySelect+` WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GatewayPolicy{}, apperror.NotFound("policy not found")
	}
	return p, err
}

func (r *gatewayRepository) CreatePolicy(ctx context.Context, in *domain.GatewayPolicy) (domain.GatewayPolicy, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO gateway_policies(route_id, type, config, enabled) VALUES ($1,$2,$3,$4) RETURNING id`,
		in.RouteID, in.Type, in.Config, in.Enabled).Scan(&id)
	if err != nil {
		return domain.GatewayPolicy{}, mapWrite(err, "policy already exists")
	}
	return r.GetPolicy(ctx, id)
}

func (r *gatewayRepository) UpdatePolicy(ctx context.Context, in *domain.GatewayPolicy) (domain.GatewayPolicy, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gateway_policies SET route_id=$2, type=$3, config=$4, enabled=$5, updated_at=now() WHERE id=$1`,
		in.ID, in.RouteID, in.Type, in.Config, in.Enabled)
	if err != nil {
		return domain.GatewayPolicy{}, mapWrite(err, "policy already exists")
	}
	if tag.RowsAffected() == 0 {
		return domain.GatewayPolicy{}, apperror.NotFound("policy not found")
	}
	return r.GetPolicy(ctx, in.ID)
}

func (r *gatewayRepository) DeletePolicy(ctx context.Context, id string) error {
	return r.execDelete(ctx, `DELETE FROM gateway_policies WHERE id = $1`, id, "policy not found")
}

// ── Telemetry ────────────────────────────────────────────────────────────—

func (r *gatewayRepository) ListRequestLogs(ctx context.Context, limit int) ([]domain.GatewayRequestLog, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT l.id, l.route_id, COALESCE(r.name,''), l.consumer_id, COALESCE(c.username,''),
		 l.method, l.path, l.status, l.latency_ms, l.client_ip, l.created_at
		 FROM gateway_request_logs l
		 LEFT JOIN gateway_routes r ON r.id = l.route_id
		 LEFT JOIN gateway_consumers c ON c.id = l.consumer_id
		 ORDER BY l.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayRequestLog, 0, limit)
	for rows.Next() {
		var l domain.GatewayRequestLog
		if err := rows.Scan(&l.ID, &l.RouteID, &l.RouteName, &l.ConsumerID, &l.Consumer,
			&l.Method, &l.Path, &l.Status, &l.LatencyMS, &l.ClientIP, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Overview нь dashboard-ийн нэгтгэлийг тооцоолно. Тоологдох утгууд (services/
// routes/consumers/keys) нь бүх хугацааных; харин request телеметр нь сүүлийн
// 24 цагийнх. Хувь/p95-ийг нэг query-д percentile_cont-оор гаргана.
func (r *gatewayRepository) Overview(ctx context.Context) (domain.GatewayOverview, error) {
	var o domain.GatewayOverview
	if err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM gateway_services),
			(SELECT count(*) FROM gateway_routes),
			(SELECT count(*) FROM gateway_consumers),
			(SELECT count(*) FROM gateway_api_keys WHERE NOT revoked),
			COALESCE(count(*),0),
			COALESCE(count(*) FILTER (WHERE status >= 500),0),
			COALESCE(count(*) FILTER (WHERE status = 429),0),
			COALESCE(avg(latency_ms),0)::int,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms),0)::int
		FROM gateway_request_logs WHERE created_at >= now() - interval '24 hours'`,
	).Scan(&o.Services, &o.Routes, &o.Consumers, &o.ActiveKeys,
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

	// Топ route-ууд (хүсэлтийн тоогоор).
	top, err := r.topRoutes(ctx)
	if err != nil {
		return domain.GatewayOverview{}, err
	}
	o.TopRoutes = top
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

func (r *gatewayRepository) topRoutes(ctx context.Context) ([]domain.GatewayRouteStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(r.name, l.path), count(*) AS n
		FROM gateway_request_logs l LEFT JOIN gateway_routes r ON r.id = l.route_id
		WHERE l.created_at >= now() - interval '24 hours'
		GROUP BY 1 ORDER BY n DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GatewayRouteStat, 0, 5)
	for rows.Next() {
		var s domain.GatewayRouteStat
		if err := rows.Scan(&s.RouteName, &s.Count); err != nil {
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

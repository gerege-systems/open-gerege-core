// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package relay нь platform-хоорондын хүсэлт дамжуулах + SLA хяналтын Postgres
// gateway. gateway_postgres-ийн адил platform-хоорондын тохиргоо/telemetry тул
// RLS-гүй (plain pool query).
package relay

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

type relayRepository struct {
	pool *pgxpool.Pool
}

func NewRelayRepository(pool *pgxpool.Pool) repointerface.RelayRepository {
	return &relayRepository{pool: pool}
}

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

func (r *relayRepository) execDelete(ctx context.Context, sql, id, notFoundMsg string) error {
	tag, err := r.pool.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound(notFoundMsg)
	}
	return nil
}

// ── Platforms ────────────────────────────────────────────────────────────────

const platformColumns = `id, code, name, direction, endpoint_url, supervisor_contact, webhook_secret, enabled, created_at`

func scanPlatform(row pgx.Row) (domain.RelayPlatform, error) {
	var p domain.RelayPlatform
	err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Direction, &p.EndpointURL, &p.SupervisorContact, &p.WebhookSecret, &p.Enabled, &p.CreatedAt)
	return p, err
}

// GetPlatformByCode нь code-оор нэг platform-ыг олно (webhook баталгаажуулах,
// дээшээ дамжуулах эх/хүрэх platform-ыг тодорхойлоход).
func (r *relayRepository) GetPlatformByCode(ctx context.Context, code string) (domain.RelayPlatform, error) {
	p, err := scanPlatform(r.pool.QueryRow(ctx,
		`SELECT `+platformColumns+` FROM relay_platforms WHERE code = $1`, code))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RelayPlatform{}, apperror.NotFound("platform not found")
		}
		return domain.RelayPlatform{}, err
	}
	return p, nil
}

func (r *relayRepository) ListPlatforms(ctx context.Context) ([]domain.RelayPlatform, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+platformColumns+` FROM relay_platforms ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RelayPlatform, 0, 16)
	for rows.Next() {
		p, scanErr := scanPlatform(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *relayRepository) CreatePlatform(ctx context.Context, in *domain.RelayPlatform) (domain.RelayPlatform, error) {
	p, err := scanPlatform(r.pool.QueryRow(ctx,
		`INSERT INTO relay_platforms(code, name, direction, endpoint_url, supervisor_contact, webhook_secret, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+platformColumns,
		in.Code, in.Name, in.Direction, in.EndpointURL, in.SupervisorContact, in.WebhookSecret, in.Enabled))
	if err != nil {
		return domain.RelayPlatform{}, mapWrite(err, "platform code already exists")
	}
	return p, nil
}

func (r *relayRepository) DeletePlatform(ctx context.Context, id string) error {
	return r.execDelete(ctx, `DELETE FROM relay_platforms WHERE id = $1`, id, "platform not found")
}

// ── Routes ───────────────────────────────────────────────────────────────────

const routeSelect = `SELECT rr.id, rr.service_code, rr.platform_id, p.name, rr.sla_minutes, rr.created_at
	FROM relay_routes rr JOIN relay_platforms p ON p.id = rr.platform_id`

func scanRoute(row pgx.Row) (domain.RelayRoute, error) {
	var rt domain.RelayRoute
	err := row.Scan(&rt.ID, &rt.ServiceCode, &rt.PlatformID, &rt.PlatformName, &rt.SLAMinutes, &rt.CreatedAt)
	return rt, err
}

func (r *relayRepository) scanRoutes(rows pgx.Rows) ([]domain.RelayRoute, error) {
	defer rows.Close()
	out := make([]domain.RelayRoute, 0, 16)
	for rows.Next() {
		rt, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, rows.Err()
}

func (r *relayRepository) ListRoutes(ctx context.Context) ([]domain.RelayRoute, error) {
	rows, err := r.pool.Query(ctx, routeSelect+` ORDER BY rr.service_code, p.name`)
	if err != nil {
		return nil, err
	}
	return r.scanRoutes(rows)
}

func (r *relayRepository) RoutesForService(ctx context.Context, serviceCode string) ([]domain.RelayRoute, error) {
	rows, err := r.pool.Query(ctx, routeSelect+` WHERE rr.service_code = $1 AND p.enabled ORDER BY p.name`, serviceCode)
	if err != nil {
		return nil, err
	}
	return r.scanRoutes(rows)
}

func (r *relayRepository) CreateRoute(ctx context.Context, in *domain.RelayRoute) (domain.RelayRoute, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO relay_routes(service_code, platform_id, sla_minutes) VALUES ($1,$2,$3) RETURNING id`,
		in.ServiceCode, in.PlatformID, in.SLAMinutes).Scan(&id)
	if err != nil {
		return domain.RelayRoute{}, mapWrite(err, "route already exists for this service+platform")
	}
	return scanRoute(r.pool.QueryRow(ctx, routeSelect+` WHERE rr.id = $1`, id))
}

func (r *relayRepository) DeleteRoute(ctx context.Context, id string) error {
	return r.execDelete(ctx, `DELETE FROM relay_routes WHERE id = $1`, id, "route not found")
}

// ── Requests + assignments ───────────────────────────────────────────────────

const requestColumns = `id, source_platform, external_ref, service_code, title, payload, priority,
	received_at, due_at, status, result, fulfilled_at, breach_notified, updated_at`

func scanRequest(row pgx.Row) (domain.RelayRequest, error) {
	var q domain.RelayRequest
	err := row.Scan(&q.ID, &q.SourcePlatform, &q.ExternalRef, &q.ServiceCode, &q.Title, &q.Payload,
		&q.Priority, &q.ReceivedAt, &q.DueAt, &q.Status, &q.Result, &q.FulfilledAt, &q.BreachNotified, &q.UpdatedAt)
	return q, err
}

const assignmentColumns = `a.id, a.request_id, a.platform_id, p.name, a.status, a.due_at,
	a.dispatched_at, a.responded_at, a.result, a.reminders_sent, a.escalated`

func scanAssignment(row pgx.Row) (domain.RelayAssignment, error) {
	var a domain.RelayAssignment
	err := row.Scan(&a.ID, &a.RequestID, &a.PlatformID, &a.PlatformName, &a.Status, &a.DueAt,
		&a.DispatchedAt, &a.RespondedAt, &a.Result, &a.RemindersSent, &a.Escalated)
	return a, err
}

func (r *relayRepository) CreateRequestWithAssignments(ctx context.Context, req *domain.RelayRequest, asg []domain.RelayAssignment) (domain.RelayRequest, []domain.RelayAssignment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RelayRequest{}, nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit returns ErrTxClosed — expected

	var reqID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO relay_requests(source_platform, external_ref, service_code, title, payload, priority, due_at, status)
		VALUES ($1,$2,$3,$4,COALESCE($5,'{}'::jsonb),$6,$7,$8) RETURNING id`,
		req.SourcePlatform, req.ExternalRef, req.ServiceCode, req.Title, req.Payload, req.Priority, req.DueAt, req.Status,
	).Scan(&reqID); err != nil {
		return domain.RelayRequest{}, nil, mapWrite(err, "request already exists")
	}

	for i := range asg {
		var aID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO relay_assignments(request_id, platform_id, status, due_at) VALUES ($1,$2,$3,$4) RETURNING id`,
			reqID, asg[i].PlatformID, domain.RelayAsgPending, asg[i].DueAt,
		).Scan(&aID); err != nil {
			return domain.RelayRequest{}, nil, mapWrite(err, "assignment insert failed")
		}
		asg[i].ID = aID
		asg[i].RequestID = reqID
		asg[i].Status = domain.RelayAsgPending
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RelayRequest{}, nil, err
	}
	stored, err := scanRequest(r.pool.QueryRow(ctx, `SELECT `+requestColumns+` FROM relay_requests WHERE id = $1`, reqID))
	if err != nil {
		return domain.RelayRequest{}, nil, err
	}
	return stored, asg, nil
}

func (r *relayRepository) GetAssignment(ctx context.Context, id string) (domain.RelayAssignment, error) {
	a, err := scanAssignment(r.pool.QueryRow(ctx,
		`SELECT `+assignmentColumns+` FROM relay_assignments a JOIN relay_platforms p ON p.id = a.platform_id WHERE a.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RelayAssignment{}, apperror.NotFound("assignment not found")
	}
	return a, err
}

func (r *relayRepository) MarkDispatched(ctx context.Context, assignmentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE relay_assignments SET dispatched_at = now(), status = $2 WHERE id = $1 AND dispatched_at IS NULL`,
		assignmentID, domain.RelayAsgAcknowledged)
	return err
}

// RespondAssignment нь assignment-ыг терминал төлөвт оруулж, бүх assignment
// терминал болсон бол хүсэлтийг fulfilled болгоно.
func (r *relayRepository) RespondAssignment(ctx context.Context, assignmentID, status string, result []byte) (domain.RelayRequest, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RelayRequest{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit returns ErrTxClosed — expected

	var reqID string
	if err := tx.QueryRow(ctx,
		`UPDATE relay_assignments SET status = $2, result = $3, responded_at = now()
		 WHERE id = $1 AND status NOT IN ('done','rejected') RETURNING request_id`,
		assignmentID, status, result,
	).Scan(&reqID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RelayRequest{}, false, apperror.Conflict("assignment already responded or not found")
		}
		return domain.RelayRequest{}, false, err
	}

	// Бүх assignment терминал болсон эсэх.
	var pending int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM relay_assignments WHERE request_id = $1 AND status NOT IN ('done','rejected')`, reqID,
	).Scan(&pending); err != nil {
		return domain.RelayRequest{}, false, err
	}

	fulfilled := false
	if pending == 0 {
		// Хүсэлтийн нэгтгэсэн хариу: assignment-уудын result-ыг цуглуулна.
		if _, err := tx.Exec(ctx, `
			UPDATE relay_requests SET status = $2, fulfilled_at = now(), updated_at = now(),
				result = COALESCE((
					SELECT jsonb_agg(jsonb_build_object('platform_id', a.platform_id, 'status', a.status, 'result', a.result))
					FROM relay_assignments a WHERE a.request_id = $1
				), '[]'::jsonb)
			WHERE id = $1`, reqID, domain.RelayReqFulfilled); err != nil {
			return domain.RelayRequest{}, false, err
		}
		fulfilled = true
	} else {
		_, _ = tx.Exec(ctx, `UPDATE relay_requests SET status = $2, updated_at = now()
			WHERE id = $1 AND status IN ('received','dispatched')`, reqID, domain.RelayReqInProgress)
	}

	stored, err := scanRequest(tx.QueryRow(ctx, `SELECT `+requestColumns+` FROM relay_requests WHERE id = $1`, reqID))
	if err != nil {
		return domain.RelayRequest{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RelayRequest{}, false, err
	}
	return stored, fulfilled, nil
}

// ── SLA sweep queries ────────────────────────────────────────────────────────

func (r *relayRepository) scanAssignments(rows pgx.Rows) ([]domain.RelayAssignment, error) {
	defer rows.Close()
	out := make([]domain.RelayAssignment, 0, 32)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DueSoonAssignments нь идэвхтэй (терминал биш, overdue биш) бөгөөд due_at
// хараахан болоогүй assignment-уудыг буцаана (reminder босго шалгахад).
func (r *relayRepository) DueSoonAssignments(ctx context.Context) ([]domain.RelayAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+assignmentColumns+` FROM relay_assignments a JOIN relay_platforms p ON p.id = a.platform_id
		 WHERE a.status IN ('pending','acknowledged','in_progress') AND a.due_at > now()`)
	if err != nil {
		return nil, err
	}
	return r.scanAssignments(rows)
}

// OverdueAssignments нь due_at өнгөрсөн ч терминал/overdue болоогүй assignment-ууд.
func (r *relayRepository) OverdueAssignments(ctx context.Context) ([]domain.RelayAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+assignmentColumns+` FROM relay_assignments a JOIN relay_platforms p ON p.id = a.platform_id
		 WHERE a.status IN ('pending','acknowledged','in_progress','overdue') AND a.due_at <= now()`)
	if err != nil {
		return nil, err
	}
	return r.scanAssignments(rows)
}

func (r *relayRepository) MarkAssignmentOverdue(ctx context.Context, assignmentID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE relay_assignments SET status = $2 WHERE id = $1 AND status <> 'overdue'`,
		assignmentID, domain.RelayAsgOverdue)
	return err
}

func (r *relayRepository) IncReminders(ctx context.Context, assignmentID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE relay_assignments SET reminders_sent = reminders_sent + 1 WHERE id = $1`, assignmentID)
	return err
}

func (r *relayRepository) MarkEscalated(ctx context.Context, assignmentID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE relay_assignments SET escalated = true WHERE id = $1`, assignmentID)
	return err
}

func (r *relayRepository) MarkRequestOverdue(ctx context.Context, requestID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE relay_requests SET status = $2, updated_at = now() WHERE id = $1 AND status NOT IN ('fulfilled','rejected','overdue')`,
		requestID, domain.RelayReqOverdue)
	return err
}

func (r *relayRepository) MarkBreachNotified(ctx context.Context, requestID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE relay_requests SET breach_notified = true WHERE id = $1 AND breach_notified = false`, requestID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ── Events ───────────────────────────────────────────────────────────────────

func (r *relayRepository) AppendEvent(ctx context.Context, e *domain.RelayEvent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO relay_events(request_id, assignment_id, type, detail) VALUES ($1,$2,$3,$4)`,
		e.RequestID, e.AssignmentID, e.Type, e.Detail)
	return err
}

func (r *relayRepository) recentEvents(ctx context.Context, limit int) ([]domain.RelayEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, request_id, assignment_id, type, detail, created_at FROM relay_events ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RelayEvent, 0, limit)
	for rows.Next() {
		var e domain.RelayEvent
		if err := rows.Scan(&e.ID, &e.RequestID, &e.AssignmentID, &e.Type, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ── Dashboard + жагсаалт ─────────────────────────────────────────────────────

func (r *relayRepository) Overview(ctx context.Context) (domain.RelayOverview, error) {
	var o domain.RelayOverview
	var onTimeFulfilled int
	if err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(count(*),0),
			COALESCE(count(*) FILTER (WHERE received_at >= date_trunc('day', now())),0),
			COALESCE(count(*) FILTER (WHERE status IN ('received','dispatched','in_progress')),0),
			COALESCE(count(*) FILTER (WHERE status = 'overdue'),0),
			COALESCE(count(*) FILTER (WHERE status = 'fulfilled'),0),
			COALESCE(count(*) FILTER (WHERE status = 'fulfilled' AND fulfilled_at <= due_at),0),
			COALESCE(avg(EXTRACT(EPOCH FROM (fulfilled_at - received_at))/60) FILTER (WHERE status='fulfilled'),0)::int
		FROM relay_requests`,
	).Scan(&o.Total, &o.ReceivedToday, &o.InProgress, &o.Overdue, &o.Fulfilled, &onTimeFulfilled, &o.AvgFulfillMins); err != nil {
		return domain.RelayOverview{}, err
	}
	if o.Fulfilled > 0 {
		o.SLACompliancePct = float64(onTimeFulfilled) / float64(o.Fulfilled)
	}

	buckets, err := r.statusBuckets(ctx)
	if err != nil {
		return domain.RelayOverview{}, err
	}
	o.StatusBuckets = buckets

	plats, err := r.platformStats(ctx)
	if err != nil {
		return domain.RelayOverview{}, err
	}
	o.Platforms = plats

	events, err := r.recentEvents(ctx, 20)
	if err != nil {
		return domain.RelayOverview{}, err
	}
	o.RecentEvents = events
	return o, nil
}

func (r *relayRepository) statusBuckets(ctx context.Context) ([]domain.RelayStatusBucket, error) {
	rows, err := r.pool.Query(ctx, `SELECT status, count(*) FROM relay_requests GROUP BY status ORDER BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RelayStatusBucket, 0, 6)
	for rows.Next() {
		var b domain.RelayStatusBucket
		if err := rows.Scan(&b.Status, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *relayRepository) platformStats(ctx context.Context) ([]domain.RelayPlatformStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.name,
			COALESCE(count(a.id),0),
			COALESCE(count(a.id) FILTER (WHERE a.status = 'done'),0),
			COALESCE(count(a.id) FILTER (WHERE a.status = 'overdue'),0),
			COALESCE(count(a.id) FILTER (WHERE a.status IN ('pending','acknowledged','in_progress')),0),
			COALESCE(count(a.id) FILTER (WHERE a.status = 'done' AND a.responded_at <= a.due_at),0)
		FROM relay_platforms p LEFT JOIN relay_assignments a ON a.platform_id = p.id
		GROUP BY p.id, p.name ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RelayPlatformStat, 0, 8)
	for rows.Next() {
		var s domain.RelayPlatformStat
		var onTime int
		if err := rows.Scan(&s.PlatformID, &s.PlatformName, &s.Total, &s.Done, &s.Overdue, &s.Pending, &onTime); err != nil {
			return nil, err
		}
		if s.Done > 0 {
			s.CompliancePct = float64(onTime) / float64(s.Done)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *relayRepository) ListRequests(ctx context.Context, limit int) ([]domain.RelayRequest, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+requestColumns+` FROM relay_requests ORDER BY received_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RelayRequest, 0, limit)
	for rows.Next() {
		q, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *relayRepository) GetRequestDetail(ctx context.Context, id string) (domain.RelayRequestDetail, error) {
	var d domain.RelayRequestDetail
	req, err := scanRequest(r.pool.QueryRow(ctx, `SELECT `+requestColumns+` FROM relay_requests WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RelayRequestDetail{}, apperror.NotFound("request not found")
	}
	if err != nil {
		return domain.RelayRequestDetail{}, err
	}
	d.Request = req

	arows, err := r.pool.Query(ctx,
		`SELECT `+assignmentColumns+` FROM relay_assignments a JOIN relay_platforms p ON p.id = a.platform_id
		 WHERE a.request_id = $1 ORDER BY p.name`, id)
	if err != nil {
		return domain.RelayRequestDetail{}, err
	}
	asg, err := r.scanAssignments(arows)
	if err != nil {
		return domain.RelayRequestDetail{}, err
	}
	d.Assignments = asg

	erows, err := r.pool.Query(ctx,
		`SELECT id, request_id, assignment_id, type, detail, created_at FROM relay_events WHERE request_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return domain.RelayRequestDetail{}, err
	}
	defer erows.Close()
	for erows.Next() {
		var e domain.RelayEvent
		if err := erows.Scan(&e.ID, &e.RequestID, &e.AssignmentID, &e.Type, &e.Detail, &e.CreatedAt); err != nil {
			return domain.RelayRequestDetail{}, err
		}
		d.Events = append(d.Events, e)
	}
	return d, erows.Err()
}

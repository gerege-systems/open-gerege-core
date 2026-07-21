// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gov нь иргэний "Төрийн үйлчилгээ" порталын Postgres gateway. gov_services
// нь нийтийн каталог (RLS-гүй лавлах); бусад хүснэгтүүд нь хэрэглэгч-тус-бүрийн
// тул хоёр давхар хамгаалалттай: query бүр user_id-гаар ИЛ шүүгдэхээс гадна
// per-user query бүр withRLS транзакцид ороож app.user_id / app.user_role GUC-г
// тавьдаг тул Postgres-ийн Row-Level Security бодлого (migration 20) мөрийн
// харагдалтыг мөн адил хязгаарлана (defense-in-depth).
package gov

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type govRepository struct {
	pool *pgxpool.Pool
}

func NewGovRepository(pool *pgxpool.Pool) repointerface.GovRepository {
	return &govRepository{pool: pool}
}

// withRLS нь нэг query-г транзакцид боож, тухайн транзакцид зориулж Postgres-ийн
// Row-Level Security session хувьсагчдыг (app.user_id, app.user_role) тогтооно.
// Утгуудыг context-оос (rls.FromContext) уншиж авдаг.
//
// set_config-ийн гурав дахь аргумент (is_local) нь `true` — `SET LOCAL`-той
// дүйцэх бөгөөд утгыг зөвхөн идэвхтэй транзакцийн туршид хадгална; pgx pool дахь
// холболт дараагийн хамааралгүй хүсэлт рүү identity алдагдуулахаас сэргийлнэ.
//
// context-д Identity байхгүй бол UserID/Role хоосон болж RLS бодлогууд бүх
// мөрийг хаана — аюулгүй өгөгдмөл (fail-closed).
func (r *govRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
	id, _ := rls.FromContext(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit returns ErrTxClosed — expected, nothing to handle

	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.user_id',$1,true), set_config('app.user_role',$2,true)`,
		id.UserID, string(id.Role),
	); err != nil {
		return fmt.Errorf("set rls session context: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── Catalog (нийтийн — RLS-гүй) ─────────────────────────────────────────────

const serviceColumns = `id, code, name, category, agency, description, fee, processing_days,
	processing_time, cofog_code, cofog_label, main_activity, sdg_code, output_type,
	output_ref_type, evidence, legal_basis, assurance_level, lifecycle, fulfilment,
	has_discretion, has_assessment, sla_hours, tacit_approval, online, enabled, created_at`

func scanService(row pgx.Row) (domain.GovService, error) {
	var s domain.GovService
	err := row.Scan(&s.ID, &s.Code, &s.Name, &s.Category, &s.Agency, &s.Description,
		&s.Fee, &s.ProcessingDays, &s.ProcessingTime, &s.COFOGCode, &s.COFOGLabel,
		&s.MainActivity, &s.SDGCode, &s.OutputType, &s.OutputRefType, &s.Evidence,
		&s.LegalBasis, &s.AssuranceLevel, &s.Lifecycle, &s.Fulfilment,
		&s.HasDiscretion, &s.HasAssessment, &s.SLAHours, &s.TacitApproval,
		&s.Online, &s.Enabled, &s.CreatedAt)
	return s, err
}

// attachLifeEvents нь өгөгдсөн үйлчилгээнүүдэд харгалзах Event-үүдийг нэг
// нэмэлт query-гээр хавсаргана (N+1-ээс сэргийлнэ).
func (r *govRepository) attachLifeEvents(ctx context.Context, list []domain.GovService) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]string, 0, len(list))
	for _, s := range list {
		ids = append(ids, s.ID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT se.service_id, le.code, le.name, le.kind, le.eu_code, le.en_label
		  FROM gov_service_events se
		  JOIN gov_life_events le ON le.code = se.event_code
		 WHERE se.service_id = ANY($1)
		 ORDER BY le.sort_order`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	byService := make(map[string][]domain.GovLifeEvent, len(list))
	for rows.Next() {
		var sid string
		var ev domain.GovLifeEvent
		if err := rows.Scan(&sid, &ev.Code, &ev.Name, &ev.Kind, &ev.EUCode, &ev.ENLabel); err != nil {
			return err
		}
		byService[sid] = append(byService[sid], ev)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range list {
		list[i].LifeEvents = byService[list[i].ID]
	}
	return nil
}

func (r *govRepository) ListServices(ctx context.Context) ([]domain.GovService, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serviceColumns+`
		FROM gov_services WHERE enabled AND lifecycle = 'active' ORDER BY category, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GovService, 0, 16)
	for rows.Next() {
		s, scanErr := scanService(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachLifeEvents(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *govRepository) GetService(ctx context.Context, id string) (domain.GovService, error) {
	s, err := scanService(r.pool.QueryRow(ctx, `SELECT `+serviceColumns+` FROM gov_services WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GovService{}, apperror.NotFound("service not found")
	}
	return s, err
}

func (r *govRepository) ListLifeEvents(ctx context.Context) ([]domain.GovLifeEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT code, name, kind, eu_code, en_label FROM gov_life_events ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GovLifeEvent, 0, 16)
	for rows.Next() {
		var ev domain.GovLifeEvent
		if err := rows.Scan(&ev.Code, &ev.Name, &ev.Kind, &ev.EUCode, &ev.ENLabel); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// ── Applications (per-user — withRLS) ───────────────────────────────────────

func scanApplication(row pgx.Row) (domain.GovApplication, error) {
	var a domain.GovApplication
	err := row.Scan(&a.ID, &a.UserID, &a.ServiceID, &a.ServiceCode, &a.ServiceName,
		&a.ReferenceNo, &a.Status, &a.Result, &a.Note, &a.Payload,
		&a.AssignedTo, &a.AssignedAt, &a.DecidedBy, &a.DecidedAt, &a.DecisionNote,
		&a.DueAt, &a.SLABreached, &a.SuspendedAt, &a.OutputRefID, &a.Tacit,
		&a.SubmittedAt, &a.UpdatedAt)
	return a, err
}

const appColumns = `id, user_id, service_id, service_code, service_name, reference_no,
	status, result, note, payload, assigned_to, assigned_at, decided_by, decided_at,
	decision_note, due_at, sla_breached, suspended_at, output_ref_id, tacit,
	submitted_at, updated_at`

func (r *govRepository) ListApplications(ctx context.Context, userID string) ([]domain.GovApplication, error) {
	var out []domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+appColumns+` FROM gov_applications WHERE user_id = $1 ORDER BY submitted_at DESC`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovApplication, 0, 16)
		for rows.Next() {
			a, scanErr := scanApplication(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

func (r *govRepository) GetApplication(ctx context.Context, userID, id string) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, scanErr := scanApplication(tx.QueryRow(ctx,
			`SELECT `+appColumns+` FROM gov_applications WHERE id = $1 AND user_id = $2`, id, userID))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.NotFound("application not found")
		}
		out = a
		return scanErr
	})
	return out, err
}

// insertApplication нь өгөгдсөн транзакцид хүсэлт үүсгэнэ (CreateApplication
// болон CreateApplicationWithOutput хоёулаа үүнийг ашиглана).
func insertApplication(ctx context.Context, tx pgx.Tx, in *domain.GovApplication) (domain.GovApplication, error) {
	payload := in.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	return scanApplication(tx.QueryRow(ctx,
		`INSERT INTO gov_applications
			(user_id, service_id, service_code, service_name, reference_no,
			 status, result, note, payload, due_at, decided_at, decision_note, tacit)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING `+appColumns,
		in.UserID, in.ServiceID, in.ServiceCode, in.ServiceName, in.ReferenceNo,
		in.Status, in.Result, in.Note, payload, in.DueAt, in.DecidedAt,
		in.DecisionNote, in.Tacit))
}

func (r *govRepository) CreateApplication(ctx context.Context, in *domain.GovApplication) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, scanErr := insertApplication(ctx, tx, in)
		if scanErr != nil {
			return scanErr
		}
		out = a
		return appendEventTx(ctx, tx, &domain.GovApplicationEvent{
			ApplicationID: a.ID, ActorID: &a.UserID, ActorRole: "user",
			ToStatus: a.Status, Type: "created",
			Detail: "Хүсэлт илгээгдэв",
		}, a.UserID)
	})
	return out, err
}

// CreateApplicationWithOutput нь AUTO горимын үйлчилгээг НЭГ транзакцид бүрэн
// биелүүлнэ: хүсэлт (completed) → лавлагаа → мэдэгдэл → timeline. Аль нэг алхам
// бүтэлгүйтвэл БҮГД буцна.
//
// EU 2018/1724 Art.6(2)-ын дагуу гаралт нь ШУУД олгогдож байгаа тул "хүлээн
// авсан" мэдэгдэл шаардлагагүй — зөвхөн "гүйцэтгэл дууссан" мэдэгдэл өгнө.
func (r *govRepository) CreateApplicationWithOutput(
	ctx context.Context,
	app *domain.GovApplication,
	ref *domain.GovReference,
	notify *domain.GovNotification,
) (domain.GovApplication, domain.GovReference, error) {
	var (
		outApp domain.GovApplication
		outRef domain.GovReference
	)
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, err := insertApplication(ctx, tx, app)
		if err != nil {
			return err
		}

		if ref != nil {
			data := ref.Data
			if len(data) == 0 {
				data = []byte("{}")
			}
			rf, refErr := scanReference(tx.QueryRow(ctx,
				`INSERT INTO gov_references(user_id, type, title, reference_no, status, valid_until, data)
				 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+refColumns,
				ref.UserID, ref.Type, ref.Title, ref.ReferenceNo, ref.Status, ref.ValidUntil, data))
			if refErr != nil {
				return refErr
			}
			outRef = rf
			if _, err := tx.Exec(ctx,
				`UPDATE gov_applications SET output_ref_id = $2 WHERE id = $1`, a.ID, rf.ID); err != nil {
				return err
			}
			a.OutputRefID = &rf.ID
		}

		if notify != nil {
			if _, err := tx.Exec(ctx,
				`INSERT INTO gov_notifications(user_id, title, body, category) VALUES ($1,$2,$3,$4)`,
				notify.UserID, notify.Title, notify.Body, notify.Category); err != nil {
				return err
			}
		}

		if err := appendEventTx(ctx, tx, &domain.GovApplicationEvent{
			ApplicationID: a.ID, ActorID: &a.UserID, ActorRole: "user",
			ToStatus: a.Status, Type: "auto_fulfilled",
			Detail: "Бүртгэлээс шууд олгогдов (хүний оролцоогүй)",
		}, a.UserID); err != nil {
			return err
		}

		outApp = a
		return nil
	})
	return outApp, outRef, err
}

// SetApplicationStatus нь одоогоор зөвхөн цуцлахад (cancelled) ашиглагдана.
// Аль хэдийн шийдэгдсэн (approved/rejected/completed) эсвэл цуцлагдсан хүсэлтийг
// дахин цуцлахгүйн тулд зөвхөн идэвхтэй эх төлвөөс (submitted/in_review) шилжинэ
// (CancelAppointment/PayPayment-ийн загвартай нийцтэй).
func (r *govRepository) SetApplicationStatus(ctx context.Context, userID, id, status string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE gov_applications SET status = $3, updated_at = now()
			 WHERE id = $1 AND user_id = $2 AND status IN ('submitted','in_review')`, id, userID, status)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("active application not found")
		}
		return nil
	})
}

// ── References (per-user — withRLS) ─────────────────────────────────────────

const refColumns = `id, user_id, type, title, reference_no, status, issued_at, valid_until, data`

func scanReference(row pgx.Row) (domain.GovReference, error) {
	var ref domain.GovReference
	err := row.Scan(&ref.ID, &ref.UserID, &ref.Type, &ref.Title, &ref.ReferenceNo, &ref.Status, &ref.IssuedAt, &ref.ValidUntil, &ref.Data)
	return ref, err
}

func (r *govRepository) ListReferences(ctx context.Context, userID string) ([]domain.GovReference, error) {
	var out []domain.GovReference
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+refColumns+` FROM gov_references WHERE user_id = $1 ORDER BY issued_at DESC`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovReference, 0, 16)
		for rows.Next() {
			ref, scanErr := scanReference(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, ref)
		}
		return rows.Err()
	})
	return out, err
}

func (r *govRepository) CreateReference(ctx context.Context, in *domain.GovReference) (domain.GovReference, error) {
	data := in.Data
	if len(data) == 0 {
		data = []byte("{}")
	}
	var out domain.GovReference
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		var scanErr error
		out, scanErr = scanReference(tx.QueryRow(ctx,
			`INSERT INTO gov_references(user_id, type, title, reference_no, status, valid_until, data)
			 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+refColumns,
			in.UserID, in.Type, in.Title, in.ReferenceNo, in.Status, in.ValidUntil, data))
		return scanErr
	})
	return out, err
}

// ── Application timeline (append-only) ──────────────────────────────────────

const eventColumns = `id, application_id, actor_id, actor_role, from_status, to_status, type, detail, created_at`

func scanAppEvent(row pgx.Row) (domain.GovApplicationEvent, error) {
	var e domain.GovApplicationEvent
	err := row.Scan(&e.ID, &e.ApplicationID, &e.ActorID, &e.ActorRole,
		&e.FromStatus, &e.ToStatus, &e.Type, &e.Detail, &e.CreatedAt)
	return e, err
}

// appendEventTx нь timeline бичлэгийг ӨГӨГДСӨН транзакцид нэмнэ — төлөв
// өөрчлөлт болон түүний ул мөр атомарт үлдэхийн тулд.
func appendEventTx(ctx context.Context, tx pgx.Tx, e *domain.GovApplicationEvent, ownerID string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO gov_application_events
			(application_id, user_id, actor_id, actor_role, from_status, to_status, type, detail)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ApplicationID, ownerID, e.ActorID, e.ActorRole, e.FromStatus, e.ToStatus, e.Type, e.Detail)
	return err
}

func (r *govRepository) AppendApplicationEvent(ctx context.Context, in *domain.GovApplicationEvent) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		var ownerID string
		if err := tx.QueryRow(ctx,
			`SELECT user_id FROM gov_applications WHERE id = $1`, in.ApplicationID).Scan(&ownerID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperror.NotFound("application not found")
			}
			return err
		}
		return appendEventTx(ctx, tx, in, ownerID)
	})
}

func (r *govRepository) ListApplicationEvents(ctx context.Context, applicationID string) ([]domain.GovApplicationEvent, error) {
	var out []domain.GovApplicationEvent
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT `+eventColumns+` FROM gov_application_events
			  WHERE application_id = $1 ORDER BY created_at`, applicationID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovApplicationEvent, 0, 8)
		for rows.Next() {
			e, scanErr := scanAppEvent(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

// ── Officer queue (менежер — officer RLS үүргээр) ───────────────────────────

// openStatuses нь нээлттэй төлвүүдийн SQL хэсэг. domain.GovIsOpen-той таарна.
const openStatuses = `('submitted','registered','in_review','info_required')`

func (r *govRepository) QueueStats(ctx context.Context, officerID string) (domain.GovQueueStats, error) {
	var s domain.GovQueueStats
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT
				count(*) FILTER (WHERE status IN `+openStatuses+`),
				count(*) FILTER (WHERE status IN `+openStatuses+` AND assigned_to IS NULL),
				count(*) FILTER (WHERE status IN `+openStatuses+` AND assigned_to = $1),
				count(*) FILTER (WHERE status IN `+openStatuses+` AND due_at < now()),
				count(*) FILTER (WHERE status IN `+openStatuses+` AND due_at >= now() AND due_at < now() + interval '24 hours')
			FROM gov_applications`, officerID).
			Scan(&s.Open, &s.Unassigned, &s.Mine, &s.Overdue, &s.DueSoon)
	})
	return s, err
}

func (r *govRepository) ListQueue(ctx context.Context, f domain.GovQueueFilter) ([]domain.GovApplication, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		// Шүүлтүүрийг параметржүүлсэн NULL-шалгалтаар илэрхийлнэ — SQL-г
		// динамикаар угсрахгүй (injection-ийн гадаргууг тэг байлгана).
		var status *string
		if f.Status != "" {
			status = &f.Status
		}
		var assignee *string
		if f.AssignedTo != "" {
			assignee = &f.AssignedTo
		}
		rows, err := tx.Query(ctx, `
			SELECT `+appColumns+` FROM gov_applications
			 WHERE ($1::text IS NULL AND status IN `+openStatuses+` OR status = $1)
			   AND ($2::uuid IS NULL OR assigned_to = $2)
			   AND (NOT $3::bool OR (due_at IS NOT NULL AND due_at < now()))
			 ORDER BY due_at NULLS LAST, submitted_at
			 LIMIT $4 OFFSET $5`, status, assignee, f.Overdue, limit, f.Offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovApplication, 0, limit)
		for rows.Next() {
			a, scanErr := scanApplication(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

func (r *govRepository) GetApplicationAny(ctx context.Context, id string) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, scanErr := scanApplication(tx.QueryRow(ctx,
			`SELECT `+appColumns+` FROM gov_applications WHERE id = $1`, id))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.NotFound("application not found")
		}
		out = a
		return scanErr
	})
	return out, err
}

// AssignApplication нь хүсэлтийг менежерт оноож in_review болгоно.
//
// WHERE guard нь хоёр зүйлийг зэрэг хийнэ: (1) зөвшөөрөгдсөн эх төлвөөс л
// шилжинэ, (2) өөр менежер аль хэдийн аваагүй байх. Зэрэг ирсэн хоёр дахь
// хүсэлт 0 мөр хөндөж Conflict авна — relay-ийн батлагдсан загвар.
func (r *govRepository) AssignApplication(ctx context.Context, id, officerID string) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		// Онгойлгосон (open) БА эзэнгүй хүсэлтийг менежер өөртөө авна. in_review-г
		// мөн зөвшөөрнө: хүсэлт хянагдаж эхэлсэн ч эзэнгүй (жишээ workflow-оос өмнөх
		// хуучин мөр, эсвэл эзнээ алдсан) бол дахин эзэмшүүлэх боломжтой. assigned_to
		// guard нь өөр менежерийнхийг булаахаас сэргийлнэ.
		a, scanErr := scanApplication(tx.QueryRow(ctx, `
			UPDATE gov_applications
			   SET assigned_to = $2, assigned_at = now(), status = 'in_review', updated_at = now()
			 WHERE id = $1
			   AND status IN ('submitted','registered','in_review')
			   AND (assigned_to IS NULL OR assigned_to = $2)
			 RETURNING `+appColumns, id, officerID))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.Conflict("хүсэлт аль хэдийн авагдсан эсвэл хянах боломжгүй төлөвт байна")
		}
		if scanErr != nil {
			return scanErr
		}
		out = a
		return appendEventTx(ctx, tx, &domain.GovApplicationEvent{
			ApplicationID: a.ID, ActorID: &officerID, ActorRole: "officer",
			FromStatus: "registered", ToStatus: a.Status, Type: "assigned",
			Detail: "Менежер хүсэлтийг хянахаар авав",
		}, a.UserID)
	})
	return out, err
}

// DecideApplication нь approve/reject шийдвэрийг НЭГ транзакцид бичнэ:
// төлөв + шийдвэрлэгч + үр дүн, зөвшөөрсөн бол гаралт (лавлагаа), мэдэгдэл,
// timeline. Approve үед төлөв шууд 'completed' болно — гаралт нь тэр дороо
// олгогдож байгаа тул 'approved' завсрын төлөвт саатуулах шалтгаангүй.
func (r *govRepository) DecideApplication(ctx context.Context, in repointerface.GovDecisionInput) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, scanErr := scanApplication(tx.QueryRow(ctx, `
			UPDATE gov_applications
			   SET status = $3, result = $4, decided_by = $2, decided_at = now(),
			       decision_note = $5, updated_at = now()
			 WHERE id = $1
			   AND status IN ('registered','in_review','info_required')
			 RETURNING `+appColumns, in.ApplicationID, in.OfficerID, in.Target, in.Result, in.Note))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.Conflict("хүсэлт аль хэдийн шийдэгдсэн эсвэл шийдвэрлэх боломжгүй төлөвт байна")
		}
		if scanErr != nil {
			return scanErr
		}

		if in.OutputRef != nil {
			data := in.OutputRef.Data
			if len(data) == 0 {
				data = []byte("{}")
			}
			rf, refErr := scanReference(tx.QueryRow(ctx,
				`INSERT INTO gov_references(user_id, type, title, reference_no, status, valid_until, data)
				 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+refColumns,
				a.UserID, in.OutputRef.Type, in.OutputRef.Title, in.OutputRef.ReferenceNo,
				in.OutputRef.Status, in.OutputRef.ValidUntil, data))
			if refErr != nil {
				return refErr
			}
			if _, err := tx.Exec(ctx,
				`UPDATE gov_applications SET output_ref_id = $2 WHERE id = $1`, a.ID, rf.ID); err != nil {
				return err
			}
			a.OutputRefID = &rf.ID
		}

		if in.Notify != nil {
			if _, err := tx.Exec(ctx,
				`INSERT INTO gov_notifications(user_id, title, body, category) VALUES ($1,$2,$3,$4)`,
				a.UserID, in.Notify.Title, in.Notify.Body, in.Notify.Category); err != nil {
				return err
			}
		}

		out = a
		return appendEventTx(ctx, tx, &domain.GovApplicationEvent{
			ApplicationID: a.ID, ActorID: &in.OfficerID, ActorRole: "officer",
			ToStatus: a.Status, Type: "decided", Detail: in.Note,
		}, a.UserID)
	})
	return out, err
}

// CompleteApplication нь 'approved' (биет гаралт хэвлэгдэж/хүргэгдэхийг хүлээж
// буй) хүсэлтийг хаана. Зөвхөн ЭНЭ эх төлвөөс шилжинэ — шийдвэр гараагүй
// хүсэлтийг "хүргэсэн" гэж хаах боломжгүй.
func (r *govRepository) CompleteApplication(ctx context.Context, id, officerID string, notify *domain.GovNotification) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, scanErr := scanApplication(tx.QueryRow(ctx, `
			UPDATE gov_applications
			   SET status = 'completed', updated_at = now()
			 WHERE id = $1 AND status = 'approved'
			 RETURNING `+appColumns, id))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.Conflict("зөвшөөрөгдсөн, хүргэгдэхийг хүлээж буй хүсэлт олдсонгүй")
		}
		if scanErr != nil {
			return scanErr
		}
		if notify != nil {
			if _, err := tx.Exec(ctx,
				`INSERT INTO gov_notifications(user_id, title, body, category) VALUES ($1,$2,$3,$4)`,
				a.UserID, notify.Title, notify.Body, notify.Category); err != nil {
				return err
			}
		}
		out = a
		return appendEventTx(ctx, tx, &domain.GovApplicationEvent{
			ApplicationID: a.ID, ActorID: &officerID, ActorRole: "officer",
			ToStatus: a.Status, Type: "delivered", Detail: "Гаралт хүргэгдэв",
		}, a.UserID)
	})
	return out, err
}

// RequestMoreInfo нь info_required руу шилжүүлж SLA ЦАГИЙГ ЗОГСООНО
// (suspended_at тамгална). Эрх зүйн үндэслэл: хугацаа нь бүх баримт бүрдсэн
// үеэс л явах ёстой — иргэний удаашрал байгууллагын зөрчил болж бүртгэгдэхгүй.
func (r *govRepository) RequestMoreInfo(ctx context.Context, id, officerID, note string) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, scanErr := scanApplication(tx.QueryRow(ctx, `
			UPDATE gov_applications
			   SET status = 'info_required', suspended_at = now(),
			       decision_note = $3, updated_at = now()
			 WHERE id = $1 AND status IN ('registered','in_review')
			 RETURNING `+appColumns, id, officerID, note))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.Conflict("хүсэлт нэмэлт мэдээлэл хүсэх боломжгүй төлөвт байна")
		}
		if scanErr != nil {
			return scanErr
		}
		out = a
		return appendEventTx(ctx, tx, &domain.GovApplicationEvent{
			ApplicationID: a.ID, ActorID: &officerID, ActorRole: "officer",
			ToStatus: a.Status, Type: "info_requested", Detail: note,
		}, a.UserID)
	})
	return out, err
}

// ResumeFromInfo нь иргэн баримтаа нэмсний дараа цагийг ҮРГЭЛЖЛҮҮЛНЭ: due_at-г
// зогссон хугацааны туршид ХОЙШЛУУЛЖ, suspended_at-г цэвэрлэнэ.
func (r *govRepository) ResumeFromInfo(ctx context.Context, userID, id string) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		a, scanErr := scanApplication(tx.QueryRow(ctx, `
			UPDATE gov_applications
			   SET status = 'in_review',
			       due_at = CASE
			           WHEN due_at IS NOT NULL AND suspended_at IS NOT NULL
			           THEN due_at + (now() - suspended_at)
			           ELSE due_at
			       END,
			       suspended_at = NULL,
			       updated_at = now()
			 WHERE id = $1 AND user_id = $2 AND status = 'info_required'
			 RETURNING `+appColumns, id, userID))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.Conflict("хүсэлт нэмэлт мэдээлэл хүлээж буй төлөвт байхгүй")
		}
		if scanErr != nil {
			return scanErr
		}
		out = a
		return appendEventTx(ctx, tx, &domain.GovApplicationEvent{
			ApplicationID: a.ID, ActorID: &userID, ActorRole: "user",
			ToStatus: a.Status, Type: "info_provided",
			Detail: "Иргэн нэмэлт мэдээлэл ирүүлэв — хугацаа үргэлжлэв",
		}, a.UserID)
	})
	return out, err
}

// MarkSLABreached нь хугацаа хэтэрсэн ч хараахан тэмдэглэгдээгүй хүсэлтүүдийг
// нэг удаа тэмдэглэж буцаана. `AND NOT sla_breached` нь latch — sweep давтан
// ажиллахад нэг хүсэлт дахин мэдэгдэл үүсгэхгүй.
//
// Цаг зогссон (info_required) хүсэлтүүдийг ОРХИНО — тэдгээрийн хугацаа
// иргэний хариу хүлээж байгаа тул явахгүй.
func (r *govRepository) MarkSLABreached(ctx context.Context) ([]domain.GovApplication, error) {
	var out []domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE gov_applications
			   SET sla_breached = true, updated_at = now()
			 WHERE status IN ('submitted','registered','in_review')
			   AND suspended_at IS NULL
			   AND due_at IS NOT NULL AND due_at < now()
			   AND NOT sla_breached
			 RETURNING `+appColumns)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovApplication, 0, 8)
		for rows.Next() {
			a, scanErr := scanApplication(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

// TacitApprovals нь чимээгүй зөвшөөрөл идэвхжсэн үйлчилгээний хугацаа хэтэрсэн
// хүсэлтүүдийг зөвшөөрөгдсөнд тооцно. tacit = true болгож тэмдэглэх нь чухал —
// шийдвэр АВТОМАТААР гарсныг иргэнд ил мэдэгдэх үүрэгтэй (Эстонийн жишиг).
func (r *govRepository) TacitApprovals(ctx context.Context) ([]domain.GovApplication, error) {
	var out []domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE gov_applications a
			   SET status = 'completed', result = 'granted', tacit = true,
			       decided_at = now(), updated_at = now(),
			       decision_note = 'Хуулийн хугацаанд шийдвэр гараагүй тул зөвшөөрсөнд тооцов'
			  FROM gov_services s
			 WHERE s.id = a.service_id
			   AND s.tacit_approval
			   AND a.status IN ('registered','in_review')
			   AND a.suspended_at IS NULL
			   AND a.due_at IS NOT NULL AND a.due_at < now()
			 RETURNING `+prefixed(appColumns, "a"))
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovApplication, 0, 4)
		for rows.Next() {
			app, scanErr := scanApplication(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, app)
		}
		return rows.Err()
	})
	return out, err
}

// prefixed нь баганын жагсаалтад alias угтвар нэмнэ (UPDATE ... FROM дахь
// RETURNING-д хоёрдмол утгатай нэрийг тодруулах шаардлагатай).
func prefixed(cols, alias string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

// ── Notifications (per-user — withRLS) ──────────────────────────────────────

// CreateNotification нь иргэнд мэдэгдэл бичнэ. Менежер/систем нь өөрийнх нь биш
// хэрэглэгчид бичих тул officer эсвэл service RLS үүрэг шаардана.
func (r *govRepository) CreateNotification(ctx context.Context, in *domain.GovNotification) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO gov_notifications(user_id, title, body, category) VALUES ($1,$2,$3,$4)`,
			in.UserID, in.Title, in.Body, in.Category)
		return err
	})
}

const notifColumns = `id, user_id, title, body, category, read, created_at`

func (r *govRepository) ListNotifications(ctx context.Context, userID string) ([]domain.GovNotification, error) {
	var out []domain.GovNotification
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+notifColumns+` FROM gov_notifications WHERE user_id = $1 ORDER BY created_at DESC`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovNotification, 0, 16)
		for rows.Next() {
			var n domain.GovNotification
			if err := rows.Scan(&n.ID, &n.UserID, &n.Title, &n.Body, &n.Category, &n.Read, &n.CreatedAt); err != nil {
				return err
			}
			out = append(out, n)
		}
		return rows.Err()
	})
	return out, err
}

func (r *govRepository) MarkNotificationRead(ctx context.Context, userID, id string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE gov_notifications SET read = true WHERE id = $1 AND user_id = $2`, id, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("notification not found")
		}
		return nil
	})
}

func (r *govRepository) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE gov_notifications SET read = true WHERE user_id = $1 AND NOT read`, userID)
		return err
	})
}

// ── Payments (per-user — withRLS) ───────────────────────────────────────────

const payColumns = `id, user_id, title, category, amount, currency, status, due_date, paid_at, created_at`

func scanPayment(row pgx.Row) (domain.GovPayment, error) {
	var p domain.GovPayment
	err := row.Scan(&p.ID, &p.UserID, &p.Title, &p.Category, &p.Amount, &p.Currency, &p.Status, &p.DueDate, &p.PaidAt, &p.CreatedAt)
	return p, err
}

func (r *govRepository) ListPayments(ctx context.Context, userID string) ([]domain.GovPayment, error) {
	var out []domain.GovPayment
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+payColumns+` FROM gov_payments WHERE user_id = $1 ORDER BY status, created_at DESC`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovPayment, 0, 16)
		for rows.Next() {
			p, scanErr := scanPayment(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func (r *govRepository) PayPayment(ctx context.Context, userID, id string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE gov_payments SET status = 'paid', paid_at = now() WHERE id = $1 AND user_id = $2 AND status = 'pending'`, id, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("pending payment not found")
		}
		return nil
	})
}

// ── Appointments (per-user — withRLS) ───────────────────────────────────────

const apptColumns = `id, user_id, service_id, service_name, agency, location, scheduled_at, status, note, created_at`

func scanAppointment(row pgx.Row) (domain.GovAppointment, error) {
	var a domain.GovAppointment
	err := row.Scan(&a.ID, &a.UserID, &a.ServiceID, &a.ServiceName, &a.Agency, &a.Location, &a.ScheduledAt, &a.Status, &a.Note, &a.CreatedAt)
	return a, err
}

func (r *govRepository) ListAppointments(ctx context.Context, userID string) ([]domain.GovAppointment, error) {
	var out []domain.GovAppointment
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+apptColumns+` FROM gov_appointments WHERE user_id = $1 ORDER BY scheduled_at`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		out = make([]domain.GovAppointment, 0, 16)
		for rows.Next() {
			a, scanErr := scanAppointment(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

func (r *govRepository) CreateAppointment(ctx context.Context, in *domain.GovAppointment) (domain.GovAppointment, error) {
	var out domain.GovAppointment
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		var scanErr error
		out, scanErr = scanAppointment(tx.QueryRow(ctx,
			`INSERT INTO gov_appointments(user_id, service_id, service_name, agency, location, scheduled_at, status, note)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+apptColumns,
			in.UserID, in.ServiceID, in.ServiceName, in.Agency, in.Location, in.ScheduledAt, in.Status, in.Note))
		return scanErr
	})
	return out, err
}

func (r *govRepository) CancelAppointment(ctx context.Context, userID, id string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE gov_appointments SET status = 'cancelled' WHERE id = $1 AND user_id = $2 AND status IN ('booked','confirmed')`, id, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("active appointment not found")
		}
		return nil
	})
}

// ── Overview / seed (per-user — withRLS) ────────────────────────────────────

func (r *govRepository) Overview(ctx context.Context, userID string) (domain.GovOverview, error) {
	var o domain.GovOverview
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM gov_applications WHERE user_id = $1 AND status IN ('submitted','in_review')),
				(SELECT count(*) FROM gov_notifications WHERE user_id = $1 AND NOT read),
				(SELECT count(*) FROM gov_payments WHERE user_id = $1 AND status = 'pending'),
				(SELECT COALESCE(sum(amount),0) FROM gov_payments WHERE user_id = $1 AND status = 'pending'),
				(SELECT count(*) FROM gov_appointments WHERE user_id = $1 AND status IN ('booked','confirmed') AND scheduled_at >= now()),
				(SELECT count(*) FROM gov_references WHERE user_id = $1 AND status = 'issued')
		`, userID).Scan(&o.OpenApplications, &o.UnreadNotifications, &o.UnpaidCount, &o.UnpaidAmount, &o.UpcomingCount, &o.IssuedReferences); err != nil {
			return err
		}

		apps, err := recentApplications(ctx, tx, userID)
		if err != nil {
			return err
		}
		o.RecentApplications = apps

		appts, err := upcomingAppointments(ctx, tx, userID)
		if err != nil {
			return err
		}
		o.UpcomingAppointments = appts
		return nil
	})
	if err != nil {
		return domain.GovOverview{}, err
	}
	return o, nil
}

func recentApplications(ctx context.Context, tx pgx.Tx, userID string) ([]domain.GovApplication, error) {
	rows, err := tx.Query(ctx, `SELECT `+appColumns+` FROM gov_applications WHERE user_id = $1 ORDER BY submitted_at DESC LIMIT 5`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GovApplication, 0, 5)
	for rows.Next() {
		a, scanErr := scanApplication(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func upcomingAppointments(ctx context.Context, tx pgx.Tx, userID string) ([]domain.GovAppointment, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+apptColumns+` FROM gov_appointments WHERE user_id = $1 AND status IN ('booked','confirmed') AND scheduled_at >= now() ORDER BY scheduled_at LIMIT 5`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.GovAppointment, 0, 5)
	for rows.Next() {
		a, scanErr := scanAppointment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *govRepository) CountUserRows(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM gov_applications WHERE user_id = $1) +
				(SELECT count(*) FROM gov_references WHERE user_id = $1) +
				(SELECT count(*) FROM gov_notifications WHERE user_id = $1) +
				(SELECT count(*) FROM gov_payments WHERE user_id = $1) +
				(SELECT count(*) FROM gov_appointments WHERE user_id = $1)
		`, userID).Scan(&n)
	})
	return n, err
}

// SeedDemoData нь хэрэглэгчид анх ороход жишээ өгөгдөл (мэдэгдэл/төлбөр/хүсэлт/
// лавлагаа/цаг) нэг withRLS транзакцид үүсгэнэ. RLS бодлого нь user_id =
// app.user_id-г WITH CHECK-ээр баталгаажуулах тул seed нь зөвхөн тухайн
// хэрэглэгчийн мөр бичнэ.
//
// Транзакц-скоуптай advisory lock (хэрэглэгчээр) нь анх ороход зэрэг ирсэн хоёр
// хүсэлт хоёулаа seed хийж давхар мөр үүсгэхээс (TOCTOU) сэргийлнэ: эхнийх нь
// lock авч seed хийгээд commit-д гарган lock-оо суллана, хоёр дахь нь lock авмагц
// доорх дахин-шалгалтаар мөр аль хэдийн байгааг олж чимээгүй буцна.
func (r *govRepository) SeedDemoData(ctx context.Context, userID string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, userID); err != nil {
			return err
		}
		// Lock дор дахин шалгана — өөр хүсэлт аль хэдийн seed хийсэн байж болно.
		var existing int
		if err := tx.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM gov_applications  WHERE user_id = $1) +
				(SELECT count(*) FROM gov_references     WHERE user_id = $1) +
				(SELECT count(*) FROM gov_notifications  WHERE user_id = $1) +
				(SELECT count(*) FROM gov_payments       WHERE user_id = $1) +
				(SELECT count(*) FROM gov_appointments   WHERE user_id = $1)
		`, userID).Scan(&existing); err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO gov_notifications(user_id, title, body, category, read, created_at) VALUES
			($1, 'Татварын тодорхойлолт бэлэн боллоо', 'Таны хүссэн татварын тодорхойлолт амжилттай олгогдлоо.', 'success', false, now() - interval '2 hours'),
			($1, 'Иргэний үнэмлэхний хугацаа дуусч байна', 'Таны иргэний үнэмлэхний хугацаа 30 хоногийн дотор дуусна.', 'warning', false, now() - interval '1 day'),
			($1, 'Нийгмийн даатгалын шимтгэл', '2026 оны 5-р сарын шимтгэл амжилттай төлөгдлөө.', 'info', true, now() - interval '6 days')
		`, userID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO gov_payments(user_id, title, category, amount, status, due_date, created_at) VALUES
			($1, 'Авто тээврийн татвар 2026', 'tax', 45000, 'pending', now() + interval '20 days', now() - interval '3 days'),
			($1, 'Жолооны үнэмлэх сунгалтын хураамж', 'fee', 35000, 'pending', now() + interval '10 days', now() - interval '1 day'),
			($1, 'Зам хөдөлгөөний торгууль', 'fine', 30000, 'paid', now() - interval '15 days', now() - interval '20 days')
		`, userID); err != nil {
			return err
		}

		// ХҮСЭЛТ болон ЛАВЛАГААГ ЗОРИУДААР seed хийхээ БОЛИВ.
		//
		// Өмнө нь энд 'approved' / 'in_review' төлөвтэй хуурамч хүсэлт, олгогдсон
		// лавлагаа үүсгэдэг байсан бөгөөд иргэний харагдац бүхэлдээ түүнээс
		// бүрддэг байлаа — үйлчилгээ "ажиллаж байгаа" мэт харагдах ч ард нь
		// ямар ч урсгал байгаагүй. Одоо хүсэлт нь бодит workflow-оор л үүснэ:
		// auto үйлчилгээ шууд биелж лавлагаа олгоно, manual нь менежерийн
		// дараалалд орно. Хуурамч мөр үлдээвэл timeline-гүй, service_id-гүй,
		// due_at-гүй "өнчин" бичлэг болж бодит урсгалыг будлиулна.
		//
		// Доорх мэдэгдэл/төлбөр/цаг захиалга нь тусдаа модулиуд тул хэвээр —
		// эдгээр нь хүсэлтийн workflow-д оролцдоггүй.
		if _, err := tx.Exec(ctx, `
			INSERT INTO gov_appointments(user_id, service_name, agency, location, scheduled_at, status, note) VALUES
			($1, 'Жолооны үнэмлэх сунгах', 'Зам тээврийн төв', 'БЗД, 13-р хороо', now() + interval '5 days' + interval '10 hours', 'booked', '')
		`, userID); err != nil {
			return err
		}

		return nil
	})
}

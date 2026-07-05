// Gerege Template Version 27.0
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
	defer tx.Rollback(ctx) //nolint:errcheck

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

const serviceColumns = `id, code, name, category, agency, description, fee, processing_days, online, enabled, created_at`

func scanService(row pgx.Row) (domain.GovService, error) {
	var s domain.GovService
	err := row.Scan(&s.ID, &s.Code, &s.Name, &s.Category, &s.Agency, &s.Description,
		&s.Fee, &s.ProcessingDays, &s.Online, &s.Enabled, &s.CreatedAt)
	return s, err
}

func (r *govRepository) ListServices(ctx context.Context) ([]domain.GovService, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serviceColumns+` FROM gov_services WHERE enabled ORDER BY category, name`)
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
	return out, rows.Err()
}

func (r *govRepository) GetService(ctx context.Context, id string) (domain.GovService, error) {
	s, err := scanService(r.pool.QueryRow(ctx, `SELECT `+serviceColumns+` FROM gov_services WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GovService{}, apperror.NotFound("service not found")
	}
	return s, err
}

// ── Applications (per-user — withRLS) ───────────────────────────────────────

func scanApplication(row pgx.Row) (domain.GovApplication, error) {
	var a domain.GovApplication
	err := row.Scan(&a.ID, &a.UserID, &a.ServiceID, &a.ServiceName, &a.ReferenceNo, &a.Status, &a.Note, &a.SubmittedAt, &a.UpdatedAt)
	return a, err
}

const appColumns = `id, user_id, service_id, service_name, reference_no, status, note, submitted_at, updated_at`

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

func (r *govRepository) CreateApplication(ctx context.Context, in *domain.GovApplication) (domain.GovApplication, error) {
	var out domain.GovApplication
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		var scanErr error
		out, scanErr = scanApplication(tx.QueryRow(ctx,
			`INSERT INTO gov_applications(user_id, service_id, service_name, reference_no, status, note)
			 VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+appColumns,
			in.UserID, in.ServiceID, in.ServiceName, in.ReferenceNo, in.Status, in.Note))
		return scanErr
	})
	return out, err
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

// ── Notifications (per-user — withRLS) ──────────────────────────────────────

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

		if _, err := tx.Exec(ctx, `
			INSERT INTO gov_applications(user_id, service_name, reference_no, status, note, submitted_at) VALUES
			($1, 'Татварын тодорхойлолт', 'APP-2026-100231', 'approved', 'Банкны зээлд шаардлагатай.', now() - interval '4 days'),
			($1, 'Оршин суугаа газрын лавлагаа', 'APP-2026-100487', 'in_review', '', now() - interval '1 day')
		`, userID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO gov_references(user_id, type, title, reference_no, status, issued_at, valid_until) VALUES
			($1, 'residence', 'Оршин суугаа газрын лавлагаа', 'REF-2026-55012', 'issued', now() - interval '10 days', now() + interval '20 days'),
			($1, 'social_ins', 'Нийгмийн даатгалын лавлагаа', 'REF-2026-55890', 'issued', now() - interval '40 days', now() - interval '10 days')
		`, userID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO gov_appointments(user_id, service_name, agency, location, scheduled_at, status, note) VALUES
			($1, 'Жолооны үнэмлэх сунгах', 'Зам тээврийн төв', 'БЗД, 13-р хороо', now() + interval '5 days' + interval '10 hours', 'booked', '')
		`, userID); err != nil {
			return err
		}

		return nil
	})
}

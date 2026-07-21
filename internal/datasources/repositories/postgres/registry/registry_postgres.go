// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package registry нь Ring System · R1 — Үйлчилгээний нэгдсэн регистрийн
// Postgres gateway (migration 42). Эдгээр хүснэгтүүд нь байгууллагын мастер
// өгөгдөл (хэрэглэгч-тус-бүрийн БИШ) тул gateway/relay-ийн адил RLS-гүй;
// хамгаалалт нь route давхаргад 'registry.manage' эрхээр хийгдэнэ.
//
// ORM-гүй: бүх query нь гараар бичсэн параметржүүлсэн SQL.
package registry

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres-ийн SQLSTATE кодууд (rbac/org repo-той ижил загвар).
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// isPgErr нь алдаа тухайн SQLSTATE кодтой Postgres алдаа эсэхийг шалгана.
func isPgErr(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func isUniqueViolation(err error) bool     { return isPgErr(err, pgUniqueViolation) }
func isForeignKeyViolation(err error) bool { return isPgErr(err, pgForeignKeyViolation) }

type registryRepository struct {
	pool *pgxpool.Pool
}

func NewRegistryRepository(pool *pgxpool.Pool) repointerface.RegistryRepository {
	return &registryRepository{pool: pool}
}

// ── Паспорт ─────────────────────────────────────────────────────────────────

const serviceColumns = `id, code, name, name_en, description, authority, authority_org_id, legal_basis,
	target_group, output, channels, fee, max_days, steps_count, annual_volume, proactivity, status,
	life_event_id, version, published_at, created_at, updated_at`

func scanService(row pgx.Row) (domain.RegistryService, error) {
	var s domain.RegistryService
	err := row.Scan(&s.ID, &s.Code, &s.Name, &s.NameEN, &s.Description, &s.Authority, &s.AuthorityOrgID,
		&s.LegalBasis, &s.TargetGroup, &s.Output, &s.Channels, &s.Fee, &s.MaxDays, &s.StepsCount,
		&s.AnnualVolume, &s.Proactivity, &s.Status, &s.LifeEventID, &s.Version, &s.PublishedAt,
		&s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (r *registryRepository) ListServices(ctx context.Context, f repointerface.RegistryFilter) ([]domain.RegistryService, error) {
	// Шүүлтүүрийг ЗӨВХӨН байрлалын параметрээр угсарна — хэрэглэгчийн утга
	// хэзээ ч SQL текстэд ордоггүй тул injection боломжгүй. Хоосон талбар нь
	// тухайн нөхцөлийг бүхэлд нь алгасна.
	var (
		where = make([]string, 0, 5)
		args  = make([]any, 0, 5)
	)
	// ph нь аргументыг нэмээд түүний "$N" placeholder-ийг буцаана.
	ph := func(val any) string {
		args = append(args, val)
		return "$" + strconv.Itoa(len(args))
	}
	if f.PublishedOnly {
		where = append(where, `status = 'published'`)
	} else if f.Status != "" {
		where = append(where, `status = `+ph(f.Status))
	}
	if f.Authority != "" {
		where = append(where, `authority = `+ph(f.Authority))
	}
	if f.LifeEventID != "" {
		where = append(where, `life_event_id = `+ph(f.LifeEventID))
	}
	if f.Proactivity != "" {
		where = append(where, `proactivity = `+ph(f.Proactivity))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		// Нэг аргументыг хоёр баганад ашиглана (ижил $N).
		p := ph(q)
		where = append(where, `(name ILIKE '%' || `+p+` || '%' OR code ILIKE '%' || `+p+` || '%')`)
	}

	sql := `SELECT ` + serviceColumns + ` FROM registry_services`
	if len(where) > 0 {
		sql += ` WHERE ` + strings.Join(where, ` AND `)
	}
	sql += ` ORDER BY status, name`

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RegistryService, 0, 32)
	for rows.Next() {
		s, scanErr := scanService(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *registryRepository) GetService(ctx context.Context, id string) (domain.RegistryService, error) {
	s, err := scanService(r.pool.QueryRow(ctx, `SELECT `+serviceColumns+` FROM registry_services WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RegistryService{}, apperror.NotFound("service not found")
	}
	if err != nil {
		return domain.RegistryService{}, err
	}
	ev, err := r.serviceEvidences(ctx, s.ID)
	if err != nil {
		return domain.RegistryService{}, err
	}
	s.Evidences = ev
	return s, nil
}

func (r *registryRepository) GetServiceByCode(ctx context.Context, code string) (domain.RegistryService, error) {
	s, err := scanService(r.pool.QueryRow(ctx, `SELECT `+serviceColumns+` FROM registry_services WHERE code = $1`, code))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RegistryService{}, apperror.NotFound("service not found")
	}
	if err != nil {
		return domain.RegistryService{}, err
	}
	ev, err := r.serviceEvidences(ctx, s.ID)
	if err != nil {
		return domain.RegistryService{}, err
	}
	s.Evidences = ev
	return s, nil
}

func (r *registryRepository) CreateService(ctx context.Context, in *domain.RegistryService) (domain.RegistryService, error) {
	s, err := scanService(r.pool.QueryRow(ctx, `
		INSERT INTO registry_services
			(code, name, name_en, description, authority, authority_org_id, legal_basis, target_group,
			 output, channels, fee, max_days, steps_count, annual_volume, proactivity, status, life_event_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING `+serviceColumns,
		in.Code, in.Name, in.NameEN, in.Description, in.Authority, in.AuthorityOrgID, in.LegalBasis,
		in.TargetGroup, in.Output, in.Channels, in.Fee, in.MaxDays, in.StepsCount, in.AnnualVolume,
		in.Proactivity, in.Status, in.LifeEventID))
	if isUniqueViolation(err) {
		return domain.RegistryService{}, apperror.Conflict("service code already exists")
	}
	return s, err
}

func (r *registryRepository) UpdateService(ctx context.Context, in *domain.RegistryService) (domain.RegistryService, error) {
	s, err := scanService(r.pool.QueryRow(ctx, `
		UPDATE registry_services SET
			name = $2, name_en = $3, description = $4, authority = $5, authority_org_id = $6,
			legal_basis = $7, target_group = $8, output = $9, channels = $10, fee = $11,
			max_days = $12, steps_count = $13, annual_volume = $14, proactivity = $15,
			life_event_id = $16, updated_at = now()
		WHERE id = $1
		RETURNING `+serviceColumns,
		in.ID, in.Name, in.NameEN, in.Description, in.Authority, in.AuthorityOrgID, in.LegalBasis,
		in.TargetGroup, in.Output, in.Channels, in.Fee, in.MaxDays, in.StepsCount, in.AnnualVolume,
		in.Proactivity, in.LifeEventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RegistryService{}, apperror.NotFound("service not found")
	}
	return s, err
}

func (r *registryRepository) SetServiceStatus(ctx context.Context, id, status string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE registry_services SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("service not found")
	}
	return nil
}

func (r *registryRepository) DeleteService(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM registry_services WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("service not found")
	}
	return nil
}

// ── Паспорт ↔ нотолгоо ──────────────────────────────────────────────────────

func (r *registryRepository) serviceEvidences(ctx context.Context, serviceID string) ([]domain.RegistryServiceEvidence, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.code, e.name, se.required, se.from_citizen, e.in_khur, se.note
		FROM registry_service_evidences se
		JOIN registry_evidences e ON e.id = se.evidence_id
		WHERE se.service_id = $1
		ORDER BY e.name`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RegistryServiceEvidence, 0, 8)
	for rows.Next() {
		var e domain.RegistryServiceEvidence
		if err := rows.Scan(&e.EvidenceID, &e.Code, &e.Name, &e.Required, &e.FromCitizen, &e.InKHUR, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetServiceEvidences нь бүрэн жагсаалтыг НЭГ транзакцид солино: бүтэлгүйтвэл
// хуучин жагсаалт бүрэн бүтэн үлдэнэ (хагас устгагдсан төлөв үүсэхгүй).
func (r *registryRepository) SetServiceEvidences(ctx context.Context, serviceID string, list []domain.RegistryServiceEvidence) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit-ийн дараах rollback нь ErrTxClosed — хүлээгдсэн

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM registry_services WHERE id = $1)`, serviceID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apperror.NotFound("service not found")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM registry_service_evidences WHERE service_id = $1`, serviceID); err != nil {
		return err
	}
	for _, e := range list {
		if _, err := tx.Exec(ctx, `
			INSERT INTO registry_service_evidences (service_id, evidence_id, required, from_citizen, note)
			VALUES ($1,$2,$3,$4,$5)`,
			serviceID, e.EvidenceID, e.Required, e.FromCitizen, e.Note); err != nil {
			if isForeignKeyViolation(err) {
				return apperror.BadRequest("unknown evidence: " + e.EvidenceID)
			}
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *registryRepository) CountCitizenDocuments(ctx context.Context, serviceID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM registry_service_evidences WHERE service_id = $1 AND from_citizen`, serviceID).Scan(&n)
	return n, err
}

// ── Хувилбар ────────────────────────────────────────────────────────────────

const versionColumns = `id, service_id, version, snapshot, change_note, is_baseline, steps_count,
	documents_count, max_days, fee, delta_steps, delta_documents, delta_days, delta_fee,
	published_at, published_by`

func scanVersion(row pgx.Row) (domain.RegistryServiceVersion, error) {
	var v domain.RegistryServiceVersion
	err := row.Scan(&v.ID, &v.ServiceID, &v.Version, &v.Snapshot, &v.ChangeNote, &v.IsBaseline,
		&v.StepsCount, &v.DocumentsCount, &v.MaxDays, &v.Fee, &v.DeltaSteps, &v.DeltaDocuments,
		&v.DeltaDays, &v.DeltaFee, &v.PublishedAt, &v.PublishedBy)
	return v, err
}

func (r *registryRepository) ListVersions(ctx context.Context, serviceID string) ([]domain.RegistryServiceVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+versionColumns+` FROM registry_service_versions WHERE service_id = $1 ORDER BY version DESC`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RegistryServiceVersion, 0, 8)
	for rows.Next() {
		v, scanErr := scanVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *registryRepository) BaselineVersion(ctx context.Context, serviceID string) (domain.RegistryServiceVersion, error) {
	v, err := scanVersion(r.pool.QueryRow(ctx,
		`SELECT `+versionColumns+` FROM registry_service_versions
		 WHERE service_id = $1 AND is_baseline ORDER BY version LIMIT 1`, serviceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RegistryServiceVersion{}, apperror.NotFound("baseline not found")
	}
	return v, err
}

// PublishVersion нь хувилбарын дугаарыг DB дотор атомаар олгож (COALESCE(max)+1),
// мөрийг оруулаад registry_services-ийг НЭГ транзакцид шинэчилнэ — зэрэгцээ хоёр
// нийтлэлт ижил дугаар авахгүй (service_id, version дээр UNIQUE).
func (r *registryRepository) PublishVersion(ctx context.Context, in *domain.RegistryServiceVersion) (domain.RegistryServiceVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RegistryServiceVersion{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit-ийн дараах rollback нь ErrTxClosed — хүлээгдсэн

	// Зэрэгцээ нийтлэлтийг цувуулна (мөрийн түгжээ).
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM registry_services WHERE id = $1 FOR UPDATE)`, in.ServiceID).Scan(&exists); err != nil {
		return domain.RegistryServiceVersion{}, err
	}
	if !exists {
		return domain.RegistryServiceVersion{}, apperror.NotFound("service not found")
	}

	snapshot := in.Snapshot
	if len(snapshot) == 0 {
		snapshot = []byte(`{}`)
	}
	v, err := scanVersion(tx.QueryRow(ctx, `
		INSERT INTO registry_service_versions
			(service_id, version, snapshot, change_note, is_baseline, steps_count, documents_count,
			 max_days, fee, delta_steps, delta_documents, delta_days, delta_fee, published_by)
		VALUES ($1,
			(SELECT COALESCE(max(version), 0) + 1 FROM registry_service_versions WHERE service_id = $1),
			$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING `+versionColumns,
		in.ServiceID, snapshot, in.ChangeNote, in.IsBaseline, in.StepsCount, in.DocumentsCount,
		in.MaxDays, in.Fee, in.DeltaSteps, in.DeltaDocuments, in.DeltaDays, in.DeltaFee, in.PublishedBy))
	if err != nil {
		return domain.RegistryServiceVersion{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE registry_services
		SET version = $2, status = 'published', published_at = now(), updated_at = now()
		WHERE id = $1`, in.ServiceID, v.Version); err != nil {
		return domain.RegistryServiceVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RegistryServiceVersion{}, err
	}
	return v, nil
}

// ── Нотолгооны каталог ──────────────────────────────────────────────────────

const evidenceColumns = `id, code, name, description, holder_agency, source_system, in_khur,
	khur_service_code, created_at, updated_at`

func scanEvidence(row pgx.Row) (domain.RegistryEvidence, error) {
	var e domain.RegistryEvidence
	err := row.Scan(&e.ID, &e.Code, &e.Name, &e.Description, &e.HolderAgency, &e.SourceSystem,
		&e.InKHUR, &e.KHURServiceCode, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func (r *registryRepository) ListEvidences(ctx context.Context) ([]domain.RegistryEvidence, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+evidenceColumns+` FROM registry_evidences ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RegistryEvidence, 0, 32)
	for rows.Next() {
		e, scanErr := scanEvidence(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *registryRepository) GetEvidence(ctx context.Context, id string) (domain.RegistryEvidence, error) {
	e, err := scanEvidence(r.pool.QueryRow(ctx, `SELECT `+evidenceColumns+` FROM registry_evidences WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RegistryEvidence{}, apperror.NotFound("evidence not found")
	}
	return e, err
}

func (r *registryRepository) CreateEvidence(ctx context.Context, in *domain.RegistryEvidence) (domain.RegistryEvidence, error) {
	e, err := scanEvidence(r.pool.QueryRow(ctx, `
		INSERT INTO registry_evidences (code, name, description, holder_agency, source_system, in_khur, khur_service_code)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+evidenceColumns,
		in.Code, in.Name, in.Description, in.HolderAgency, in.SourceSystem, in.InKHUR, in.KHURServiceCode))
	if isUniqueViolation(err) {
		return domain.RegistryEvidence{}, apperror.Conflict("evidence code already exists")
	}
	return e, err
}

func (r *registryRepository) UpdateEvidence(ctx context.Context, in *domain.RegistryEvidence) (domain.RegistryEvidence, error) {
	e, err := scanEvidence(r.pool.QueryRow(ctx, `
		UPDATE registry_evidences SET
			name = $2, description = $3, holder_agency = $4, source_system = $5,
			in_khur = $6, khur_service_code = $7, updated_at = now()
		WHERE id = $1
		RETURNING `+evidenceColumns,
		in.ID, in.Name, in.Description, in.HolderAgency, in.SourceSystem, in.InKHUR, in.KHURServiceCode))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RegistryEvidence{}, apperror.NotFound("evidence not found")
	}
	return e, err
}

func (r *registryRepository) DeleteEvidence(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM registry_evidences WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("evidence not found")
	}
	return nil
}

// ── Амьдралын үйл явдал ─────────────────────────────────────────────────────

const lifeEventColumns = `id, code, name, kind, description, lead_agency, sort_order, created_at`

func scanLifeEvent(row pgx.Row) (domain.RegistryLifeEvent, error) {
	var l domain.RegistryLifeEvent
	err := row.Scan(&l.ID, &l.Code, &l.Name, &l.Kind, &l.Description, &l.LeadAgency, &l.SortOrder, &l.CreatedAt)
	return l, err
}

func (r *registryRepository) ListLifeEvents(ctx context.Context) ([]domain.RegistryLifeEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+lifeEventColumns+` FROM registry_life_events ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RegistryLifeEvent, 0, 16)
	for rows.Next() {
		l, scanErr := scanLifeEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *registryRepository) CreateLifeEvent(ctx context.Context, in *domain.RegistryLifeEvent) (domain.RegistryLifeEvent, error) {
	l, err := scanLifeEvent(r.pool.QueryRow(ctx, `
		INSERT INTO registry_life_events (code, name, kind, description, lead_agency, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+lifeEventColumns,
		in.Code, in.Name, in.Kind, in.Description, in.LeadAgency, in.SortOrder))
	if isUniqueViolation(err) {
		return domain.RegistryLifeEvent{}, apperror.Conflict("life event code already exists")
	}
	return l, err
}

func (r *registryRepository) DeleteLifeEvent(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM registry_life_events WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("life event not found")
	}
	return nil
}

// ── Once-only + нэгтгэл ─────────────────────────────────────────────────────

func (r *registryRepository) OnceOnlyViolations(ctx context.Context, authority string) ([]domain.RegistryOnceOnlyViolation, error) {
	sql := `SELECT service_id, service_code, service_name, authority, service_status,
	               evidence_id, evidence_code, evidence_name, holder_agency, khur_service_code, annual_volume
	        FROM registry_once_only_violations`
	args := []any{}
	if authority != "" {
		sql += ` WHERE authority = $1`
		args = append(args, authority)
	}
	sql += ` ORDER BY annual_volume DESC, service_name, evidence_name`

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.RegistryOnceOnlyViolation, 0, 32)
	for rows.Next() {
		var v domain.RegistryOnceOnlyViolation
		if err := rows.Scan(&v.ServiceID, &v.ServiceCode, &v.ServiceName, &v.Authority, &v.ServiceStatus,
			&v.EvidenceID, &v.EvidenceCode, &v.EvidenceName, &v.HolderAgency, &v.KHURServiceCode,
			&v.AnnualVolume); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *registryRepository) Overview(ctx context.Context) (domain.RegistryOverview, error) {
	var o domain.RegistryOverview
	o.ByProactivity = map[string]int{}

	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM registry_services),
			(SELECT count(*) FROM registry_services WHERE status = 'published'),
			(SELECT count(*) FROM registry_services WHERE status = 'draft'),
			(SELECT count(*) FROM registry_life_events),
			(SELECT count(*) FROM registry_evidences),
			(SELECT count(*) FROM registry_evidences WHERE in_khur),
			(SELECT count(*) FROM registry_once_only_violations),
			(SELECT COALESCE(sum(annual_volume), 0) FROM registry_once_only_violations),
			(SELECT COALESCE(avg(max_days), 0) FROM registry_services WHERE status = 'published')
	`).Scan(&o.TotalServices, &o.PublishedServices, &o.DraftServices, &o.LifeEvents, &o.Evidences,
		&o.EvidencesInKHUR, &o.OnceOnlyViolations, &o.OnceOnlyAnnualHits, &o.AvgMaxDays)
	if err != nil {
		return domain.RegistryOverview{}, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT proactivity, count(*) FROM registry_services GROUP BY proactivity`)
	if err != nil {
		return domain.RegistryOverview{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return domain.RegistryOverview{}, err
		}
		o.ByProactivity[k] = n
	}
	return o, rows.Err()
}

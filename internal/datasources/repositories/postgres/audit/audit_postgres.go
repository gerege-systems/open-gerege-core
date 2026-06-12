// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package audit нь hash-chained, append-only audit_log хүснэгтийн Postgres
// gateway юм. audit_log нь admin-only тул энэ repository нь хүсэлтийн (user)
// RLS identity-аас үл хамааран бичих/уншихдаа транзакц дотроо тусгайлан
// "service" эсвэл "admin" GUC тогтоодог (eID reference-ийн "audit writer
// bypasses request RLS context" зарчмыг дагасан — системийн ямар ч actor-ийн
// нэрийн өмнөөс лог бичих чадвартай байх ёстой).
//
// Гинжийн зөв холболтыг хангахын тулд Append нь нэг global advisory lock-оор
// бичилтийг цувралжуулна — зэрэгцээ бичигчид нэг нэгээрээ дараалж prev_hash-аа
// зөв уншина. Хүснэгт жижиг тул advisory lock хангалттай энгийн бөгөөд зөв.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	repointerface "template/internal/datasources/repositories/interface"
	pkgaudit "template/pkg/audit"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// auditChainLockKey нь Append-ийн бичилтийг цувралжуулах транзакц-түвшний
// advisory lock-ийн тогтмол түлхүүр. Утга нь дурын тогтмол int64 — зөвхөн
// audit_log бичигчид хуваалцана.
const auditChainLockKey int64 = 778899

type auditRepository struct {
	pool *pgxpool.Pool
}

// NewAuditRepository нь pgx pool дээр audit gateway үүсгэнэ.
func NewAuditRepository(pool *pgxpool.Pool) repointerface.AuditRepository {
	return &auditRepository{pool: pool}
}

// setRole нь транзакцид зориулж app.user_role GUC-ийг тогтооно (SET LOCAL-тэй
// дүйцэх). audit_log-ийн бичилт/уншилт энэ дор явна.
func setRole(ctx context.Context, tx pgx.Tx, role string) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.user_role',$1,true)`, role); err != nil {
		return fmt.Errorf("set audit rls role: %w", err)
	}
	return nil
}

// Append нь нэг үйл явдлыг гинжийн төгсгөлд нэмнэ. service GUC дор нэг
// транзакцид: (1) advisory lock авч бичигчдийг цувралжуулна, (2) хамгийн
// сүүлийн мөрийн chain_hash-г prev_hash болгож уншина (хоосон гинжид ""),
// (3) шинэ chain_hash тооцоолж мөр оруулна. Бичигдсэн chain_hash-г буцаана.
func (r *auditRepository) Append(ctx context.Context, e pkgaudit.ChainEntry) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := setRole(ctx, tx, "service"); err != nil {
		return "", err
	}

	// Бичигчдийг цувралжуул — lock нь транзакц commit/rollback хийгдтэл хэвээр.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, auditChainLockKey); err != nil {
		return "", fmt.Errorf("audit chain lock: %w", err)
	}

	var prevHash string
	err = tx.QueryRow(ctx, `SELECT chain_hash FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&prevHash)
	if errors.Is(err, pgx.ErrNoRows) {
		prevHash = "" // genesis
	} else if err != nil {
		return "", fmt.Errorf("audit read prev hash: %w", err)
	}

	if e.OccurredAt.IsZero() {
		return "", fmt.Errorf("audit append: occurred_at is required")
	}

	chainHash, err := pkgaudit.ComputeChainHash(prevHash, e)
	if err != nil {
		return "", fmt.Errorf("audit compute hash: %w", err)
	}

	metaJSON, err := json.Marshal(e.Metadata)
	if err != nil {
		return "", fmt.Errorf("audit metadata marshal: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log
		    (occurred_at, actor_user_id, action, category, target, request_id, metadata, prev_hash, chain_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.OccurredAt, nullableUUID(e.ActorUserID), e.Action, nullableText(e.Category),
		nullableText(e.Target), nullableText(e.RequestID), metaJSON,
		nullableText(prevHash), chainHash,
	); err != nil {
		return "", fmt.Errorf("audit insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("audit commit: %w", err)
	}
	return chainHash, nil
}

// List нь audit мөрүүдийг admin GUC дор id буурахаар хуудаслан буцаана.
func (r *auditRepository) List(ctx context.Context, filter repointerface.AuditListFilter, limit, offset int) ([]repointerface.AuditLogRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := setRole(ctx, tx, "admin"); err != nil {
		return nil, err
	}

	// Параметрчилсэн динамик WHERE — утга бүр нь bind параметр (SQL injection-гүй).
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 4)
	idx := 1
	if filter.Action != "" {
		clauses = append(clauses, fmt.Sprintf("action = $%d", idx))
		args = append(args, filter.Action)
		idx++
	}
	if filter.ActorUserID != "" {
		clauses = append(clauses, fmt.Sprintf("actor_user_id = $%d", idx))
		args = append(args, filter.ActorUserID)
		idx++
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, occurred_at, actor_user_id, action, category, target, request_id, metadata, prev_hash, chain_hash
		  FROM audit_log%s
		 ORDER BY id DESC
		 LIMIT $%d OFFSET $%d`, where, idx, idx+1)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]repointerface.AuditLogRow, 0, limit)
	for rows.Next() {
		row, scanErr := scanAuditRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, row)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, tx.Commit(ctx)
}

// VerifyChain нь гинжийг admin GUC дор genesis-ээс эхлэн дахин тооцоолно. Мөр
// бүрийн хадгалагдсан prev_hash болон chain_hash-г шинээр тооцоолсонтой
// харьцуулна; эхний зөрчилтэй мөрийн id-г буцаана.
func (r *auditRepository) VerifyChain(ctx context.Context) (bool, int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := setRole(ctx, tx, "admin"); err != nil {
		return false, 0, err
	}

	rows, err := tx.Query(ctx, `
		SELECT id, occurred_at, actor_user_id, action, category, target, request_id, metadata, prev_hash, chain_hash
		  FROM audit_log
		 ORDER BY id ASC`)
	if err != nil {
		return false, 0, err
	}
	defer rows.Close()

	prev := "" // genesis
	for rows.Next() {
		row, scanErr := scanAuditRow(rows)
		if scanErr != nil {
			return false, 0, scanErr
		}
		// prev_hash тасралтгүй эсэх.
		if row.PrevHash != prev {
			return false, row.ID, nil
		}
		computed, hErr := pkgaudit.ComputeChainHash(prev, pkgaudit.ChainEntry{
			OccurredAt:  row.OccurredAt,
			ActorUserID: row.ActorUserID,
			Action:      row.Action,
			Category:    row.Category,
			Target:      row.Target,
			RequestID:   row.RequestID,
			Metadata:    row.Metadata,
		})
		if hErr != nil {
			return false, 0, hErr
		}
		if computed != row.ChainHash {
			return false, row.ID, nil
		}
		prev = row.ChainHash
	}
	if rows.Err() != nil {
		return false, 0, rows.Err()
	}
	return true, 0, tx.Commit(ctx)
}

// scanAuditRow нь нэг audit_log мөрийг AuditLogRow болгон уншина. NULL талбаруудыг
// (actor_user_id, category, target, request_id, prev_hash, metadata) аюулгүй
// задлахын тулд pointer/[]byte ашиглана.
func scanAuditRow(row pgx.Row) (repointerface.AuditLogRow, error) {
	var (
		out      repointerface.AuditLogRow
		actor    *string
		category *string
		target   *string
		reqID    *string
		prevHash *string
		metaJSON []byte
	)
	if err := row.Scan(&out.ID, &out.OccurredAt, &actor, &out.Action, &category,
		&target, &reqID, &metaJSON, &prevHash, &out.ChainHash); err != nil {
		return repointerface.AuditLogRow{}, err
	}
	out.ActorUserID = deref(actor)
	out.Category = deref(category)
	out.Target = deref(target)
	out.RequestID = deref(reqID)
	out.PrevHash = deref(prevHash)
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &out.Metadata)
	}
	return out, nil
}

// nullableUUID нь хоосон тэмдэгт мөрийг SQL NULL (nil) болгоно — actor_user_id нь
// uuid багана тул хоосон тэмдэгт мөр ”::uuid алдаа өгөхөөс сэргийлнэ.
func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableText нь хоосон тэмдэгт мөрийг SQL NULL болгоно (TEXT багануудад).
func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

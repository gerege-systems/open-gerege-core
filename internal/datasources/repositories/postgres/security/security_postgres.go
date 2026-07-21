// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package security нь RASP-style security_events хүснэгтийн Postgres gateway юм.
// Ingest нь нэвтэрсэн хэрэглэгчийн (user) RLS identity дор ажилладаг тул RLS
// бодлого user_id = app.user_id-г баталгаажуулна (хэрэглэгч зөвхөн өөрийнхөө
// тухай event бичиж чадна); List нь admin GUC дор бүх event-ийг уншина.
package security

import (
	"context"
	"encoding/json"
	"fmt"

	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type securityRepository struct {
	pool *pgxpool.Pool
}

// NewSecurityEventRepository нь pgx pool дээр security event gateway үүсгэнэ.
func NewSecurityEventRepository(pool *pgxpool.Pool) repointerface.SecurityEventRepository {
	return &securityRepository{pool: pool}
}

// withRLS нь users repository-тэй ижил загвараар хүсэлтийн context дахь
// Identity-г (rls.FromContext) транзакцид SET LOCAL болгож тогтооно — ингэснээр
// security_events дээрх RLS бодлого тухайн хэрэглэгчийн нэрийн өмнөөс хэрэгжинэ.
func (r *securityRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
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

// Ingest нь нэг security event бичнэ (хэрэглэгчийн RLS identity дор).
func (r *securityRepository) Ingest(ctx context.Context, e repointerface.SecurityEventRecord) error {
	detailJSON, err := json.Marshal(e.Detail)
	if err != nil {
		return fmt.Errorf("security detail marshal: %w", err)
	}
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, `
			INSERT INTO security_events
			    (user_id, kind, severity, source, user_agent, ip, detail)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			nullableUUID(e.UserID), e.Kind, nullableText(e.Severity), nullableText(e.Source),
			nullableText(e.UserAgent), nullableText(e.IP), detailJSON,
		)
		return execErr
	})
}

// List нь event-үүдийг admin GUC дор received_at буурахаар хуудаслан буцаана.
func (r *securityRepository) List(ctx context.Context, limit, offset int) ([]repointerface.SecurityEventRecord, error) {
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
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit returns ErrTxClosed — expected, nothing to handle
	if _, err := tx.Exec(ctx, `SELECT set_config('app.user_role','admin',true)`); err != nil {
		return nil, fmt.Errorf("set security rls role: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT id, received_at, user_id, kind, severity, source, user_agent, ip, detail
		  FROM security_events
		 ORDER BY id DESC
		 LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]repointerface.SecurityEventRecord, 0, limit)
	for rows.Next() {
		var (
			rec       repointerface.SecurityEventRecord
			userID    *string
			severity  *string
			source    *string
			userAgent *string
			ip        *string
			detail    []byte
		)
		if err := rows.Scan(&rec.ID, &rec.ReceivedAt, &userID, &rec.Kind, &severity,
			&source, &userAgent, &ip, &detail); err != nil {
			return nil, err
		}
		rec.UserID = deref(userID)
		rec.Severity = deref(severity)
		rec.Source = deref(source)
		rec.UserAgent = deref(userAgent)
		rec.IP = deref(ip)
		if len(detail) > 0 {
			_ = json.Unmarshal(detail, &rec.Detail)
		}
		out = append(out, rec)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, tx.Commit(ctx)
}

func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

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

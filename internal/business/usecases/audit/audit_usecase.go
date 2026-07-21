// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package audit нь hash-chained, append-only audit log-ийн use case давхарга юм:
// үйл явдал бичих (RecordEvent), admin жагсаалт (ListEvents) болон гинжийн
// бүрэн бүтэн байдлыг шалгах (VerifyChain). Бичих actor нь хүсэлтийн RLS
// context (rls.FromContext)-оос автоматаар уншигдана; request_id-г context-оос
// (logger.RequestIDKey) гаргана.
package audit

import (
	"context"
	"time"

	"template/internal/apperror"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"
	"template/pkg/audit"
	"template/pkg/logger"
)

// Usecase нь audit log-ийн оролтын хил юм.
type Usecase interface {
	// RecordEvent нь нэг audit үйл явдлыг гинжид нэмнэ. actor-г хүсэлтийн RLS
	// identity-аас, request_id-г context-оос уншина. Бичих алдааг буцаана —
	// дуудагчид (existing flow-д) үүнийг best-effort/non-fatal-аар хэрэглэнэ.
	RecordEvent(ctx context.Context, action, category, target string, metadata map[string]any) error
	// ListEvents нь admin-д зориулсан хуудаслагдсан жагсаалт буцаана.
	ListEvents(ctx context.Context, filter repointerface.AuditListFilter, limit, offset int) ([]repointerface.AuditLogRow, error)
	// VerifyChain нь гинжийн бүрэн бүтэн байдлыг буцаана (ok + эвдэрсэн эхний мөр).
	VerifyChain(ctx context.Context) (VerifyResult, error)
}

// VerifyResult нь VerifyChain-ийн үр дүн.
type VerifyResult struct {
	OK       bool  `json:"ok"`
	BrokenID int64 `json:"broken_id,omitempty"` // OK=false үед эвдэрсэн эхний мөрийн id
}

type usecase struct {
	repo repointerface.AuditRepository
}

// NewUsecase нь audit use case-ийг үүсгэнэ.
func NewUsecase(repo repointerface.AuditRepository) Usecase {
	return &usecase{repo: repo}
}

func (u *usecase) RecordEvent(ctx context.Context, action, category, target string, metadata map[string]any) error {
	if action == "" {
		return apperror.BadRequest("audit action is required")
	}
	actor := ""
	if id, ok := rls.FromContext(ctx); ok {
		actor = id.UserID
	}
	requestID := ""
	if v, ok := ctx.Value(logger.RequestIDKey).(string); ok {
		requestID = v
	}
	entry := audit.ChainEntry{
		// Postgres timestamptz нь микросекунд нарийвчлалтай тул hash-д орох цагийг
		// микросекунд хүртэл тайрна — эс бөгөөс DB-д бичсэний дараа уншиж дахин
		// тооцоолоход (VerifyChain) наносекундын зөрүүгээс болж hash таарахгүй.
		OccurredAt:  time.Now().UTC().Truncate(time.Microsecond),
		ActorUserID: actor,
		Action:      action,
		Category:    category,
		Target:      target,
		RequestID:   requestID,
		Metadata:    metadata,
	}
	if _, err := u.repo.Append(ctx, entry); err != nil {
		return apperror.InternalCause(err)
	}
	return nil
}

func (u *usecase) ListEvents(ctx context.Context, filter repointerface.AuditListFilter, limit, offset int) ([]repointerface.AuditLogRow, error) {
	rows, err := u.repo.List(ctx, filter, limit, offset)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	return rows, nil
}

func (u *usecase) VerifyChain(ctx context.Context) (VerifyResult, error) {
	ok, brokenID, err := u.repo.VerifyChain(ctx)
	if err != nil {
		return VerifyResult{}, apperror.InternalCause(err)
	}
	return VerifyResult{OK: ok, BrokenID: brokenID}, nil
}

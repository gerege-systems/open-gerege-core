// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package security нь RASP-style security event-ийн use case давхарга юм:
// нэвтэрсэн хэрэглэгчээс event хүлээн авах (Ingest) болон admin-д зориулсан
// жагсаалт (List). actor (user_id), IP, user-agent зэрэг нь handler-ээс
// дамждаг — usecase нь зөвхөн баталгаажуулалт + repository руу дамжуулна.
package security

import (
	"context"
	"strings"

	"template/internal/apperror"
	repointerface "template/internal/datasources/repositories/interface"
)

// Usecase нь security event-ийн оролтын хил юм.
type Usecase interface {
	// Ingest нь нэг security event бичнэ. UserID нь хэрэглэгчийн RLS identity-тэй
	// таарах ёстой (RLS бодлого баталгаажуулна).
	Ingest(ctx context.Context, req IngestRequest) error
	// List нь admin-д зориулсан хуудаслагдсан жагсаалт буцаана.
	List(ctx context.Context, limit, offset int) ([]repointerface.SecurityEventRecord, error)
}

// IngestRequest нь нэг event ингест хийх оролт. UserID/IP/UserAgent-г handler
// нь request context-оос бөглөнө; Kind заавал.
type IngestRequest struct {
	UserID    string
	Kind      string
	Severity  string
	Source    string
	UserAgent string
	IP        string
	Detail    map[string]any
}

type usecase struct {
	repo repointerface.SecurityEventRepository
}

// NewUsecase нь security event use case-ийг үүсгэнэ.
func NewUsecase(repo repointerface.SecurityEventRepository) Usecase {
	return &usecase{repo: repo}
}

func (u *usecase) Ingest(ctx context.Context, req IngestRequest) error {
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		return apperror.BadRequest("security event kind is required")
	}
	if err := u.repo.Ingest(ctx, repointerface.SecurityEventRecord{
		UserID:    req.UserID,
		Kind:      kind,
		Severity:  strings.TrimSpace(req.Severity),
		Source:    strings.TrimSpace(req.Source),
		UserAgent: req.UserAgent,
		IP:        req.IP,
		Detail:    req.Detail,
	}); err != nil {
		return apperror.InternalCause(err)
	}
	return nil
}

func (u *usecase) List(ctx context.Context, limit, offset int) ([]repointerface.SecurityEventRecord, error) {
	rows, err := u.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	return rows, nil
}

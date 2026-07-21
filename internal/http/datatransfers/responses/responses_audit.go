// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	repointerface "template/internal/datasources/repositories/interface"
)

// AuditLogResponse нь hash-chained audit_log-ийн нэг мөрийг клиентэд (admin)
// буцаана. chain_hash/prev_hash-г оруулдаг тул гадны аудитор гинжийг шалгаж чадна.
type AuditLogResponse struct {
	ID          int64          `json:"id"`
	OccurredAt  time.Time      `json:"occurred_at"`
	ActorUserID string         `json:"actor_user_id,omitempty"`
	Action      string         `json:"action"`
	Category    string         `json:"category,omitempty"`
	Target      string         `json:"target,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	PrevHash    string         `json:"prev_hash,omitempty"`
	ChainHash   string         `json:"chain_hash"`
}

// FromAuditRow нь audit мөрийг хариуны DTO руу буулгана.
func FromAuditRow(row repointerface.AuditLogRow) AuditLogResponse {
	return AuditLogResponse{
		ID:          row.ID,
		OccurredAt:  row.OccurredAt,
		ActorUserID: row.ActorUserID,
		Action:      row.Action,
		Category:    row.Category,
		Target:      row.Target,
		RequestID:   row.RequestID,
		Metadata:    row.Metadata,
		PrevHash:    row.PrevHash,
		ChainHash:   row.ChainHash,
	}
}

// ToAuditList нь audit мөрүүдийг DTO жагсаалт болгоно.
func ToAuditList(rows []repointerface.AuditLogRow) []AuditLogResponse {
	out := make([]AuditLogResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, FromAuditRow(r))
	}
	return out
}

// AuditVerifyResponse нь гинжийн бүрэн бүтэн байдлын төлвийг буцаана.
type AuditVerifyResponse struct {
	OK       bool  `json:"ok"`
	BrokenID int64 `json:"broken_id,omitempty"`
}

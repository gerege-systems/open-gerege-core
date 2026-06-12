// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	repointerface "template/internal/datasources/repositories/interface"
)

// SecurityEventResponse нь RASP-style security_events-ийн нэг мөрийг (admin)
// буцаана.
type SecurityEventResponse struct {
	ID         int64          `json:"id"`
	ReceivedAt time.Time      `json:"received_at"`
	UserID     string         `json:"user_id,omitempty"`
	Kind       string         `json:"kind"`
	Severity   string         `json:"severity,omitempty"`
	Source     string         `json:"source,omitempty"`
	UserAgent  string         `json:"user_agent,omitempty"`
	IP         string         `json:"ip,omitempty"`
	Detail     map[string]any `json:"detail,omitempty"`
}

// FromSecurityEvent нь security event мөрийг хариуны DTO руу буулгана.
func FromSecurityEvent(rec repointerface.SecurityEventRecord) SecurityEventResponse {
	return SecurityEventResponse{
		ID:         rec.ID,
		ReceivedAt: rec.ReceivedAt,
		UserID:     rec.UserID,
		Kind:       rec.Kind,
		Severity:   rec.Severity,
		Source:     rec.Source,
		UserAgent:  rec.UserAgent,
		IP:         rec.IP,
		Detail:     rec.Detail,
	}
}

// ToSecurityEventList нь security event-үүдийг DTO жагсаалт болгоно.
func ToSecurityEventList(rows []repointerface.SecurityEventRecord) []SecurityEventResponse {
	out := make([]SecurityEventResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, FromSecurityEvent(r))
	}
	return out
}

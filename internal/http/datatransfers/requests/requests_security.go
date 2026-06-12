// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// IngestSecurityEventRequest нь POST /v1/security/events-ийн body. kind заавал
// (жишээ: "rasp.jailbreak", "integrity.tamper", "anomaly.timing"); severity
// сонголттой (low/medium/high/critical). detail нь PII-гүй нэмэлт нотолгоо.
type IngestSecurityEventRequest struct {
	Kind     string         `json:"kind" validate:"required,max=80"`
	Severity string         `json:"severity" validate:"omitempty,oneof=low medium high critical"`
	Source   string         `json:"source" validate:"omitempty,max=80"`
	Detail   map[string]any `json:"detail" validate:"omitempty"`
}

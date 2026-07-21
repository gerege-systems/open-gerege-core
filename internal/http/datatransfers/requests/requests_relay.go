// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

import (
	"encoding/json"
	"time"
)

// RelayIngestRequest нь дээд platform-оос ирэх хүсэлт (m2m). DueAt хоосон бол
// routing-ийн хамгийн урт SLA-аар тооцно.
type RelayIngestRequest struct {
	SourcePlatform string          `json:"source_platform"`
	ExternalRef    string          `json:"external_ref"`
	ServiceCode    string          `json:"service_code" validate:"required"`
	Title          string          `json:"title"`
	Payload        json.RawMessage `json:"payload"`
	Priority       string          `json:"priority"`
	DueAt          *time.Time      `json:"due_at"`
}

// RelayRespondRequest нь доод platform-ын callback (m2m).
type RelayRespondRequest struct {
	Status string          `json:"status" validate:"required,oneof=done rejected"`
	Result json.RawMessage `json:"result"`
}

// RelayPlatformRequest нь доод platform бүртгэх/шинэчлэх (admin).
type RelayPlatformRequest struct {
	Code              string `json:"code" validate:"required"`
	Name              string `json:"name" validate:"required"`
	Direction         string `json:"direction" validate:"omitempty,oneof=upstream downstream"`
	EndpointURL       string `json:"endpoint_url"`
	SupervisorContact string `json:"supervisor_contact"`
	WebhookSecret     string `json:"webhook_secret"`
	Enabled           bool   `json:"enabled"`
}

// RelayRouteRequest нь чиглүүлэлт (service_code → platform) үүсгэх (admin).
type RelayRouteRequest struct {
	ServiceCode string `json:"service_code" validate:"required"`
	PlatformID  string `json:"platform_id" validate:"required"`
	SLAMinutes  int    `json:"sla_minutes"`
}

// RelayForwardRequest нь хүсэлтийг сонгосон дээд (upstream) platform руу дамжуулах
// (admin).
type RelayForwardRequest struct {
	PlatformCode string `json:"platform_code" validate:"required"`
}

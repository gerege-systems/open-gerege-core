// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

import (
	"encoding/json"
	"time"
)

// GovApplyRequest нь үйлчилгээнд хүсэлт гаргах body. Payload нь үйлчилгээ тус
// бүрийн маягтын өгөгдөл (cpsv:hasInput-д тохирох) — чөлөөт бүтэцтэй.
type GovApplyRequest struct {
	ServiceID string          `json:"service_id" validate:"required,uuid"`
	Note      string          `json:"note" validate:"omitempty,max=500"`
	Payload   json.RawMessage `json:"payload" validate:"omitempty"`
}

// GovDecideRequest нь менежерийн шийдвэрийн body. Татгалзах үед note-г
// usecase давхарга заавал шаардана (иргэн үндэслэлийг мэдэх эрхтэй).
type GovDecideRequest struct {
	Approve bool   `json:"approve"`
	Note    string `json:"note" validate:"omitempty,max=1000"`
	Result  string `json:"result" validate:"omitempty,oneof=granted refused withdrawn not_admissible processed"`
}

// GovInfoRequest нь нэмэлт мэдээлэл хүсэх/өгөх body.
type GovInfoRequest struct {
	Note string `json:"note" validate:"required,max=1000"`
}

// GovReferenceRequest нь лавлагаа захиалах body.
type GovReferenceRequest struct {
	Type string `json:"type" validate:"required,oneof=residence birth marriage tax social_ins criminal"`
}

// GovBookRequest нь цаг захиалах body.
type GovBookRequest struct {
	ServiceID   string    `json:"service_id" validate:"omitempty,uuid"`
	ScheduledAt time.Time `json:"scheduled_at" validate:"required"`
	Location    string    `json:"location" validate:"omitempty,max=200"`
	Note        string    `json:"note" validate:"omitempty,max=500"`
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

import "time"

// GovApplyRequest нь үйлчилгээнд хүсэлт гаргах body.
type GovApplyRequest struct {
	ServiceID string `json:"service_id" validate:"required,uuid"`
	Note      string `json:"note" validate:"omitempty,max=500"`
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

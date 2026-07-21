// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

import "encoding/json"

// ThemeUpsertRequest нь theme үүсгэх/шинэчлэх админ хүсэлт. config нь чөлөөт
// JSONB — нарийн шалгалт usecase/domain давхаргад (ValidateThemeConfig).
type ThemeUpsertRequest struct {
	Name   string          `json:"name" validate:"required,max=80"`
	Config json.RawMessage `json:"config"`
}

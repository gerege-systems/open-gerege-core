// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"encoding/json"
	"time"

	"template/internal/business/domain"
)

// ThemeResponse нь landing-ийн нэрлэсэн загварыг frontend-д буцаах хэлбэр.
// config нь бүрэн theme (харагдац + landing текст/цэс).
type ThemeResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
}

func ToTheme(t domain.Theme) ThemeResponse {
	cfg := t.Config
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	return ThemeResponse{
		ID:        t.ID,
		Name:      t.Name,
		Config:    cfg,
		IsActive:  t.IsActive,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func ToThemeList(list []domain.Theme) []ThemeResponse {
	out := make([]ThemeResponse, 0, len(list))
	for _, t := range list {
		out = append(out, ToTheme(t))
	}
	return out
}

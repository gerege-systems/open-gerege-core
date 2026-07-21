// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/domain"
)

// SiteAppearanceResponse нь сайтын харагдацын default-ыг frontend-д буцаах
// хэлбэр. accent нь preset нэр ('cobalt' г.м.) эсвэл '#rrggbb' custom hex.
type SiteAppearanceResponse struct {
	Accent    string     `json:"accent"`
	Font      string     `json:"font"`
	Style     string     `json:"style"`
	Theme     string     `json:"theme"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// ToSiteAppearance нь domain-ийг DTO болгоно.
func ToSiteAppearance(a domain.SiteAppearance) SiteAppearanceResponse {
	return SiteAppearanceResponse{
		Accent:    a.Accent,
		Font:      a.Font,
		Style:     a.Style,
		Theme:     a.Theme,
		UpdatedAt: a.UpdatedAt,
	}
}

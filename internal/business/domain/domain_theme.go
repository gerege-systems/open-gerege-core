// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// Theme нь landing (нүүр) хуудасны нэрлэсэн бүрэн загвар — харагдац (палетр ·
// фонт · стиль · загвар) + landing-ийн бүх текст/цэс. config нь JSONB (frontend
// template default дээр deep-merge хийдэг тул уян хатан). Идэвхтэй (is_active)
// theme-ийг нэвтрээгүй зочин харна.
type Theme struct {
	ID        string          `db:"id"`
	Name      string          `db:"name"`
	Config    json.RawMessage `db:"config"`
	IsActive  bool            `db:"is_active"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt *time.Time      `db:"updated_at"`
}

// ThemeAppearance нь theme-ийн харагдацын хэсэг. Colors нь base токенуудын
// хэсэгчилсэн газрын зураг (hex); frontend hover/soft/text хувилбарыг color-mix-
// ээр гаргана. Хоосон бол template default (globals.css) хэвээр.
type ThemeAppearance struct {
	Mode   string            `json:"mode"`
	Font   string            `json:"font"`
	Style  string            `json:"style"`
	Colors map[string]string `json:"colors"`
}

// ThemeConfig нь config JSONB-ийн баталгаажуулах бүтэц. Landing нь чөлөөт хэлбэр
// (текст/цэс) тул зөвхөн хэмжээгээр нь шалгана.
type ThemeConfig struct {
	Appearance ThemeAppearance `json:"appearance"`
	Landing    json.RawMessage `json:"landing"`
}

// Theme-д зөвшөөрөгдсөн өнгөний base токенуудын түлхүүр (globals.css :root-тэй нийцнэ).
var ThemeColorKeys = map[string]bool{
	"bg": true, "surface": true, "surface2": true, "fg": true, "muted": true,
	"border": true, "borderStrong": true, "danBlue": true, "gold": true,
	"success": true, "danger": true,
	// lpNavy — landing hero/body-ийн navy дэвсгэр (--lp-navy); lpHeader — landing
	// дээд цэс (header/nav)-ийн дэвсгэр (--lp-header). Бусад токен app-ын --bg
	// г.м.-д ноогддог бол эдгээр landing-д тусгайлан ноогдоно.
	"lpNavy": true, "lpHeader": true,
}

// config JSONB-ийн дээд хэмжээ (нийт текст/цэс хоёр хэлээр) — DoS-оос хамгаална.
const ThemeConfigMaxBytes = 128 * 1024

// ValidateThemeConfig нь config JSONB-ийг баталгаажуулна: appearance-ийн enum/
// hex, өнгөний түлхүүр, нийт хэмжээ. Landing-ийн текст чөлөөт.
func ValidateThemeConfig(raw json.RawMessage) error {
	if len(raw) > ThemeConfigMaxBytes {
		return fmt.Errorf("theme config too large (%d bytes, max %d)", len(raw), ThemeConfigMaxBytes)
	}
	var cfg ThemeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("theme config not valid JSON: %w", err)
	}
	a := cfg.Appearance
	if a.Mode != "" && !SiteThemes[a.Mode] {
		return fmt.Errorf("invalid appearance.mode %q", a.Mode)
	}
	if a.Font != "" && !SiteFonts[a.Font] {
		return fmt.Errorf("invalid appearance.font %q", a.Font)
	}
	if a.Style != "" && !SiteStyles[a.Style] {
		return fmt.Errorf("invalid appearance.style %q", a.Style)
	}
	for key, val := range a.Colors {
		if !ThemeColorKeys[key] {
			return fmt.Errorf("unknown color token %q", key)
		}
		if !siteHexRe.MatchString(val) {
			return fmt.Errorf("color %q must be #rrggbb hex, got %q", key, val)
		}
	}
	return nil
}

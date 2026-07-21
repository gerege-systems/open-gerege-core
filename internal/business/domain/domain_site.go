// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import (
	"regexp"
	"time"
)

// SiteAppearance нь сайтын нийтийн харагдацын default — админ тохируулж, бүх
// зочин үүгээр эхэлнэ. accent нь preset нэр ЭСВЭЛ '#rrggbb' custom hex.
type SiteAppearance struct {
	Accent    string     `db:"accent"`
	Font      string     `db:"font"`
	Style     string     `db:"style"`
	Theme     string     `db:"theme"`
	UpdatedAt *time.Time `db:"updated_at"`
}

// Зөвшөөрөгдсөн утгууд — frontend-ийн preset жагсаалттай нэг мөр байх ёстой
// (globals.css html[data-*], preferences.ts).
var (
	SiteAccentPresets = map[string]bool{"cobalt": true, "teal": true, "violet": true, "emerald": true, "amber": true}
	SiteFonts         = map[string]bool{"inter": true, "serif": true, "system": true}
	SiteStyles        = map[string]bool{"comfortable": true, "compact": true}
	SiteThemes        = map[string]bool{"light": true, "dark": true, "system": true}
)

// siteHexRe нь custom accent-ийн '#rrggbb' хэлбэрийг шалгана (3-оронтой хэлбэр
// зөвшөөрөхгүй — frontend нь 6-оронтойг л илгээнэ).
var siteHexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// DefaultSiteAppearance нь seed/fallback утга (repo уншиж чадаагүй үед ч ашиглана).
func DefaultSiteAppearance() SiteAppearance {
	return SiteAppearance{Accent: "cobalt", Font: "inter", Style: "comfortable", Theme: "light"}
}

// ValidSiteAccent нь preset нэр эсвэл '#rrggbb' hex мөнийг шалгана.
func ValidSiteAccent(accent string) bool {
	return SiteAccentPresets[accent] || siteHexRe.MatchString(accent)
}

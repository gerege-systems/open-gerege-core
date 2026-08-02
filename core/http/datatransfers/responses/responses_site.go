// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
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

// SiteAuthResponse нь нэвтрэх гадаргууны горимыг frontend-д мэдэгдэнэ —
// нүүр хуудас болон /login нь нэвтрэх картыг өөрөө үзүүлэх үү, эсвэл дээд SSO
// руу шилжүүлэх үү гэдгийг үүгээр шийднэ. Нэвтрэлт шаардахгүй (landing уншина).
//
// Нууц утга агуулахгүй: горим, дээд IdP-ийн НИЙТИЙН issuer, өөрөө issuer эсэх —
// гуравуулаа browser-т аль хэдийн харагддаг мэдээлэл.
type SiteAuthResponse struct {
	// Mode — "provider" (өөрөө нэвтрүүлнэ) эсвэл "client" (SSO руу шилжүүлнэ).
	Mode string `json:"mode"`
	// SSOIssuer — client горимд дээд IdP-ийн issuer (жишээ https://sso.gerege.mn).
	// provider горимд хоосон.
	SSOIssuer string `json:"sso_issuer,omitempty"`
	// Provider — энэ платформ ӨӨРӨӨ OIDC issuer мөн эсэх (OAUTH_ISSUER +
	// SSO_STATE_KEY бүрдсэн эсэх). Горимоос ТУСДАА тэнхлэг.
	Provider bool `json:"provider"`
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

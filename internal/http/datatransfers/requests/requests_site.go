// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// SiteAppearanceUpdateRequest нь сайтын харагдацын default-ыг шинэчлэх админ
// хүсэлт. accent нь preset нэр эсвэл '#rrggbb' hex — утгын нарийн шалгалт
// usecase давхаргад (domain.ValidSiteAccent г.м.).
type SiteAppearanceUpdateRequest struct {
	Accent string `json:"accent" validate:"required,max=32"`
	Font   string `json:"font" validate:"required,max=16"`
	Style  string `json:"style" validate:"required,max=16"`
	Theme  string `json:"theme" validate:"required,max=16"`
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// AssetURLRequest нь гарын үсэг / тамганы зургийн (Google Drive) URL-ийг хадгалах body.
// Зургийг BFF талд Drive-д байршуулж, энд зөвхөн URL-ийг дамжуулна.
type AssetURLRequest struct {
	URL string `json:"url" validate:"required,url,max=1000"`
}

// LatinNameRequest нь хэрэглэгчийн латин нэрийг гараар засах body.
type LatinNameRequest struct {
	FirstNameEn string `json:"first_name_en" validate:"omitempty,max=120"`
	LastNameEn  string `json:"last_name_en" validate:"omitempty,max=120"`
}

// OrgNameLatinRequest нь байгууллагын латин нэрийг гараар засах body (ADMIN).
type OrgNameLatinRequest struct {
	NameLatin string `json:"name_latin" validate:"omitempty,max=200"`
}

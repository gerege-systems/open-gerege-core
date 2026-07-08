// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// AssetURLRequest нь гарын үсэг / тамганы зургийн (Google Drive) URL-ийг хадгалах body.
// Зургийг BFF талд Drive-д байршуулж, энд зөвхөн URL-ийг дамжуулна.
type AssetURLRequest struct {
	URL string `json:"url" validate:"required,url,max=1000"`
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// ConnectIntegrationRequest нь POST /integrations-ийн body. Frontend BFF нь
// OAuth token exchange-ийн дараа энэ хүсэлтийг (хэрэглэгчийн session-тэйгээр)
// илгээж токеныг backend-д шифрлэн хадгалуулна.
type ConnectIntegrationRequest struct {
	Provider     string `json:"provider" validate:"required,max=32"`
	AccessToken  string `json:"access_token" validate:"required,max=4096"`
	RefreshToken string `json:"refresh_token" validate:"omitempty,max=4096"`
	// ExpiresAtMs нь токены дуусах epoch миллисекунд (0 бол хугацаагүй).
	ExpiresAtMs int64 `json:"expires_at_ms" validate:"omitempty"`
}

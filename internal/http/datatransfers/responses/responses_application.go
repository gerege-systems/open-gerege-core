// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/domain"
)

// ApplicationResponse нь Application-ыг frontend-д буцаах хэлбэр. secret нь
// зөвхөн үүсгэх/эргүүлэх хариунд дүүрнэ (reveal-once); DB-д зөвхөн hash үлдэнэ.
type ApplicationResponse struct {
	ID           string     `json:"id"`
	ClientID     string     `json:"client_id"`
	Name         string     `json:"name"`
	AppType      string     `json:"app_type"`
	Tags         []string   `json:"tags"`
	RedirectURIs []string   `json:"redirect_uris"`
	Enabled      bool       `json:"enabled"`
	ServiceIDs   []string   `json:"service_ids"`
	Secret       string     `json:"secret,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

// FromApplication нь domain-ийг DTO болгоно. (nonNil нь responses_gateway.go-д.)
func FromApplication(a domain.Application) ApplicationResponse {
	return ApplicationResponse{
		ID:           a.ID,
		ClientID:     a.ClientID,
		Name:         a.Name,
		AppType:      a.AppType,
		Tags:         nonNil(a.Tags),
		RedirectURIs: nonNil(a.RedirectURIs),
		Enabled:      a.Enabled,
		ServiceIDs:   nonNil(a.ServiceIDs),
		Secret:       a.Secret,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// ToApplicationList нь домэйн жагсаалтыг DTO болгоно.
func ToApplicationList(in []domain.Application) []ApplicationResponse {
	out := make([]ApplicationResponse, 0, len(in))
	for _, a := range in {
		out = append(out, FromApplication(a))
	}
	return out
}

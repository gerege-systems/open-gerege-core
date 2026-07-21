// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/usecases/integrations"
)

// IntegrationResponse нь нэг холбосон провайдерын аюулгүй далайц — токен
// агуулахгүй.
type IntegrationResponse struct {
	Provider    string `json:"provider"`
	Connected   bool   `json:"connected"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	ConnectedAt string `json:"connected_at"`
}

// IntegrationTokenResponse нь нэг провайдерын decrypt хийсэн токен — ЗӨВХӨН
// server-тал (BFF) ашиглана, browser руу буцаахгүй.
type IntegrationTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAtMs  int64  `json:"expires_at_ms"`
}

// FromTokenData нь decrypt хийсэн токеныг хариу DTO болгоно.
func FromTokenData(t integrations.TokenData) IntegrationTokenResponse {
	r := IntegrationTokenResponse{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken}
	if t.ExpiresAt != nil {
		r.ExpiresAtMs = t.ExpiresAt.UnixMilli()
	}
	return r
}

// FromConnectedProviders нь usecase-ийн буцаалтыг хариу DTO болгоно.
func FromConnectedProviders(in []integrations.ConnectedProvider) []IntegrationResponse {
	out := make([]IntegrationResponse, 0, len(in))
	for _, p := range in {
		r := IntegrationResponse{
			Provider:    p.Provider,
			Connected:   true,
			ConnectedAt: p.ConnectedAt.Format(time.RFC3339),
		}
		if p.ExpiresAt != nil {
			r.ExpiresAt = p.ExpiresAt.Format(time.RFC3339)
		}
		out = append(out, r)
	}
	return out
}

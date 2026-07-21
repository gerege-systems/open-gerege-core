// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// GatewayServiceRequest нь upstream service үүсгэх/шинэчлэх body.
type GatewayServiceRequest struct {
	Name           string   `json:"name" validate:"required,min=2,max=80"`
	Protocol       string   `json:"protocol" validate:"omitempty,oneof=http https"`
	Host           string   `json:"host" validate:"required,max=255"`
	Port           int      `json:"port" validate:"omitempty,min=1,max=65535"`
	Path           string   `json:"path" validate:"omitempty,max=255"`
	Retries        int      `json:"retries" validate:"omitempty,min=0,max=10"`
	ConnectTimeout int      `json:"connect_timeout_ms" validate:"omitempty,min=100,max=600000"`
	Tags           []string `json:"tags" validate:"omitempty,dive,max=40"`
	Enabled        bool     `json:"enabled"`
}

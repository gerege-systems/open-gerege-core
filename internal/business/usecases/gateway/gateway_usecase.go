// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gateway нь API Gateway-ийн admin удирдлагын business logic —
// upstream service-үүдийн CRUD, dashboard-ийн нэгтгэл болон бодит /api хүсэлтийн
// лог бичилтийг хариуцна.
package gateway

import (
	"context"

	"template/internal/business/domain"
)

type Usecase interface {
	// Services
	ListServices(ctx context.Context) ([]domain.GatewayService, error)
	CreateService(ctx context.Context, in ServiceInput) (domain.GatewayService, error)
	UpdateService(ctx context.Context, id string, in ServiceInput) (domain.GatewayService, error)
	DeleteService(ctx context.Context, id string) error

	// Telemetry
	ListRequestLogs(ctx context.Context, limit int) ([]domain.GatewayRequestLog, error)
	Overview(ctx context.Context) (domain.GatewayOverview, error)
	// RecordRequest нь middleware-ээс ирсэн бодит /api хүсэлтийг лог-д бичнэ
	// (best-effort; алдааг залгидаг тул хүсэлтийг блоклохгүй).
	RecordRequest(ctx context.Context, in RequestLogInput)
}

type (
	ServiceInput struct {
		Name           string
		Protocol       string
		Host           string
		Port           int
		Path           string
		Retries        int
		ConnectTimeout int
		Tags           []string
		Enabled        bool
	}
	// RequestLogInput нь middleware-ээс ирэх бодит хүсэлтийн бичлэг.
	RequestLogInput struct {
		Method    string
		Path      string
		Status    int
		LatencyMS int
		ClientIP  string
	}
)

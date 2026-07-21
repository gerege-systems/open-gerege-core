// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package relay нь platform-хоорондын үйлчилгээний хүсэлт дамжуулах + SLA
// хяналтын business logic. Дээд platform-оос хугацаатай хүсэлт хүлээж авч
// (Ingest), routing дүрмээр доод platform-ууд руу дамжуулж, заагдсан хугацаанд
// биелэлтийг хянаж/шахаж (SLASweep), хариуг цуглуулна (Respond).
package relay

import (
	"context"
	"time"

	"template/internal/business/domain"
)

type Usecase interface {
	// Ingest нь дээд platform-оос ирсэн хүсэлтийг хүлээж авч, routing дүрмээр
	// assignment үүсгэн доод platform-ууд руу дамжуулна.
	Ingest(ctx context.Context, in IngestInput) (domain.RelayRequest, error)
	// Respond нь доод platform-ын callback — assignment-ыг терминал болгоно.
	Respond(ctx context.Context, assignmentID string, in RespondInput) error

	// ReceiveWebhook нь бүртгэлтэй peer (дээд эсвэл доод) platform-оос ирсэн
	// webhook-ийг HMAC гарын үсгээр баталгаажуулж, шинэ хүсэлт болгон ingest хийнэ.
	ReceiveWebhook(ctx context.Context, sourceCode, signature string, body []byte) (domain.RelayRequest, error)
	// ForwardUp нь хүсэлтийг сонгосон дээд (upstream) platform руу webhook-оор
	// дамжуулна (тайлагнах/шат ахиулах).
	ForwardUp(ctx context.Context, requestID, platformCode string) error

	// SLASweep нь background worker-ийн нэг алхам: reminder/overdue/breach/escalate.
	SLASweep(ctx context.Context) error
	// SimulateStep нь demo (scaffold) — доод platform-уудын нэрийн өмнөөс хариу
	// үүсгэж, зарим хүсэлтийг overdue болгоно (dashboard-ыг өөрөө хөдөлгөнө).
	SimulateStep(ctx context.Context)
	// SimulateIngest нь demo (scaffold) — санамсаргүй service_code-оор шинэ демо
	// хүсэлт (богино SLA цонхтой) ingest хийж, dashboard-ийн урсгалыг үргэлжлүүлнэ.
	SimulateIngest(ctx context.Context)

	// Dashboard + жагсаалт.
	Overview(ctx context.Context) (domain.RelayOverview, error)
	ListRequests(ctx context.Context, limit int) ([]domain.RelayRequest, error)
	GetRequest(ctx context.Context, id string) (domain.RelayRequestDetail, error)

	// Platforms / routes (admin).
	ListPlatforms(ctx context.Context) ([]domain.RelayPlatform, error)
	CreatePlatform(ctx context.Context, in PlatformInput) (domain.RelayPlatform, error)
	DeletePlatform(ctx context.Context, id string) error
	ListRoutes(ctx context.Context) ([]domain.RelayRoute, error)
	CreateRoute(ctx context.Context, in RouteInput) (domain.RelayRoute, error)
	DeleteRoute(ctx context.Context, id string) error
}

type (
	// IngestInput нь дээд platform-оос ирэх хүсэлт. DueAt хоосон бол routing-ийн
	// хамгийн урт SLA-аар тооцно.
	IngestInput struct {
		SourcePlatform string
		ExternalRef    string
		ServiceCode    string
		Title          string
		Payload        []byte
		Priority       string
		DueAt          *time.Time
	}
	// RespondInput нь доод platform-ын хариу. Status = done | rejected.
	RespondInput struct {
		Status string
		Result []byte
	}
	PlatformInput struct {
		Code              string
		Name              string
		Direction         string // upstream | downstream (хоосон бол downstream)
		EndpointURL       string
		SupervisorContact string
		WebhookSecret     string // хоосон бол автоматаар үүсгэнэ
		Enabled           bool
	}
	RouteInput struct {
		ServiceCode string
		PlatformID  string
		SLAMinutes  int
	}
)

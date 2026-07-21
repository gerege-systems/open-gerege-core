// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package _interface

import (
	"context"

	"template/internal/business/domain"
)

// RelayRepository нь platform-хоорондын хүсэлт дамжуулах + SLA хяналтын gateway.
// gateway_postgres-ийн адил RLS-гүй (platform-хоорондын тохиргоо/telemetry).
type RelayRepository interface {
	// Platforms (upstream/downstream peer registry).
	ListPlatforms(ctx context.Context) ([]domain.RelayPlatform, error)
	GetPlatformByCode(ctx context.Context, code string) (domain.RelayPlatform, error)
	CreatePlatform(ctx context.Context, in *domain.RelayPlatform) (domain.RelayPlatform, error)
	DeletePlatform(ctx context.Context, id string) error

	// Routes (service_code → platform).
	ListRoutes(ctx context.Context) ([]domain.RelayRoute, error)
	RoutesForService(ctx context.Context, serviceCode string) ([]domain.RelayRoute, error)
	CreateRoute(ctx context.Context, in *domain.RelayRoute) (domain.RelayRoute, error)
	DeleteRoute(ctx context.Context, id string) error

	// Requests + assignments.
	// CreateRequestWithAssignments нь хүсэлт + assignment-уудыг нэг транзакцаар
	// үүсгэнэ (assignments дээр due_at аль хэдийн тооцоологдсон).
	CreateRequestWithAssignments(ctx context.Context, req *domain.RelayRequest, asg []domain.RelayAssignment) (domain.RelayRequest, []domain.RelayAssignment, error)
	GetAssignment(ctx context.Context, id string) (domain.RelayAssignment, error)
	// RespondAssignment нь assignment-ыг терминал төлөвт (done/rejected) оруулж,
	// бүх assignment терминал болсон бол хүсэлтийг fulfilled болгоно. Хүсэлт
	// бүхэлдээ дуусвал true буцаана.
	RespondAssignment(ctx context.Context, assignmentID, status string, result []byte) (domain.RelayRequest, bool, error)
	MarkDispatched(ctx context.Context, assignmentID string) error

	// SLA sweep-д хэрэглэгдэх query-ууд.
	DueSoonAssignments(ctx context.Context) ([]domain.RelayAssignment, error)
	OverdueAssignments(ctx context.Context) ([]domain.RelayAssignment, error)
	MarkAssignmentOverdue(ctx context.Context, assignmentID string) error
	IncReminders(ctx context.Context, assignmentID string) error
	MarkEscalated(ctx context.Context, assignmentID string) error
	MarkRequestOverdue(ctx context.Context, requestID string) error
	// MarkBreachNotified нь breach_notified-ыг true болгоно; шинээр true болсон
	// (өмнө нь false байсан) бол true буцаана — дээд platform-д зөвхөн НЭГ удаа
	// мэдэгдэхэд ашиглана.
	MarkBreachNotified(ctx context.Context, requestID string) (bool, error)

	// Events (timeline + realtime feed).
	AppendEvent(ctx context.Context, e *domain.RelayEvent) error

	// Dashboard + жагсаалт.
	Overview(ctx context.Context) (domain.RelayOverview, error)
	ListRequests(ctx context.Context, limit int) ([]domain.RelayRequest, error)
	GetRequestDetail(ctx context.Context, id string) (domain.RelayRequestDetail, error)
}

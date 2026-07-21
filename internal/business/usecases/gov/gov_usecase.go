// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gov нь иргэний "Төрийн үйлчилгээ" порталын business logic —
// каталог, хүсэлт, лавлагаа, мэдэгдэл, төлбөр, цаг захиалга. Хэрэглэгч-тус-бүрийн
// үйлдлүүд нь баталгаажсан userID-г шаардана.
package gov

import (
	"context"
	"time"

	"template/internal/business/domain"
)

type Usecase interface {
	// Каталог (нийтийн)
	ListServices(ctx context.Context) ([]domain.GovService, error)
	ListLifeEvents(ctx context.Context) ([]domain.GovLifeEvent, error)

	// Нүүр
	Overview(ctx context.Context, userID string) (domain.GovOverview, error)

	// Хүсэлт (иргэн)
	ListApplications(ctx context.Context, userID string) ([]domain.GovApplication, error)
	Apply(ctx context.Context, userID string, in ApplyRequest) (ApplyResult, error)
	CancelApplication(ctx context.Context, userID, id string) error
	// ApplicationTimeline нь иргэнд өөрийн хүсэлтийн явцыг харуулна.
	ApplicationTimeline(ctx context.Context, userID, id string) ([]domain.GovApplicationEvent, error)
	// ProvideInfo нь info_required төлөвт байгаа хүсэлтэд иргэн нэмэлт
	// мэдээлэл өгснийг бүртгэж SLA цагийг үргэлжлүүлнэ.
	ProvideInfo(ctx context.Context, userID, id, note string) (domain.GovApplication, error)

	// Хүсэлт (менежер — gov.review эрхтэй)
	QueueStats(ctx context.Context, officerID string) (domain.GovQueueStats, error)
	ListQueue(ctx context.Context, officerID string, f domain.GovQueueFilter) ([]domain.GovApplication, error)
	QueueItem(ctx context.Context, id string) (QueueItemDetail, error)
	Assign(ctx context.Context, officerID, id string) (domain.GovApplication, error)
	Decide(ctx context.Context, officerID, id string, in DecideRequest) (domain.GovApplication, error)
	// Complete нь биет гаралт хүргэгдсэнийг бүртгэж хүсэлтийг хаана.
	Complete(ctx context.Context, officerID, id string) (domain.GovApplication, error)
	RequestInfo(ctx context.Context, officerID, id, note string) (domain.GovApplication, error)

	// SLASweep нь background worker-ээс дуудагдана: хугацаа хэтэрсэн хүсэлтийг
	// тэмдэглэж, чимээгүй зөвшөөрлийг хэрэгжүүлж, иргэнд мэдэгдэнэ.
	SLASweep(ctx context.Context) error

	// Лавлагаа
	ListReferences(ctx context.Context, userID string) ([]domain.GovReference, error)
	RequestReference(ctx context.Context, userID string, in ReferenceRequest) (domain.GovReference, error)

	// Мэдэгдэл
	ListNotifications(ctx context.Context, userID string) ([]domain.GovNotification, error)
	MarkNotificationRead(ctx context.Context, userID, id string) error
	MarkAllNotificationsRead(ctx context.Context, userID string) error

	// Төлбөр
	ListPayments(ctx context.Context, userID string) ([]domain.GovPayment, error)
	PayPayment(ctx context.Context, userID, id string) error

	// Цаг захиалга
	ListAppointments(ctx context.Context, userID string) ([]domain.GovAppointment, error)
	BookAppointment(ctx context.Context, userID string, in BookRequest) (domain.GovAppointment, error)
	CancelAppointment(ctx context.Context, userID, id string) error
}

type (
	ApplyRequest struct {
		ServiceID string
		Note      string
		Payload   []byte
	}
	// ApplyResult нь хүсэлтийн үр дүн. AutoIssued=true үед үйлчилгээ ШУУД
	// биелсэн бөгөөд Reference нь олгогдсон лавлагаа; эсрэг тохиолдолд хүсэлт
	// менежерийн дараалалд орсон бөгөөд DueAt нь амлагдсан хугацаа.
	ApplyResult struct {
		Application domain.GovApplication
		Reference   *domain.GovReference
		AutoIssued  bool
	}
	// DecideRequest нь менежерийн шийдвэр.
	DecideRequest struct {
		Approve bool
		Note    string
		Result  string
	}
	// QueueItemDetail нь дараалал дахь нэг хүсэлтийн дэлгэрэнгүй — хүсэлт,
	// түүний үйлчилгээний тодорхойлолт, бүрэн timeline.
	QueueItemDetail struct {
		Application domain.GovApplication
		Service     *domain.GovService
		Events      []domain.GovApplicationEvent
	}
	ReferenceRequest struct {
		Type string
	}
	BookRequest struct {
		ServiceID   string
		ScheduledAt time.Time
		Location    string
		Note        string
	}
)

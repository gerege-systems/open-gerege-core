// Gerege Template Version 27.0
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

	// Нүүр
	Overview(ctx context.Context, userID string) (domain.GovOverview, error)

	// Хүсэлт
	ListApplications(ctx context.Context, userID string) ([]domain.GovApplication, error)
	Apply(ctx context.Context, userID string, in ApplyRequest) (domain.GovApplication, error)
	CancelApplication(ctx context.Context, userID, id string) error

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

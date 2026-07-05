// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gov

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/logger"
)

type usecase struct {
	repo repointerface.GovRepository
	now  func() time.Time
}

func NewUsecase(repo repointerface.GovRepository) Usecase {
	return &usecase{repo: repo, now: time.Now}
}

// ensureSeeded нь хэрэглэгч анх ороход (per-user өгөгдөл огт байхгүй үед) жишээ
// demo өгөгдлийг нэг удаа үүсгэнэ. Алдааг залгидаг — seed бүтэлгүйтсэн ч уншилт
// үргэлжилнэ (зүгээр хоосон харагдана).
func (uc *usecase) ensureSeeded(ctx context.Context, userID string) {
	n, err := uc.repo.CountUserRows(ctx, userID)
	if err != nil || n > 0 {
		return
	}
	if err := uc.repo.SeedDemoData(ctx, userID); err != nil {
		logger.WarnWithContext(ctx, "gov: demo seed failed", logger.Fields{"user_id": userID, "error": err.Error()})
	}
}

func (uc *usecase) ListServices(ctx context.Context) ([]domain.GovService, error) {
	return uc.repo.ListServices(ctx)
}

func (uc *usecase) Overview(ctx context.Context, userID string) (domain.GovOverview, error) {
	uc.ensureSeeded(ctx, userID)
	return uc.repo.Overview(ctx, userID)
}

// ── Applications ──────────────────────────────────────────────────────────—

func (uc *usecase) ListApplications(ctx context.Context, userID string) ([]domain.GovApplication, error) {
	uc.ensureSeeded(ctx, userID)
	return uc.repo.ListApplications(ctx, userID)
}

func (uc *usecase) Apply(ctx context.Context, userID string, in ApplyRequest) (domain.GovApplication, error) {
	if strings.TrimSpace(in.ServiceID) == "" {
		return domain.GovApplication{}, apperror.BadRequest("service is required")
	}
	svc, err := uc.repo.GetService(ctx, in.ServiceID)
	if err != nil {
		return domain.GovApplication{}, err
	}
	sid := svc.ID
	app := domain.GovApplication{
		UserID:      userID,
		ServiceID:   &sid,
		ServiceName: svc.Name,
		ReferenceNo: uc.refNo("APP"),
		Status:      "submitted",
		Note:        strings.TrimSpace(in.Note),
	}
	return uc.repo.CreateApplication(ctx, &app)
}

func (uc *usecase) CancelApplication(ctx context.Context, userID, id string) error {
	return uc.repo.SetApplicationStatus(ctx, userID, id, "cancelled")
}

// ── References ────────────────────────────────────────────────────────────—

// referenceTitles нь зөвшөөрөгдсөн лавлагааны төрөл → гарчиг.
var referenceTitles = map[string]string{
	"residence":  "Оршин суугаа газрын лавлагаа",
	"birth":      "Төрсний гэрчилгээний лавлагаа",
	"marriage":   "Гэрлэлтийн байдлын лавлагаа",
	"tax":        "Татварын тодорхойлолт",
	"social_ins": "Нийгмийн даатгалын лавлагаа",
	"criminal":   "Ял эдэлж байгаагүй тодорхойлолт",
}

func (uc *usecase) ListReferences(ctx context.Context, userID string) ([]domain.GovReference, error) {
	uc.ensureSeeded(ctx, userID)
	return uc.repo.ListReferences(ctx, userID)
}

func (uc *usecase) RequestReference(ctx context.Context, userID string, in ReferenceRequest) (domain.GovReference, error) {
	t := strings.ToLower(strings.TrimSpace(in.Type))
	title, ok := referenceTitles[t]
	if !ok {
		return domain.GovReference{}, apperror.BadRequest("unknown reference type: " + in.Type)
	}
	valid := uc.now().AddDate(0, 1, 0) // 30 хоног хүчинтэй
	ref := domain.GovReference{
		UserID:      userID,
		Type:        t,
		Title:       title,
		ReferenceNo: uc.refNo("REF"),
		Status:      "issued",
		ValidUntil:  &valid,
	}
	return uc.repo.CreateReference(ctx, &ref)
}

// ── Notifications ─────────────────────────────────────────────────────────—

func (uc *usecase) ListNotifications(ctx context.Context, userID string) ([]domain.GovNotification, error) {
	uc.ensureSeeded(ctx, userID)
	return uc.repo.ListNotifications(ctx, userID)
}

func (uc *usecase) MarkNotificationRead(ctx context.Context, userID, id string) error {
	return uc.repo.MarkNotificationRead(ctx, userID, id)
}

func (uc *usecase) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	return uc.repo.MarkAllNotificationsRead(ctx, userID)
}

// ── Payments ──────────────────────────────────────────────────────────────—

func (uc *usecase) ListPayments(ctx context.Context, userID string) ([]domain.GovPayment, error) {
	uc.ensureSeeded(ctx, userID)
	return uc.repo.ListPayments(ctx, userID)
}

func (uc *usecase) PayPayment(ctx context.Context, userID, id string) error {
	return uc.repo.PayPayment(ctx, userID, id)
}

// ── Appointments ──────────────────────────────────────────────────────────—

func (uc *usecase) ListAppointments(ctx context.Context, userID string) ([]domain.GovAppointment, error) {
	uc.ensureSeeded(ctx, userID)
	return uc.repo.ListAppointments(ctx, userID)
}

func (uc *usecase) BookAppointment(ctx context.Context, userID string, in BookRequest) (domain.GovAppointment, error) {
	if in.ScheduledAt.IsZero() {
		return domain.GovAppointment{}, apperror.BadRequest("scheduled time is required")
	}
	if in.ScheduledAt.Before(uc.now()) {
		return domain.GovAppointment{}, apperror.BadRequest("scheduled time must be in the future")
	}
	// Дээд хязгаар — 1 жилээс хол цагийг татгалзана (утгагүй/хог өгөгдлөөс сэргийлнэ).
	if in.ScheduledAt.After(uc.now().AddDate(1, 0, 0)) {
		return domain.GovAppointment{}, apperror.BadRequest("scheduled time is too far in the future")
	}
	appt := domain.GovAppointment{
		UserID:      userID,
		Location:    strings.TrimSpace(in.Location),
		ScheduledAt: in.ScheduledAt,
		Status:      "booked",
		Note:        strings.TrimSpace(in.Note),
	}
	if id := strings.TrimSpace(in.ServiceID); id != "" {
		svc, err := uc.repo.GetService(ctx, id)
		if err != nil {
			return domain.GovAppointment{}, err
		}
		appt.ServiceID = &svc.ID
		appt.ServiceName = svc.Name
		appt.Agency = svc.Agency
	}
	return uc.repo.CreateAppointment(ctx, &appt)
}

func (uc *usecase) CancelAppointment(ctx context.Context, userID, id string) error {
	return uc.repo.CancelAppointment(ctx, userID, id)
}

// refNo нь "PREFIX-YYYY-NNNNNN" хэлбэрийн лавлах дугаар үүсгэнэ (6 оронтой
// криптографийн санамсаргүй тоо).
func (uc *usecase) refNo(prefix string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	seq := int64(100000)
	if err == nil {
		seq += n.Int64()
	}
	return fmt.Sprintf("%s-%d-%06d", prefix, uc.now().Year(), seq)
}

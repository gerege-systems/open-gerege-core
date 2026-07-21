// Government Template Platform V3.0
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
	"template/internal/datasources/rls"
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

// Apply нь иргэний хүсэлтийг хүлээн авна. Энэ бол модулийн ГОЛ салаалт:
//
//	fulfilment = 'auto'   → үйлчилгээ ШУУД биелнэ. Хүсэлт, лавлагаа, мэдэгдэл
//	                        нэг транзакцид үүсч, төлөв шууд 'completed' болно.
//	                        Хүн оролцохгүй тул менежерийн дараалалд ОРОХГҮЙ.
//	fulfilment = 'manual' → хүсэлт бүртгэгдэж SLA цаг эхэлнэ, менежерийн
//	                        дараалалд орно. Иргэнд "хүлээн авсан" мэдэгдэл өгнө.
//
// Хоёр салааны ялгаа нь EU 2018/1724 Art.6(2)-той нийцнэ: гаралт шууд
// олгогдохгүй бол хүлээн авсан тухай автомат мэдэгдэл өгөх ёстой; шууд
// олгогдож байвал шаардлагагүй.
func (uc *usecase) Apply(ctx context.Context, userID string, in ApplyRequest) (ApplyResult, error) {
	if strings.TrimSpace(in.ServiceID) == "" {
		return ApplyResult{}, apperror.BadRequest("service is required")
	}
	svc, err := uc.repo.GetService(ctx, in.ServiceID)
	if err != nil {
		return ApplyResult{}, err
	}
	if !svc.Enabled || svc.Lifecycle != "active" {
		return ApplyResult{}, apperror.BadRequest("энэ үйлчилгээ идэвхгүй байна")
	}

	sid := svc.ID
	app := domain.GovApplication{
		UserID:      userID,
		ServiceID:   &sid,
		ServiceCode: svc.Code,
		ServiceName: svc.Name,
		ReferenceNo: uc.refNo("APP"),
		Note:        strings.TrimSpace(in.Note),
		Payload:     in.Payload,
	}

	if svc.Fulfilment == domain.GovFulfilmentAuto {
		return uc.applyAuto(ctx, svc, app)
	}
	return uc.applyManual(ctx, svc, app)
}

// applyAuto нь шууд биелэх үйлчилгээг гүйцэтгэнэ.
//
// Хамгаалалт: каталог auto гэж бичигдсэн ч үнэлэх эрх/үнэлгээний зай
// тэмдэглэгдсэн бол автоматаар шийдвэрлэхээс ТАТГАЛЗАЖ гараар хянуулна
// (§155(4) AO-ийн "флаг асвал хүнд шилжүүл" загвар). Ингэснээр каталогийн
// буруу тохиргоо хүний оролцоог чимээгүй алгасахаас сэргийлнэ.
func (uc *usecase) applyAuto(ctx context.Context, svc domain.GovService, app domain.GovApplication) (ApplyResult, error) {
	if svc.HasDiscretion || svc.HasAssessment {
		logger.WarnWithContext(ctx, "gov: auto үйлчилгээ үнэлэх эрхтэй тэмдэглэгдсэн — гараар хянуулна", logger.Fields{
			"service_code": svc.Code,
		})
		return uc.applyManual(ctx, svc, app)
	}

	now := uc.now()
	app.Status = domain.GovStatusCompleted
	app.Result = domain.GovResultProcessed
	app.DecidedAt = &now
	app.DecisionNote = "Улсын бүртгэлээс шууд олгогдов"

	var ref *domain.GovReference
	if svc.OutputRefType != "" {
		valid := now.AddDate(0, 1, 0) // 30 хоног хүчинтэй
		ref = &domain.GovReference{
			UserID:      app.UserID,
			Type:        svc.OutputRefType,
			Title:       svc.Name,
			ReferenceNo: uc.refNo("REF"),
			Status:      "issued",
			ValidUntil:  &valid,
		}
	}

	notify := &domain.GovNotification{
		UserID:   app.UserID,
		Title:    svc.Name + " бэлэн боллоо",
		Body:     "Таны хүссэн " + svc.Name + " амжилттай олгогдлоо. Лавлах дугаар: " + app.ReferenceNo,
		Category: "success",
	}

	outApp, outRef, err := uc.repo.CreateApplicationWithOutput(ctx, &app, ref, notify)
	if err != nil {
		return ApplyResult{}, err
	}
	res := ApplyResult{Application: outApp, AutoIssued: true}
	if ref != nil {
		res.Reference = &outRef
	}
	return res, nil
}

// applyManual нь менежерийн шийдвэр шаардах хүсэлтийг бүртгэнэ. SLA эцсийн
// хугацааг ЭНД нэг удаа тамгална — уншилт бүрт дахин тооцвол хугацаа "гулсаж"
// зөрчлийг нуух байсан.
func (uc *usecase) applyManual(ctx context.Context, svc domain.GovService, app domain.GovApplication) (ApplyResult, error) {
	app.Status = domain.GovStatusRegistered
	if svc.SLAHours > 0 {
		due := uc.now().Add(time.Duration(svc.SLAHours) * time.Hour)
		app.DueAt = &due
	}

	out, err := uc.repo.CreateApplication(ctx, &app)
	if err != nil {
		return ApplyResult{}, err
	}

	// Art.6(2)(b) — гаралт шууд олгогдоогүй тул хүлээн авсан тухай мэдэгдэл.
	body := "Таны " + svc.Name + " хүсэлт бүртгэгдлээ. Лавлах дугаар: " + out.ReferenceNo + "."
	if out.DueAt != nil {
		body += " Шийдвэрлэх хугацаа: " + out.DueAt.Format("2006-01-02 15:04") + "."
	}
	uc.notify(ctx, out.UserID, "Хүсэлт хүлээн авлаа", body, "info")

	return ApplyResult{Application: out, AutoIssued: false}, nil
}

func (uc *usecase) CancelApplication(ctx context.Context, userID, id string) error {
	return uc.repo.SetApplicationStatus(ctx, userID, id, domain.GovStatusCancelled)
}

func (uc *usecase) ApplicationTimeline(ctx context.Context, userID, id string) ([]domain.GovApplicationEvent, error) {
	// Эзэмшлийг эхлээд шалгана — timeline нь RLS-ээр хамгаалагдсан ч
	// "байхгүй" ба "чинийх биш" хоёрыг ижил 404-өөр хариулж, өөр хүний
	// хүсэлт байгаа эсэхийг тандахаас сэргийлнэ.
	if _, err := uc.repo.GetApplication(ctx, userID, id); err != nil {
		return nil, err
	}
	return uc.repo.ListApplicationEvents(ctx, id)
}

func (uc *usecase) ProvideInfo(ctx context.Context, userID, id, note string) (domain.GovApplication, error) {
	app, err := uc.repo.ResumeFromInfo(ctx, userID, id)
	if err != nil {
		return domain.GovApplication{}, err
	}
	if n := strings.TrimSpace(note); n != "" {
		uc.event(ctx, app.ID, userID, "user", app.Status, "info_note", n)
	}
	return app, nil
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

// ── Officer (менежер) ─────────────────────────────────────────────────────—

func (uc *usecase) QueueStats(ctx context.Context, officerID string) (domain.GovQueueStats, error) {
	return uc.repo.QueueStats(ctx, officerID)
}

func (uc *usecase) ListQueue(ctx context.Context, officerID string, f domain.GovQueueFilter) ([]domain.GovApplication, error) {
	// "me" нь UI-ийн товчлол — энд л баталгаажсан officerID болж хөрвөнө,
	// ингэснээр клиент өөр хүний ID-г шургуулах боломжгүй.
	if f.AssignedTo == "me" {
		f.AssignedTo = officerID
	} else if f.AssignedTo != "" {
		return nil, apperror.BadRequest("assigned_to нь зөвхөн 'me' байж болно")
	}
	if f.Status != "" && !domain.GovIsOpen(f.Status) && f.Status != domain.GovStatusCompleted && f.Status != domain.GovStatusRejected {
		return nil, apperror.BadRequest("тодорхойгүй төлөв: " + f.Status)
	}
	return uc.repo.ListQueue(ctx, f)
}

func (uc *usecase) QueueItem(ctx context.Context, id string) (QueueItemDetail, error) {
	app, err := uc.repo.GetApplicationAny(ctx, id)
	if err != nil {
		return QueueItemDetail{}, err
	}
	out := QueueItemDetail{Application: app}

	if app.ServiceID != nil {
		if svc, svcErr := uc.repo.GetService(ctx, *app.ServiceID); svcErr == nil {
			out.Service = &svc
		}
	}
	events, err := uc.repo.ListApplicationEvents(ctx, id)
	if err != nil {
		return QueueItemDetail{}, err
	}
	out.Events = events
	return out, nil
}

func (uc *usecase) Assign(ctx context.Context, officerID, id string) (domain.GovApplication, error) {
	return uc.repo.AssignApplication(ctx, id, officerID)
}

// Decide нь менежерийн эцсийн шийдвэрийг гүйцэтгэнэ. Зөвшөөрсөн тохиолдолд
// үйлчилгээний тодорхойлолт дээр үндэслэн гаралтыг (лавлагаа) үүсгэнэ.
func (uc *usecase) Decide(ctx context.Context, officerID, id string, in DecideRequest) (domain.GovApplication, error) {
	note := strings.TrimSpace(in.Note)
	// Татгалзах шийдвэр нь ҮРГЭЛЖ үндэслэлтэй байх ёстой — иргэн юунд
	// татгалзсаныг мэдэж, гомдол гаргах боломжтой байх нь наад захын шаардлага.
	if !in.Approve && note == "" {
		return domain.GovApplication{}, apperror.BadRequest("татгалзах үндэслэл заавал бичих ёстой")
	}

	app, err := uc.repo.GetApplicationAny(ctx, id)
	if err != nil {
		return domain.GovApplication{}, err
	}

	decision := repointerface.GovDecisionInput{
		ApplicationID: id,
		OfficerID:     officerID,
		Approve:       in.Approve,
		Note:          note,
		Result:        domain.GovResultRefused,
		Target:        domain.GovStatusRejected,
	}

	if in.Approve {
		decision.Result = domain.GovResultGranted
		if r := strings.TrimSpace(in.Result); r != "" {
			decision.Result = r
		}

		// Зөвшөөрсний дараа хүсэлт ДУУССАН эсэхийг гаралтын төрөл шийднэ:
		// лавлагаа/тодорхойлолт бол тэр дороо олгогдож дуусна; биет зүйл
		// (үнэмлэх, гэрчилгээ) бол хэвлэгдэж хүргэгдэх хүртэл 'approved'.
		decision.Target = domain.GovStatusApproved
		if app.ServiceID != nil {
			if svc, svcErr := uc.repo.GetService(ctx, *app.ServiceID); svcErr == nil && svc.OutputRefType != "" {
				valid := uc.now().AddDate(0, 1, 0)
				decision.OutputRef = &domain.GovReference{
					Type:        svc.OutputRefType,
					Title:       svc.Name,
					ReferenceNo: uc.refNo("REF"),
					Status:      "issued",
					ValidUntil:  &valid,
				}
				decision.Target = domain.GovStatusCompleted
			}
		}

		if decision.Target == domain.GovStatusCompleted {
			decision.Notify = &domain.GovNotification{
				Title:    app.ServiceName + " бэлэн боллоо",
				Body:     "Таны " + app.ServiceName + " хүсэлт (" + app.ReferenceNo + ") зөвшөөрөгдөж, гаралт олгогдлоо.",
				Category: "success",
			}
		} else {
			decision.Notify = &domain.GovNotification{
				Title:    app.ServiceName + " хүсэлт зөвшөөрөгдлөө",
				Body:     "Таны " + app.ServiceName + " хүсэлт (" + app.ReferenceNo + ") зөвшөөрөгдлөө. Бэлэн болмогц мэдэгдэнэ.",
				Category: "success",
			}
		}
	} else {
		decision.Notify = &domain.GovNotification{
			Title:    app.ServiceName + " хүсэлт татгалзагдлаа",
			Body:     "Таны " + app.ServiceName + " хүсэлт (" + app.ReferenceNo + ") татгалзагдлаа. Үндэслэл: " + note,
			Category: "warning",
		}
	}

	// Төлөвийн машиныг ЭНД шалгана — ойлгомжтой алдааны мэдэгдэл өгөхийн тулд.
	// Repository-ийн SQL `WHERE` guard нь үүнийг ДАХИН хэрэгжүүлдэг: тэр нь
	// уралдааныг (хоёр менежер зэрэг дарах) хаах давхарга, энэ нь дүрмийн
	// уншигдахуйц эх сурвалж. Хоёулаа хэрэгтэй.
	if !domain.GovCanTransition(app.Status, decision.Target) {
		return domain.GovApplication{}, apperror.Conflict(
			"'" + app.Status + "' төлөвөөс '" + decision.Target + "' руу шилжих боломжгүй")
	}
	return uc.repo.DecideApplication(ctx, decision)
}

// Complete нь биет гаралт хүргэгдсэнийг бүртгэж хүсэлтийг хаана.
func (uc *usecase) Complete(ctx context.Context, officerID, id string) (domain.GovApplication, error) {
	app, err := uc.repo.GetApplicationAny(ctx, id)
	if err != nil {
		return domain.GovApplication{}, err
	}
	notify := &domain.GovNotification{
		Title:    app.ServiceName + " бэлэн боллоо",
		Body:     "Таны " + app.ServiceName + " (" + app.ReferenceNo + ") бэлэн болж, гүйцэтгэл дууслаа.",
		Category: "success",
	}
	return uc.repo.CompleteApplication(ctx, id, officerID, notify)
}

func (uc *usecase) RequestInfo(ctx context.Context, officerID, id, note string) (domain.GovApplication, error) {
	n := strings.TrimSpace(note)
	if n == "" {
		return domain.GovApplication{}, apperror.BadRequest("ямар мэдээлэл дутуу байгааг тодорхой бичнэ үү")
	}
	app, err := uc.repo.RequestMoreInfo(ctx, id, officerID, n)
	if err != nil {
		return domain.GovApplication{}, err
	}
	uc.notify(ctx, app.UserID, app.ServiceName+" — нэмэлт мэдээлэл шаардлагатай",
		"Таны хүсэлт ("+app.ReferenceNo+") дээр нэмэлт мэдээлэл шаардагдаж байна: "+n+
			" Мэдээллийг ирүүлэх хүртэл шийдвэрлэх хугацаа зогсоно.", "warning")
	return app, nil
}

// ── SLA sweep (background worker) ─────────────────────────────────────────—

// SLASweep нь хоёр зүйлийг хийнэ:
//  1. Хугацаа хэтэрсэн хүсэлтийг нэг удаа тэмдэглэж иргэнд мэдэгдэнэ.
//  2. Чимээгүй зөвшөөрөл идэвхтэй үйлчилгээний хугацаа хэтэрсэн хүсэлтийг
//     зөвшөөрөгдсөнд тооцож иргэнд мэдэгдэнэ.
//
// Алдааг залгидаггүй ч нэг хүсэлтийн алдаа бусдыг зогсоохгүй — sweep нь
// давтагдан ажилладаг тул дараагийн эргэлтэд дахин оролдоно.
func (uc *usecase) SLASweep(ctx context.Context) error {
	// Sweep нь HTTP хүсэлт биш, background worker-ээс дуудагдана — context-д
	// ямар ч identity байхгүй. RLS нь identity-гүй үед БҮХ мөрийг хаадаг
	// (fail-closed) тул энд заавал системийн үүрэг тавина, эс тэгвээс sweep
	// чимээгүйхэн тэг мөр боловсруулж "ажиллаж байгаа мэт" харагдана.
	ctx = rls.WithService(ctx)

	breached, err := uc.repo.MarkSLABreached(ctx)
	if err != nil {
		logger.ErrorWithContext(ctx, "gov: SLA breach sweep failed", logger.Fields{"error": err.Error()})
	}
	for _, a := range breached {
		uc.notify(ctx, a.UserID, a.ServiceName+" — хугацаа хэтэрлээ",
			"Таны хүсэлт ("+a.ReferenceNo+") шийдвэрлэх хугацаа хэтэрсэн байна. Байгууллага яаралтай хянана.",
			"warning")
		uc.event(ctx, a.ID, "", "system", a.Status, "sla_breached", "Шийдвэрлэх хугацаа хэтэрлээ")
	}

	tacit, err := uc.repo.TacitApprovals(ctx)
	if err != nil {
		logger.ErrorWithContext(ctx, "gov: tacit approval sweep failed", logger.Fields{"error": err.Error()})
		return nil
	}
	for _, a := range tacit {
		uc.notify(ctx, a.UserID, a.ServiceName+" — зөвшөөрөгдсөнд тооцов",
			"Таны хүсэлт ("+a.ReferenceNo+") хуулийн хугацаанд шийдвэрлэгдээгүй тул зөвшөөрсөнд тооцлоо. "+
				"Энэ шийдвэр автоматаар гарсан болно.", "success")
		uc.event(ctx, a.ID, "", "system", a.Status, "tacit_approved",
			"Хугацаа хэтэрсэн тул чимээгүй зөвшөөрлөөр шийдэгдэв")
	}
	return nil
}

// ── Туслах ────────────────────────────────────────────────────────────────—

// notify нь иргэнд мэдэгдэл бичнэ (best-effort — үндсэн үйлдлийг блоклохгүй).
func (uc *usecase) notify(ctx context.Context, userID, title, body, category string) {
	err := uc.repo.CreateNotification(ctx, &domain.GovNotification{
		UserID: userID, Title: title, Body: body, Category: category,
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "gov: notification write failed (non-fatal)", logger.Fields{
			"user_id": userID, "error": err.Error(),
		})
	}
}

// event нь timeline бичлэг нэмнэ (best-effort).
func (uc *usecase) event(ctx context.Context, appID, actorID, actorRole, status, typ, detail string) {
	e := &domain.GovApplicationEvent{
		ApplicationID: appID, ActorRole: actorRole,
		ToStatus: status, Type: typ, Detail: detail,
	}
	if actorID != "" {
		e.ActorID = &actorID
	}
	if err := uc.repo.AppendApplicationEvent(ctx, e); err != nil {
		logger.ErrorWithContext(ctx, "gov: timeline write failed (non-fatal)", logger.Fields{
			"application_id": appID, "error": err.Error(),
		})
	}
}

func (uc *usecase) ListLifeEvents(ctx context.Context) ([]domain.GovLifeEvent, error) {
	return uc.repo.ListLifeEvents(ctx)
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

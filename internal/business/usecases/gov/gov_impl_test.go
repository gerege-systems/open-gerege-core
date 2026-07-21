// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gov

import (
	"context"
	"testing"
	"time"

	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

// fakeRepo нь GovRepository-ийн санах-ой хувилбар. Зөвхөн энэ тестийн хөндөх
// хэсгүүд утга буцаана; бусад нь тэг утга (usecase тэднийг дуудахгүй).
type fakeRepo struct {
	svc          domain.GovService
	queuedStatus string // GetApplicationAny буцаах төлөв (өгөгдмөл: in_review)

	// бичигдсэн үр дүн
	createdApp    *domain.GovApplication
	withOutputApp *domain.GovApplication
	withOutputRef *domain.GovReference
	notifications []domain.GovNotification
	decision      *repointerface.GovDecisionInput
}

func (f *fakeRepo) GetService(context.Context, string) (domain.GovService, error) {
	return f.svc, nil
}

func (f *fakeRepo) CreateApplication(_ context.Context, in *domain.GovApplication) (domain.GovApplication, error) {
	cp := *in
	cp.ID = "app-1"
	f.createdApp = &cp
	return cp, nil
}

func (f *fakeRepo) CreateApplicationWithOutput(_ context.Context, app *domain.GovApplication, ref *domain.GovReference, notify *domain.GovNotification) (domain.GovApplication, domain.GovReference, error) {
	cp := *app
	cp.ID = "app-auto"
	f.withOutputApp = &cp
	f.withOutputRef = ref
	if notify != nil {
		f.notifications = append(f.notifications, *notify)
	}
	var outRef domain.GovReference
	if ref != nil {
		outRef = *ref
		outRef.ID = "ref-1"
	}
	return cp, outRef, nil
}

func (f *fakeRepo) CreateNotification(_ context.Context, in *domain.GovNotification) error {
	f.notifications = append(f.notifications, *in)
	return nil
}

func (f *fakeRepo) GetApplicationAny(context.Context, string) (domain.GovApplication, error) {
	a := domain.GovApplication{
		ID: "app-1", ServiceName: "Тест", ReferenceNo: "APP-1",
		Status: domain.GovStatusInReview,
	}
	if f.queuedStatus != "" {
		a.Status = f.queuedStatus
	}
	if f.svc.ID != "" {
		sid := f.svc.ID
		a.ServiceID = &sid
	}
	return a, nil
}

func (f *fakeRepo) DecideApplication(_ context.Context, in repointerface.GovDecisionInput) (domain.GovApplication, error) {
	f.decision = &in
	return domain.GovApplication{ID: in.ApplicationID, Status: domain.GovStatusCompleted}, nil
}

// ── Хөндөгдөхгүй методууд ───────────────────────────────────────────────────

func (f *fakeRepo) ListServices(context.Context) ([]domain.GovService, error)     { return nil, nil }
func (f *fakeRepo) ListLifeEvents(context.Context) ([]domain.GovLifeEvent, error) { return nil, nil }
func (f *fakeRepo) ListApplications(context.Context, string) ([]domain.GovApplication, error) {
	return nil, nil
}
func (f *fakeRepo) GetApplication(context.Context, string, string) (domain.GovApplication, error) {
	return domain.GovApplication{}, nil
}
func (f *fakeRepo) SetApplicationStatus(context.Context, string, string, string) error { return nil }
func (f *fakeRepo) QueueStats(context.Context, string) (domain.GovQueueStats, error) {
	return domain.GovQueueStats{}, nil
}
func (f *fakeRepo) ListQueue(context.Context, domain.GovQueueFilter) ([]domain.GovApplication, error) {
	return nil, nil
}
func (f *fakeRepo) AssignApplication(context.Context, string, string) (domain.GovApplication, error) {
	return domain.GovApplication{}, nil
}
func (f *fakeRepo) RequestMoreInfo(context.Context, string, string, string) (domain.GovApplication, error) {
	return domain.GovApplication{}, nil
}
func (f *fakeRepo) ResumeFromInfo(context.Context, string, string) (domain.GovApplication, error) {
	return domain.GovApplication{}, nil
}
func (f *fakeRepo) AppendApplicationEvent(context.Context, *domain.GovApplicationEvent) error {
	return nil
}
func (f *fakeRepo) ListApplicationEvents(context.Context, string) ([]domain.GovApplicationEvent, error) {
	return nil, nil
}
func (f *fakeRepo) MarkSLABreached(context.Context) ([]domain.GovApplication, error) {
	return nil, nil
}
func (f *fakeRepo) TacitApprovals(context.Context) ([]domain.GovApplication, error) { return nil, nil }
func (f *fakeRepo) ListReferences(context.Context, string) ([]domain.GovReference, error) {
	return nil, nil
}
func (f *fakeRepo) CreateReference(context.Context, *domain.GovReference) (domain.GovReference, error) {
	return domain.GovReference{}, nil
}
func (f *fakeRepo) ListNotifications(context.Context, string) ([]domain.GovNotification, error) {
	return nil, nil
}
func (f *fakeRepo) MarkNotificationRead(context.Context, string, string) error { return nil }
func (f *fakeRepo) MarkAllNotificationsRead(context.Context, string) error     { return nil }
func (f *fakeRepo) ListPayments(context.Context, string) ([]domain.GovPayment, error) {
	return nil, nil
}
func (f *fakeRepo) PayPayment(context.Context, string, string) error { return nil }
func (f *fakeRepo) ListAppointments(context.Context, string) ([]domain.GovAppointment, error) {
	return nil, nil
}
func (f *fakeRepo) CreateAppointment(context.Context, *domain.GovAppointment) (domain.GovAppointment, error) {
	return domain.GovAppointment{}, nil
}
func (f *fakeRepo) CancelAppointment(context.Context, string, string) error { return nil }
func (f *fakeRepo) Overview(context.Context, string) (domain.GovOverview, error) {
	return domain.GovOverview{}, nil
}
func (f *fakeRepo) CountUserRows(context.Context, string) (int, error) { return 1, nil }
func (f *fakeRepo) SeedDemoData(context.Context, string) error         { return nil }

func newUC(f *fakeRepo) *usecase {
	return &usecase{repo: f, now: func() time.Time { return time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC) }}
}

// ── Тестүүд ─────────────────────────────────────────────────────────────────

// auto горимын үйлчилгээ нь менежерийн дараалалд ОРОХГҮЙ, нэг транзакцид
// биелж лавлагаа олгох ёстой.
func TestApplyAutoIssuesImmediately(t *testing.T) {
	f := &fakeRepo{svc: domain.GovService{
		ID: "svc-1", Code: "MN-0133-002", Name: "Оршин суугаа газрын лавлагаа",
		Fulfilment: domain.GovFulfilmentAuto, OutputRefType: "residence",
		Enabled: true, Lifecycle: "active",
	}}
	res, err := newUC(f).Apply(context.Background(), "user-1", ApplyRequest{ServiceID: "svc-1"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !res.AutoIssued {
		t.Error("auto үйлчилгээ AutoIssued=true байх ёстой")
	}
	if f.withOutputApp == nil {
		t.Fatal("CreateApplicationWithOutput дуудагдаагүй — атомар биш байна")
	}
	if got := f.withOutputApp.Status; got != domain.GovStatusCompleted {
		t.Errorf("төлөв = %q, хүсэн хүлээсэн %q", got, domain.GovStatusCompleted)
	}
	if got := f.withOutputApp.Result; got != domain.GovResultProcessed {
		t.Errorf("үр дүн = %q, хүсэн хүлээсэн %q", got, domain.GovResultProcessed)
	}
	if f.withOutputApp.DueAt != nil {
		t.Error("шууд биелсэн үйлчилгээнд SLA хугацаа байх учиргүй")
	}
	if res.Reference == nil {
		t.Error("лавлагаа олгогдоогүй")
	}
	if f.createdApp != nil {
		t.Error("auto үйлчилгээ энгийн CreateApplication руу орох ёсгүй")
	}
}

// manual горим нь SLA хугацаа тамгалж, иргэнд "хүлээн авлаа" мэдэгдэл өгөх
// ёстой (EU 2018/1724 Art.6(2)(b)-ийн загвар).
func TestApplyManualQueuesWithDeadline(t *testing.T) {
	f := &fakeRepo{svc: domain.GovService{
		ID: "svc-2", Code: "MN-0451-001", Name: "Жолооны үнэмлэх сунгах",
		Fulfilment: domain.GovFulfilmentManual, SLAHours: 120,
		Enabled: true, Lifecycle: "active",
	}}
	res, err := newUC(f).Apply(context.Background(), "user-1", ApplyRequest{ServiceID: "svc-2"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if res.AutoIssued {
		t.Error("manual үйлчилгээ AutoIssued=false байх ёстой")
	}
	if f.createdApp == nil {
		t.Fatal("CreateApplication дуудагдаагүй")
	}
	if got := f.createdApp.Status; got != domain.GovStatusRegistered {
		t.Errorf("төлөв = %q, хүсэн хүлээсэн %q", got, domain.GovStatusRegistered)
	}
	if f.createdApp.DueAt == nil {
		t.Fatal("SLA хугацаа тамгалагдаагүй")
	}
	want := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC) // +120 цаг
	if !f.createdApp.DueAt.Equal(want) {
		t.Errorf("due_at = %v, хүсэн хүлээсэн %v", f.createdApp.DueAt, want)
	}
	if len(f.notifications) != 1 {
		t.Fatalf("хүлээн авсан мэдэгдэл = %d, хүсэн хүлээсэн 1", len(f.notifications))
	}
	if f.withOutputApp != nil {
		t.Error("manual үйлчилгээ шууд биелэх ёсгүй")
	}
}

// Каталог auto гэж бичигдсэн ч үнэлэх эрх тэмдэглэгдсэн бол автоматаар
// шийдвэрлэхээс ТАТГАЛЗАЖ гараар хянуулах ёстой — буруу тохиргоо хүний
// оролцоог чимээгүй алгасах эрсдэлээс хамгаална.
func TestApplyAutoWithDiscretionFallsBackToManual(t *testing.T) {
	f := &fakeRepo{svc: domain.GovService{
		ID: "svc-3", Name: "Буруу тохируулсан үйлчилгээ",
		Fulfilment: domain.GovFulfilmentAuto, HasDiscretion: true, SLAHours: 48,
		Enabled: true, Lifecycle: "active",
	}}
	res, err := newUC(f).Apply(context.Background(), "user-1", ApplyRequest{ServiceID: "svc-3"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.AutoIssued {
		t.Error("үнэлэх эрхтэй үйлчилгээ автоматаар биелэх ЁСГҮЙ")
	}
	if f.withOutputApp != nil {
		t.Error("автомат биелүүлэлт дуудагдсан — хамгаалалт ажиллаагүй")
	}
	if f.createdApp == nil || f.createdApp.Status != domain.GovStatusRegistered {
		t.Error("гараар хянах дараалалд ороогүй")
	}
}

// Идэвхгүй үйлчилгээнд хүсэлт хүлээж авахгүй.
func TestApplyRejectsInactiveService(t *testing.T) {
	f := &fakeRepo{svc: domain.GovService{
		ID: "svc-4", Fulfilment: domain.GovFulfilmentAuto, Enabled: false, Lifecycle: "active",
	}}
	if _, err := newUC(f).Apply(context.Background(), "user-1", ApplyRequest{ServiceID: "svc-4"}); err == nil {
		t.Error("идэвхгүй үйлчилгээ татгалзагдах ёстой")
	}
}

// Татгалзах шийдвэр нь үндэслэлгүй байж болохгүй — иргэн юунд татгалзсаныг
// мэдэж, гомдол гаргах эрхтэй.
func TestDecideRejectRequiresReason(t *testing.T) {
	f := &fakeRepo{}
	_, err := newUC(f).Decide(context.Background(), "officer-1", "app-1", DecideRequest{Approve: false})
	if err == nil {
		t.Fatal("үндэслэлгүй татгалзал зөвшөөрөгдөх ёсгүй")
	}
	if f.decision != nil {
		t.Error("баталгаажаагүй шийдвэр DB рүү бичигдсэн")
	}
}

// Лавлагаа гаргадаг үйлчилгээ зөвшөөрөгдвөл гаралт тэр дороо олгогдож хүсэлт
// ДУУСНА (completed) — завсрын төлөвт саатахгүй.
func TestDecideApproveWithDocumentOutputCompletes(t *testing.T) {
	f := &fakeRepo{svc: domain.GovService{
		ID: "svc-1", Name: "Ял эдэлж байгаагүй тодорхойлолт", OutputRefType: "criminal",
	}}
	_, err := newUC(f).Decide(context.Background(), "officer-1", "app-1", DecideRequest{Approve: true})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if f.decision == nil {
		t.Fatal("шийдвэр бичигдээгүй")
	}
	if f.decision.Result != domain.GovResultGranted {
		t.Errorf("үр дүн = %q, хүсэн хүлээсэн %q", f.decision.Result, domain.GovResultGranted)
	}
	if f.decision.Target != domain.GovStatusCompleted {
		t.Errorf("зорилтот төлөв = %q, хүсэн хүлээсэн %q", f.decision.Target, domain.GovStatusCompleted)
	}
	if f.decision.OutputRef == nil {
		t.Error("лавлагаа олгогдоогүй")
	}
	if f.decision.Notify == nil {
		t.Error("иргэнд мэдэгдэл бэлдээгүй")
	}
	if f.decision.OfficerID != "officer-1" {
		t.Errorf("шийдвэрлэгч = %q", f.decision.OfficerID)
	}
}

// БИЕТ гаралттай үйлчилгээ (үнэмлэх) зөвшөөрөгдөхөд хүсэлт ДУУСАХГҮЙ —
// хэвлэгдэж хүргэгдэх хүртэл 'approved' төлөвт хүлээнэ.
func TestDecideApproveWithPhysicalOutputAwaitsDelivery(t *testing.T) {
	f := &fakeRepo{svc: domain.GovService{
		ID: "svc-9", Name: "Иргэний үнэмлэх захиалах", OutputType: "Physical object",
		OutputRefType: "", // лавлагаа гардаггүй — биет зүйл
	}}
	_, err := newUC(f).Decide(context.Background(), "officer-1", "app-1", DecideRequest{Approve: true})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if f.decision.Target != domain.GovStatusApproved {
		t.Errorf("зорилтот төлөв = %q, хүсэн хүлээсэн %q", f.decision.Target, domain.GovStatusApproved)
	}
	if f.decision.OutputRef != nil {
		t.Error("биет гаралтад лавлагаа үүсгэх ёсгүй")
	}
}

// Аль хэдийн шийдэгдсэн хүсэлтийг дахин шийдэж болохгүй.
func TestDecideRejectsTerminalApplication(t *testing.T) {
	f := &fakeRepo{queuedStatus: domain.GovStatusCompleted}
	_, err := newUC(f).Decide(context.Background(), "officer-1", "app-1", DecideRequest{Approve: true})
	if err == nil {
		t.Fatal("дууссан хүсэлтийг дахин шийдэх нь татгалзагдах ёстой")
	}
	if f.decision != nil {
		t.Error("хүчингүй шилжилт DB рүү бичигдсэн")
	}
}

// Менежер өөр хүний нэрээр дараалал шүүж чадахгүй — 'me' л зөвшөөрөгдөнө.
func TestListQueueRejectsForeignAssignee(t *testing.T) {
	uc := newUC(&fakeRepo{})
	if _, err := uc.ListQueue(context.Background(), "officer-1", domain.GovQueueFilter{AssignedTo: "someone-else"}); err == nil {
		t.Error("өөр хүний ID-гаар шүүх нь татгалзагдах ёстой")
	}
	if _, err := uc.ListQueue(context.Background(), "officer-1", domain.GovQueueFilter{AssignedTo: "me"}); err != nil {
		t.Errorf("'me' шүүлтүүр ажиллах ёстой: %v", err)
	}
}

func (f *fakeRepo) CompleteApplication(_ context.Context, id, _ string, _ *domain.GovNotification) (domain.GovApplication, error) {
	return domain.GovApplication{ID: id, Status: domain.GovStatusCompleted}, nil
}

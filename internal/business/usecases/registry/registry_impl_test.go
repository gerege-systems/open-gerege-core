// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Регистрийн usecase-ийн unit тест — DB шаардлагагүй (fake repository).
// Гол шалгах зүйлс: паспортын валидаци, once-only зөрчил илрүүлэлт, нийтлэлт
// дэх baseline delta тооцоолол, регистрийн үнэн зөвийг хамгаалах хамгаалалтууд.
package registry

import (
	"context"
	"errors"
	"testing"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

// ── Fake repository ─────────────────────────────────────────────────────────

type fakeRepo struct {
	repointerface.RegistryRepository // хэрэглээгүй метод дуудвал panic (тестийн алдааг ил гаргана)

	svc      domain.RegistryService
	baseline *domain.RegistryServiceVersion
	docs     int

	lastFilter  repointerface.RegistryFilter
	published   *domain.RegistryServiceVersion
	deletedID   string
	statusCalls []string
}

func (f *fakeRepo) ListServices(_ context.Context, filter repointerface.RegistryFilter) ([]domain.RegistryService, error) {
	f.lastFilter = filter
	return []domain.RegistryService{f.svc}, nil
}

func (f *fakeRepo) GetService(_ context.Context, id string) (domain.RegistryService, error) {
	if f.svc.ID != id {
		return domain.RegistryService{}, apperror.NotFound("service not found")
	}
	return f.svc, nil
}

func (f *fakeRepo) CountCitizenDocuments(context.Context, string) (int, error) { return f.docs, nil }

func (f *fakeRepo) BaselineVersion(context.Context, string) (domain.RegistryServiceVersion, error) {
	if f.baseline == nil {
		return domain.RegistryServiceVersion{}, apperror.NotFound("baseline not found")
	}
	return *f.baseline, nil
}

func (f *fakeRepo) PublishVersion(_ context.Context, in *domain.RegistryServiceVersion) (domain.RegistryServiceVersion, error) {
	cp := *in
	cp.Version = 1
	f.published = &cp
	return cp, nil
}

func (f *fakeRepo) CreateService(_ context.Context, in *domain.RegistryService) (domain.RegistryService, error) {
	return *in, nil
}

func (f *fakeRepo) DeleteService(_ context.Context, id string) error {
	f.deletedID = id
	return nil
}

func (f *fakeRepo) SetServiceStatus(_ context.Context, _, status string) error {
	f.statusCalls = append(f.statusCalls, status)
	return nil
}

// validInput нь бүх шаардлагыг хангасан жишиг оролт.
func validInput() ServiceInput {
	return ServiceInput{
		Code:        "RS_TEST",
		Name:        "Туршилтын үйлчилгээ",
		Authority:   "УБЕГ",
		Channels:    []string{"office", "e-mongolia"},
		Fee:         1000,
		MaxDays:     5,
		StepsCount:  4,
		Proactivity: domain.ProactivityOnline,
	}
}

// wantErrType нь алдаа тухайн domain төрөлтэй эсэхийг шалгана.
func wantErrType(t *testing.T, err error, typ apperror.ErrorType) {
	t.Helper()
	if err == nil {
		t.Fatalf("алдаа хүлээсэн, гэвч nil ирлээ")
	}
	var de *apperror.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("DomainError хүлээсэн, гэвч %T ирлээ: %v", err, err)
	}
	if de.Type != typ {
		t.Fatalf("алдааны төрөл: авсан %v, хүлээсэн %v (%s)", de.Type, typ, de.Message)
	}
}

// ── Валидаци ────────────────────────────────────────────────────────────────

func TestCreateServiceValidation(t *testing.T) {
	cases := map[string]func(*ServiceInput){
		"код буруу хэлбэртэй":         func(in *ServiceInput) { in.Code = "rs test!" },
		"нэр хоосон":                  func(in *ServiceInput) { in.Name = "  " },
		"эрх бүхий байгууллагагүй":    func(in *ServiceInput) { in.Authority = "" },
		"суваг тодорхойгүй":           func(in *ServiceInput) { in.Channels = []string{"telegram"} },
		"проактив шат тодорхойгүй":    func(in *ServiceInput) { in.Proactivity = "magic" },
		"сөрөг төлбөр":                func(in *ServiceInput) { in.Fee = -1 },
		"хугацаа хязгаараас хэтэрсэн": func(in *ServiceInput) { in.MaxDays = maxDaysLimit + 1 },
	}
	uc := NewUsecase(&fakeRepo{})
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			mutate(&in)
			_, err := uc.CreateService(context.Background(), in)
			wantErrType(t, err, apperror.ErrTypeBadRequest)
		})
	}
}

func TestCreateServiceNormalizes(t *testing.T) {
	uc := NewUsecase(&fakeRepo{})
	in := validInput()
	in.Code = " rs_test "
	in.Name = "  Туршилт  "
	in.Channels = []string{"OFFICE", "office", " e-mongolia "}

	got, err := uc.CreateService(context.Background(), in)
	if err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if got.Code != "RS_TEST" {
		t.Errorf("код: авсан %q, хүлээсэн %q", got.Code, "RS_TEST")
	}
	if got.Name != "Туршилт" {
		t.Errorf("нэр: авсан %q", got.Name)
	}
	if len(got.Channels) != 2 {
		t.Errorf("суваг: давхардал арилаагүй — %v", got.Channels)
	}
	// Шинэ паспорт ҮРГЭЛЖ ноорогоор эхэлнэ (нийтлэлт нь тусдаа үйлдэл).
	if got.Status != domain.RegistryStatusDraft {
		t.Errorf("статус: авсан %q, хүлээсэн %q", got.Status, domain.RegistryStatusDraft)
	}
}

// ── Once-only ───────────────────────────────────────────────────────────────

func TestCheckOnceOnly(t *testing.T) {
	repo := &fakeRepo{svc: domain.RegistryService{
		ID: "svc-1", Code: "RS_TEST", Name: "Туршилт", Proactivity: domain.ProactivityOnline,
		Evidences: []domain.RegistryServiceEvidence{
			{EvidenceID: "e1", Code: "EV_APPLICATION", FromCitizen: true, InKHUR: false},
			{EvidenceID: "e2", Code: "EV_RESIDENCE", FromCitizen: true, InKHUR: true},  // ⚠ зөрчил
			{EvidenceID: "e3", Code: "EV_CIVIL_ID", FromCitizen: true, InKHUR: true},   // ⚠ зөрчил
			{EvidenceID: "e4", Code: "EV_TAX_CLEAR", FromCitizen: false, InKHUR: true}, // ✓ системээс татдаг
		},
	}}
	rep, err := NewUsecase(repo).CheckOnceOnly(context.Background(), "svc-1")
	if err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if rep.CitizenDocuments != 3 {
		t.Errorf("иргэнээс шаардсан баримт: авсан %d, хүлээсэн 3", rep.CitizenDocuments)
	}
	if len(rep.Violations) != 2 {
		t.Fatalf("зөрчил: авсан %d, хүлээсэн 2", len(rep.Violations))
	}
	if rep.Compliant {
		t.Error("зөрчилтэй атал compliant=true")
	}
}

func TestCheckOnceOnlyCompliant(t *testing.T) {
	repo := &fakeRepo{svc: domain.RegistryService{
		ID: "svc-1", Proactivity: domain.ProactivityOnceOnly,
		Evidences: []domain.RegistryServiceEvidence{
			{EvidenceID: "e1", FromCitizen: true, InKHUR: false},
			{EvidenceID: "e2", FromCitizen: false, InKHUR: true},
		},
	}}
	rep, err := NewUsecase(repo).CheckOnceOnly(context.Background(), "svc-1")
	if err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if !rep.Compliant {
		t.Error("зөрчилгүй атал compliant=false")
	}
	if rep.EligibleProactivity != domain.ProactivityOnceOnly {
		t.Errorf("боломжит шат: авсан %q, хүлээсэн %q", rep.EligibleProactivity, domain.ProactivityOnceOnly)
	}
}

// ── Нийтлэлт ба baseline delta ──────────────────────────────────────────────

func TestPublishFirstBecomesBaseline(t *testing.T) {
	repo := &fakeRepo{
		svc:  domain.RegistryService{ID: "svc-1", Proactivity: domain.ProactivityOnline, StepsCount: 11, MaxDays: 10, Fee: 44000},
		docs: 4,
	}
	got, err := NewUsecase(repo).Publish(context.Background(), "svc-1", PublishInput{ChangeNote: "Анхны бүртгэл"})
	if err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if !got.IsBaseline {
		t.Error("эхний нийтлэлт baseline биш байна")
	}
	if got.DeltaSteps != 0 || got.DeltaDocuments != 0 || got.DeltaDays != 0 || got.DeltaFee != 0 {
		t.Errorf("baseline дээр delta тэг байх ёстой: %+v", got)
	}
	if got.DocumentsCount != 4 {
		t.Errorf("баримтын тоо: авсан %d, хүлээсэн 4", got.DocumentsCount)
	}
	if len(got.Snapshot) == 0 {
		t.Error("snapshot хоосон байна")
	}
}

func TestPublishComputesDeltaAgainstBaseline(t *testing.T) {
	// Дахин инженерчлэлийн дараа: 11→6 алхам, 4→2 баримт, 10→3 хоног.
	repo := &fakeRepo{
		svc:  domain.RegistryService{ID: "svc-1", Proactivity: domain.ProactivityOnline, StepsCount: 6, MaxDays: 3, Fee: 44000},
		docs: 2,
		baseline: &domain.RegistryServiceVersion{
			Version: 1, IsBaseline: true, StepsCount: 11, DocumentsCount: 4, MaxDays: 10, Fee: 44000,
		},
	}
	got, err := NewUsecase(repo).Publish(context.Background(), "svc-1", PublishInput{ChangeNote: "ZGB кампанит ажил"})
	if err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if got.IsBaseline {
		t.Error("хоёр дахь нийтлэлт baseline болжээ")
	}
	// Сөрөг delta = сайжралт.
	if got.DeltaSteps != -5 {
		t.Errorf("delta_steps: авсан %d, хүлээсэн -5", got.DeltaSteps)
	}
	if got.DeltaDocuments != -2 {
		t.Errorf("delta_documents: авсан %d, хүлээсэн -2", got.DeltaDocuments)
	}
	if got.DeltaDays != -7 {
		t.Errorf("delta_days: авсан %d, хүлээсэн -7", got.DeltaDays)
	}
	if got.DeltaFee != 0 {
		t.Errorf("delta_fee: авсан %d, хүлээсэн 0", got.DeltaFee)
	}
}

// Зөрчилтэй атал "once_only" гэж зарлаж нийтлэхийг хориглоно — регистр өөрөө
// худал мэдээлэл агуулах ёсгүй.
func TestPublishRejectsOverclaimedProactivity(t *testing.T) {
	repo := &fakeRepo{svc: domain.RegistryService{
		ID: "svc-1", Proactivity: domain.ProactivityOnceOnly,
		Evidences: []domain.RegistryServiceEvidence{
			{EvidenceID: "e1", FromCitizen: true, InKHUR: true}, // ⚠ зөрчил
		},
	}}
	_, err := NewUsecase(repo).Publish(context.Background(), "svc-1", PublishInput{})
	wantErrType(t, err, apperror.ErrTypeConflict)
	if repo.published != nil {
		t.Error("татгалзсан атал хувилбар бичигдсэн байна")
	}
}

func TestPublishRejectsArchived(t *testing.T) {
	repo := &fakeRepo{svc: domain.RegistryService{ID: "svc-1", Status: domain.RegistryStatusArchived}}
	_, err := NewUsecase(repo).Publish(context.Background(), "svc-1", PublishInput{})
	wantErrType(t, err, apperror.ErrTypeConflict)
}

// ── Устгалтын хамгаалалт ────────────────────────────────────────────────────

func TestDeletePublishedServiceRejected(t *testing.T) {
	repo := &fakeRepo{svc: domain.RegistryService{ID: "svc-1", Status: domain.RegistryStatusPublished}}
	err := NewUsecase(repo).DeleteService(context.Background(), "svc-1")
	wantErrType(t, err, apperror.ErrTypeConflict)
	if repo.deletedID != "" {
		t.Error("нийтлэгдсэн паспорт устгагдсан байна")
	}
}

func TestDeleteDraftServiceAllowed(t *testing.T) {
	repo := &fakeRepo{svc: domain.RegistryService{ID: "svc-1", Status: domain.RegistryStatusDraft}}
	if err := NewUsecase(repo).DeleteService(context.Background(), "svc-1"); err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if repo.deletedID != "svc-1" {
		t.Error("ноорог паспорт устгагдаагүй")
	}
}

// ── Нийтийн каталог ─────────────────────────────────────────────────────────

// PublicCatalog нь дуудагч ямар статус хүссэнээс үл хамааран зөвхөн
// нийтлэгдсэн паспортыг эргүүлнэ (ноорог гадагш гарахгүй).
func TestPublicCatalogForcesPublishedOnly(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := NewUsecase(repo).PublicCatalog(context.Background(), ListFilter{Status: "draft"}); err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if !repo.lastFilter.PublishedOnly {
		t.Error("PublishedOnly тавигдаагүй")
	}
	if repo.lastFilter.Status != "" {
		t.Errorf("дуудагчийн статус шүүлтүүр дамжсан: %q", repo.lastFilter.Status)
	}
}

func TestArchiveServiceSetsStatus(t *testing.T) {
	repo := &fakeRepo{svc: domain.RegistryService{ID: "svc-1", Status: domain.RegistryStatusPublished}}
	if err := NewUsecase(repo).ArchiveService(context.Background(), "svc-1"); err != nil {
		t.Fatalf("санаандгүй алдаа: %v", err)
	}
	if len(repo.statusCalls) != 1 || repo.statusCalls[0] != domain.RegistryStatusArchived {
		t.Errorf("статусын дуудалт: %v", repo.statusCalls)
	}
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package registry нь Ring System · R1 — Үйлчилгээний нэгдсэн регистрийн
// business logic: CPSV-AP үйлчилгээний паспорт, нотолгооны каталог ба ХУР
// mapping, once-only зөрчил илрүүлэгч, паспортын хувилбар + baseline delta,
// амьдралын үйл явдлын давхарга (RING_SYSTEM_PLAN.md §R1).
package registry

import (
	"context"

	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

// ListFilter нь паспортын жагсаалтын шүүлтүүр (HTTP query-гээс).
type ListFilter struct {
	Status      string
	Authority   string
	LifeEventID string
	Proactivity string
	Query       string
}

// ServiceInput нь паспорт үүсгэх/засах оролт. Code нь зөвхөн үүсгэх үед
// хэрэглэгдэнэ (паспортын код өөрчлөгддөггүй — түүхэн мөрдөлт тасрахаас
// сэргийлнэ).
type ServiceInput struct {
	Code           string
	Name           string
	NameEN         string
	Description    string
	Authority      string
	AuthorityOrgID *string
	LegalBasis     string
	TargetGroup    string
	Output         string
	Channels       []string
	Fee            int
	MaxDays        int
	StepsCount     int
	AnnualVolume   int
	Proactivity    string
	LifeEventID    *string
}

// EvidenceLink нь паспортод нотолгоо холбох мөр.
type EvidenceLink struct {
	EvidenceID  string
	Required    bool
	FromCitizen bool
	Note        string
}

// EvidenceInput нь нотолгооны каталогийн бичлэг.
type EvidenceInput struct {
	Code            string
	Name            string
	Description     string
	HolderAgency    string
	SourceSystem    string
	InKHUR          bool
	KHURServiceCode string
}

// LifeEventInput нь амьдралын/бизнесийн үйл явдлын бичлэг.
type LifeEventInput struct {
	Code        string
	Name        string
	Kind        string
	Description string
	LeadAgency  string
	SortOrder   int
}

// PublishInput нь паспортыг нийтлэх (шинэ хувилбар үүсгэх) оролт.
type PublishInput struct {
	ChangeNote  string
	PublishedBy *string
}

// OnceOnlyReport нь нэг үйлчилгээний once-only шалгалтын дүн.
type OnceOnlyReport struct {
	ServiceID   string
	ServiceCode string
	ServiceName string
	// CitizenDocuments — иргэнээс шаардаж буй нийт баримтын тоо.
	CitizenDocuments int
	// Violations — тэдгээрээс ХУР-д АЛЬ ХЭДИЙН байгаа нь (=устгах боломжтой).
	Violations []domain.RegistryServiceEvidence
	// Compliant нь зөрчилгүй эсэх.
	Compliant bool
	// EligibleProactivity нь одоогийн зөрчлийн байдалд хүрч болох дээд шат.
	EligibleProactivity string
}

// Usecase нь регистрийн бүх үйлдэл. Уншилтын Public* хувилбарууд нь зөвхөн
// нийтлэгдсэн (published) паспортыг харуулна — иргэн рүү харсан каталогид.
type Usecase interface {
	// ── Паспорт ─────────────────────────────────────────────────────────
	ListServices(ctx context.Context, f ListFilter) ([]domain.RegistryService, error)
	PublicCatalog(ctx context.Context, f ListFilter) ([]domain.RegistryService, error)
	// PublicService нь иргэн рүү харсан дэлгэрэнгүй — нийтлэгдээгүй паспортыг
	// NotFound болгоно (ноорог гадагш гарахгүй).
	PublicService(ctx context.Context, id string) (domain.RegistryService, error)
	GetService(ctx context.Context, id string) (domain.RegistryService, error)
	CreateService(ctx context.Context, in ServiceInput) (domain.RegistryService, error)
	UpdateService(ctx context.Context, id string, in ServiceInput) (domain.RegistryService, error)
	DeleteService(ctx context.Context, id string) error
	ArchiveService(ctx context.Context, id string) error

	// ── Нотолгооны холбоос + хувилбар ───────────────────────────────────
	SetEvidences(ctx context.Context, serviceID string, list []EvidenceLink) (domain.RegistryService, error)
	Publish(ctx context.Context, serviceID string, in PublishInput) (domain.RegistryServiceVersion, error)
	ListVersions(ctx context.Context, serviceID string) ([]domain.RegistryServiceVersion, error)

	// ── Нотолгооны каталог ──────────────────────────────────────────────
	ListEvidences(ctx context.Context) ([]domain.RegistryEvidence, error)
	CreateEvidence(ctx context.Context, in EvidenceInput) (domain.RegistryEvidence, error)
	UpdateEvidence(ctx context.Context, id string, in EvidenceInput) (domain.RegistryEvidence, error)
	DeleteEvidence(ctx context.Context, id string) error

	// ── Амьдралын үйл явдал ─────────────────────────────────────────────
	ListLifeEvents(ctx context.Context) ([]domain.RegistryLifeEvent, error)
	CreateLifeEvent(ctx context.Context, in LifeEventInput) (domain.RegistryLifeEvent, error)
	DeleteLifeEvent(ctx context.Context, id string) error

	// ── Once-only ба нэгтгэл ────────────────────────────────────────────
	OnceOnlyViolations(ctx context.Context, authority string) ([]domain.RegistryOnceOnlyViolation, error)
	CheckOnceOnly(ctx context.Context, serviceID string) (OnceOnlyReport, error)
	Overview(ctx context.Context) (domain.RegistryOverview, error)
}

// NewUsecase нь регистрийн usecase-ийг репозиторын интерфэйсээр угсарна
// (postgres адаптерээс хамааралгүй — Clean Architecture).
func NewUsecase(repo repointerface.RegistryRepository) Usecase {
	return &usecase{repo: repo}
}

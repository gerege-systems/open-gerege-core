// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// Ring System · R1 — Үйлчилгээний нэгдсэн регистрийн домэйн entity-үүд
// (migration 42). Эдгээр нь байгууллагын мастер өгөгдөл (хэрэглэгч-тус-бүрийн
// БИШ) тул RLS-гүй; хамгаалалт нь 'registry.manage' эрхээр HTTP давхаргад.

// Паспортын статус.
const (
	RegistryStatusDraft     = "draft"
	RegistryStatusPublished = "published"
	RegistryStatusArchived  = "archived"
)

// Проактив байдлын шат (Эстони загвар) — мэдээллээс автомат үйлчилгээ хүртэл.
const (
	ProactivityInformation = "information" // зөвхөн мэдээлэл нийтэлсэн
	ProactivityOnline      = "online"      // онлайн өргөдөл авдаг
	ProactivityOnceOnly    = "once_only"   // байгаа өгөгдлийг дахин шаарддаггүй
	ProactivityProactive   = "proactive"   // иргэн хүсэлт гаргалгүй өөрөө санал болгодог
)

// RegistryLifeEvent нь амьдралын/бизнесийн үйл явдал (төрөлт, гэрлэлт,
// бизнес эхлүүлэх…) — үйлчилгээнүүдийг журнейгээр багцалдаг давхарга.
type RegistryLifeEvent struct {
	ID          string
	Code        string
	Name        string
	Kind        string // life | business
	Description string
	LeadAgency  string
	SortOrder   int
	CreatedAt   time.Time
}

// RegistryEvidence нь нотолгооны каталогийн нэг бичиг баримт. InKHUR нь уг
// мэдээлэл ХУР-д аль хэдийн байгаа эсэхийг заана — once-only шалгалтын үндэс.
type RegistryEvidence struct {
	ID              string
	Code            string
	Name            string
	Description     string
	HolderAgency    string
	SourceSystem    string
	InKHUR          bool
	KHURServiceCode string
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

// RegistryServiceEvidence нь паспорт ↔ нотолгооны холбоос. FromCitizen нь уг
// баримтыг ИРГЭНЭЭС шаардаж байгаа эсэх (эсрэг тохиолдолд байгууллага өөрөө
// системээс татдаг).
type RegistryServiceEvidence struct {
	EvidenceID string
	Code       string
	Name       string
	Required   bool
	// FromCitizen нь иргэнээс шаардаж буй эсэх.
	FromCitizen bool
	// InKHUR нь ХУР-д байгаа эсэх (evidence-ээс уншигдана).
	InKHUR bool
	Note   string
}

// RegistryService нь CPSV-AP-д нийцсэн үйлчилгээний паспорт.
type RegistryService struct {
	ID             string
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
	Fee            int // MNT
	MaxDays        int // хуулийн шийдвэрлэх дээд хугацаа
	StepsCount     int
	AnnualVolume   int
	Proactivity    string
	Status         string
	LifeEventID    *string
	Version        int
	PublishedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      *time.Time

	// Дэлгэрэнгүй уншилтад л дүүргэгдэнэ (жагсаалтад хоосон).
	Evidences []RegistryServiceEvidence
}

// RegistryServiceVersion нь нийтлэгдсэн паспортын хувилбар. Delta* талбарууд
// нь baseline (дахин инженерчлэлийн өмнөх төлөв)-тэй харьцуулсан ялгаа —
// сөрөг утга нь сайжралт.
type RegistryServiceVersion struct {
	ID             string
	ServiceID      string
	Version        int
	Snapshot       []byte // jsonb
	ChangeNote     string
	IsBaseline     bool
	StepsCount     int
	DocumentsCount int
	MaxDays        int
	Fee            int
	DeltaSteps     int
	DeltaDocuments int
	DeltaDays      int
	DeltaFee       int
	PublishedAt    time.Time
	PublishedBy    *string
}

// RegistryOnceOnlyViolation нь ХУР-д БАЙГАА мэдээллийг иргэнээс дахин шаардаж
// буй нэг тохиолдол.
type RegistryOnceOnlyViolation struct {
	ServiceID       string
	ServiceCode     string
	ServiceName     string
	Authority       string
	ServiceStatus   string
	EvidenceID      string
	EvidenceCode    string
	EvidenceName    string
	HolderAgency    string
	KHURServiceCode string
	AnnualVolume    int
}

// RegistryOverview нь регистрийн удирдлагын нэгтгэл — "төрийн үйлчилгээний
// инвентар хэр бүрэн, хэр дижитал, once-only-д хэр ойрхон вэ".
type RegistryOverview struct {
	TotalServices     int
	PublishedServices int
	DraftServices     int
	LifeEvents        int
	Evidences         int
	EvidencesInKHUR   int
	// Once-only зөрчлийн тоо ба тэдгээрийн жилийн нийт давтамж (иргэдэд
	// учирч буй дарамтын хэмжээст ойролцоолол).
	OnceOnlyViolations int
	OnceOnlyAnnualHits int
	// Проактив байдлын шат бүрээр үйлчилгээний тоо.
	ByProactivity map[string]int
	// Дундаж хуулийн шийдвэрлэх хугацаа (нийтлэгдсэн үйлчилгээнүүдээр).
	AvgMaxDays float64
}

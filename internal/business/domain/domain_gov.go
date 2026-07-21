// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// Иргэний "Төрийн үйлчилгээ" порталын домэйн entity-үүд (me систем). gov_services
// нь нийтийн каталог; бусад нь хэрэглэгч-тус-бүрийн (UserID-гаар scope хийгдэнэ).

// ── Үйлчилгээний биелүүлэх горим ─────────────────────────────────────────────

const (
	// GovFulfilmentAuto нь бүртгэлээс шууд уншиж олгодог үйлчилгээ (лавлагаа,
	// тодорхойлолт). Хүн оролцохгүй — хүсэлт өгмөгц нэг транзакцид биелнэ.
	GovFulfilmentAuto = "auto"
	// GovFulfilmentManual нь менежер (officer) хянаж шийдвэрлэсний дараа
	// биелэх үйлчилгээ.
	GovFulfilmentManual = "manual"
)

// ── Хүсэлтийн төлөвийн машин (migration 44-ийн CHECK-тэй таарна) ─────────────

const (
	GovStatusSubmitted    = "submitted"     // иргэн илгээв
	GovStatusRegistered   = "registered"    // албан ёсоор бүртгэгдэв, SLA эхлэв
	GovStatusInReview     = "in_review"     // менежер хянаж байна
	GovStatusInfoRequired = "info_required" // иргэнээс нэмэлт баримт хүлээж байна
	GovStatusApproved     = "approved"      // зөвшөөрөв
	GovStatusRejected     = "rejected"      // татгалзав
	GovStatusCompleted    = "completed"     // гаралт олгогдов
	GovStatusCancelled    = "cancelled"     // иргэн буцаав
	GovStatusExpired      = "expired"       // хугацаа дуусав
)

// Үр дүнгийн толь. Голландын ZGW-ийн сургамж: ЯВЦЫН толийг (status) байгууллага
// өөрөө тодорхойлдог ч ҮР ДҮНГИЙН толийг улсын хэмжээнд нэгтгэдэг — тайлан,
// статистик үүн дээр тогтдог.
const (
	GovResultGranted       = "granted"        // олгов
	GovResultRefused       = "refused"        // татгалзав
	GovResultWithdrawn     = "withdrawn"      // иргэн татав
	GovResultNotAdmissible = "not_admissible" // хүлээн авах боломжгүй
	GovResultProcessed     = "processed"      // шийдвэргүйгээр боловсруулав (auto)
)

// govTransitions нь зөвшөөрөгдсөн шилжилтүүд. Энэ нь ГАНЦ эх сурвалж —
// repository-ийн SQL нь эдгээрийг `WHERE status IN (...)` болгон давхар
// хэрэгжүүлж, зэрэг ирсэн хоёр шийдвэрийн уралдааныг (race) хаана.
// Шийдвэрлэх төлвүүдээс 'approved' болон 'completed' ХОЁУЛАА зөвшөөрөгдөнө:
//
//	→ completed : гаралт нь тэр дороо олгогдох боломжтой (лавлагаа, тодорхойлолт).
//	              Шийдвэр ба хүргэлт нэг үйлдэл тул завсрын төлөв утгагүй.
//	→ approved  : гаралт нь БИЕТ зүйл (үнэмлэх, гэрчилгээ). Шийдвэр гарсан ч
//	              хэвлэгдэж/хүргэгдэх хүртэл дуусаагүй. Дараа нь completed болно.
//
// Хоёрдмол байдлыг зайлуулах нь чухал: өмнө нь 'approved' нь хэрэглэгддэггүй
// чимэглэл байсан бол одоо тодорхой утгатай — "шийдэгдсэн, хүргэгдээгүй".
var govTransitions = map[string][]string{
	GovStatusSubmitted:    {GovStatusRegistered, GovStatusCancelled},
	GovStatusRegistered:   {GovStatusInReview, GovStatusInfoRequired, GovStatusApproved, GovStatusCompleted, GovStatusRejected, GovStatusCancelled, GovStatusExpired},
	GovStatusInReview:     {GovStatusInfoRequired, GovStatusApproved, GovStatusCompleted, GovStatusRejected, GovStatusCancelled, GovStatusExpired},
	GovStatusInfoRequired: {GovStatusInReview, GovStatusApproved, GovStatusCompleted, GovStatusRejected, GovStatusCancelled, GovStatusExpired},
	GovStatusApproved:     {GovStatusCompleted},
	// Терминал төлөвүүд — цаашид шилжихгүй.
	GovStatusRejected:  {},
	GovStatusCompleted: {},
	GovStatusCancelled: {},
	GovStatusExpired:   {},
}

// GovCanTransition нь from → to шилжилт зөвшөөрөгдсөн эсэхийг хэлнэ.
func GovCanTransition(from, to string) bool {
	for _, allowed := range govTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// GovIsOpen нь хүсэлт ХАРААХАН шийдэгдээгүй (менежерийн дараалалд байгаа)
// эсэхийг хэлнэ. Overview-ийн тоолол болон SLA sweep үүнийг ашиглана.
func GovIsOpen(status string) bool {
	switch status {
	case GovStatusSubmitted, GovStatusRegistered, GovStatusInReview, GovStatusInfoRequired:
		return true
	}
	return false
}

// GovService нь каталогийн нэг үйлчилгээ. Талбарууд нь CPSV-AP 3.2.0 (SEMIC)
// -ийн Public Service класстай зэрэгцүүлэгдсэн — тайлбарт харгалзах property-г
// бичив. Namespace: cv: = http://data.europa.eu/m8g/ (migration 44).
type GovService struct {
	ID             string
	Code           string // dct:identifier — MN-<COFOG>-<дугаар>
	Name           string // dct:title
	Category       string
	Agency         string // cv:hasCompetentAuthority
	Description    string // dct:description
	Fee            int    // cv:hasCost (MNT)
	ProcessingDays int
	ProcessingTime string // cv:processingTime — ISO 8601 duration (хуулийн хугацаа)
	COFOGCode      string // НҮБ COFOG 1999 — CPSV-AP-д шууд харгалзах property байхгүй
	COFOGLabel     string
	MainActivity   string   // dct:type — ЕХ main-activity authority table
	SDGCode        string   // SDG Annex II procedure код (SEMIC codelist)
	OutputType     string   // cpsv:produces — CPSV-AP Output толь
	OutputRefType  string   // гаралт лавлагаа бол gov_references.type
	Evidence       []string // cpsv:hasInput
	LegalBasis     string   // cv:hasLegalResource
	AssuranceLevel string   // eIDAS: low/substantial/high (CPSV-AP-д байхгүй өргөтгөл)
	Lifecycle      string   // adms:status
	Fulfilment     string   // auto | manual
	HasDiscretion  bool     // үнэлэх эрх (Ermessen) байгаа эсэх
	HasAssessment  bool     // үнэлгээний зай (Beurteilungsspielraum) байгаа эсэх
	SLAHours       int      // үйлчилгээний норм (ZGW `servicenorm`)
	TacitApproval  bool
	LifeEvents     []GovLifeEvent // cv:isGroupedBy
	Online         bool
	Enabled        bool
	CreatedAt      time.Time
}

// GovLifeEvent нь CPSV-AP-ийн Event (Life/Business). EUCode нь ЕХ-ны хяналттай
// толийн код: life → ox8/life-event/LE, business → m58/business-event/BE.
type GovLifeEvent struct {
	Code    string
	Name    string
	Kind    string // life | business
	EUCode  string
	ENLabel string
}

// GovApplication нь иргэний үйлчилгээний хүсэлт.
type GovApplication struct {
	ID           string
	UserID       string
	ServiceID    *string
	ServiceCode  string
	ServiceName  string
	ReferenceNo  string
	Status       string
	Result       string
	Note         string
	Payload      []byte // jsonb — маягтын өгөгдөл
	AssignedTo   *string
	AssignedAt   *time.Time
	DecidedBy    *string
	DecidedAt    *time.Time
	DecisionNote string
	DueAt        *time.Time
	SLABreached  bool
	SuspendedAt  *time.Time // цаг зогссон мөч (ZGW `Opschorting`)
	OutputRefID  *string
	Tacit        bool
	SubmittedAt  time.Time
	UpdatedAt    *time.Time
}

// GovApplicationEvent нь хүсэлтийн timeline-ийн нэг бичлэг (append-only).
type GovApplicationEvent struct {
	ID            string
	ApplicationID string
	ActorID       *string
	ActorRole     string
	FromStatus    string
	ToStatus      string
	Type          string
	Detail        string
	CreatedAt     time.Time
}

// GovReference нь олгогдсон лавлагаа/тодорхойлолт.
type GovReference struct {
	ID          string
	UserID      string
	Type        string
	Title       string
	ReferenceNo string
	Status      string
	IssuedAt    time.Time
	ValidUntil  *time.Time
	Data        []byte // jsonb
}

// GovNotification нь иргэнд илгээсэн мэдэгдэл.
type GovNotification struct {
	ID        string
	UserID    string
	Title     string
	Body      string
	Category  string
	Read      bool
	CreatedAt time.Time
}

// GovPayment нь төлбөр (татвар/хураамж/торгууль).
type GovPayment struct {
	ID        string
	UserID    string
	Title     string
	Category  string
	Amount    int
	Currency  string
	Status    string
	DueDate   *time.Time
	PaidAt    *time.Time
	CreatedAt time.Time
}

// GovAppointment нь төрийн байгууллага дахь цаг захиалга.
type GovAppointment struct {
	ID          string
	UserID      string
	ServiceID   *string
	ServiceName string
	Agency      string
	Location    string
	ScheduledAt time.Time
	Status      string
	Note        string
	CreatedAt   time.Time
}

// GovOverview нь иргэний нүүр хуудасны нэгтгэл.
type GovOverview struct {
	OpenApplications     int
	UnreadNotifications  int
	UnpaidCount          int
	UnpaidAmount         int
	UpcomingCount        int
	IssuedReferences     int
	RecentApplications   []GovApplication
	UpcomingAppointments []GovAppointment
}

// GovQueueStats нь МЕНЕЖЕРИЙН дарааллын нэгтгэл (иргэний Overview-ээс тусдаа —
// өөр эрх, өөр хамрах хүрээ).
type GovQueueStats struct {
	Open       int // нээлттэй бүх хүсэлт
	Unassigned int // хараахан хэн ч аваагүй
	Mine       int // тухайн менежерт оногдсон
	Overdue    int // хугацаа хэтэрсэн
	DueSoon    int // 24 цагийн дотор дуусах
}

// GovQueueFilter нь менежерийн дарааллын шүүлтүүр.
type GovQueueFilter struct {
	Status     string // хоосон бол бүх НЭЭЛТТЭЙ төлөв
	AssignedTo string // "me" гэсэн утгыг usecase нь userID болгож хөрвүүлнэ
	Overdue    bool
	Limit      int
	Offset     int
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"encoding/json"
	"time"

	"template/internal/business/domain"
)

// ── Services ────────────────────────────────────────────────────────────────

type GovLifeEventResponse struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	EUCode  string `json:"eu_code"`
	ENLabel string `json:"en_label"`
}

func ToGovLifeEventList(list []domain.GovLifeEvent) []GovLifeEventResponse {
	out := make([]GovLifeEventResponse, 0, len(list))
	for _, e := range list {
		out = append(out, GovLifeEventResponse{
			Code: e.Code, Name: e.Name, Kind: e.Kind, EUCode: e.EUCode, ENLabel: e.ENLabel,
		})
	}
	return out
}

type GovServiceResponse struct {
	ID             string                 `json:"id"`
	Code           string                 `json:"code"`
	Name           string                 `json:"name"`
	Category       string                 `json:"category"`
	Agency         string                 `json:"agency"`
	Description    string                 `json:"description"`
	Fee            int                    `json:"fee"`
	ProcessingDays int                    `json:"processing_days"`
	ProcessingTime string                 `json:"processing_time"`
	COFOGCode      string                 `json:"cofog_code"`
	COFOGLabel     string                 `json:"cofog_label"`
	SDGCode        string                 `json:"sdg_code"`
	OutputType     string                 `json:"output_type"`
	Evidence       []string               `json:"evidence"`
	LegalBasis     string                 `json:"legal_basis"`
	AssuranceLevel string                 `json:"assurance_level"`
	Fulfilment     string                 `json:"fulfilment"`
	SLAHours       int                    `json:"sla_hours"`
	TacitApproval  bool                   `json:"tacit_approval"`
	LifeEvents     []GovLifeEventResponse `json:"life_events"`
	Online         bool                   `json:"online"`
}

func FromGovService(s domain.GovService) GovServiceResponse {
	ev := s.Evidence
	if ev == nil {
		ev = []string{}
	}
	return GovServiceResponse{
		ID: s.ID, Code: s.Code, Name: s.Name, Category: s.Category, Agency: s.Agency,
		Description: s.Description, Fee: s.Fee, ProcessingDays: s.ProcessingDays,
		ProcessingTime: s.ProcessingTime, COFOGCode: s.COFOGCode, COFOGLabel: s.COFOGLabel,
		SDGCode: s.SDGCode, OutputType: s.OutputType, Evidence: ev, LegalBasis: s.LegalBasis,
		AssuranceLevel: s.AssuranceLevel, Fulfilment: s.Fulfilment, SLAHours: s.SLAHours,
		TacitApproval: s.TacitApproval, LifeEvents: ToGovLifeEventList(s.LifeEvents),
		Online: s.Online,
	}
}

func ToGovServiceList(list []domain.GovService) []GovServiceResponse {
	out := make([]GovServiceResponse, 0, len(list))
	for _, s := range list {
		out = append(out, FromGovService(s))
	}
	return out
}

// ── Applications ──────────────────────────────────────────────────────────—

type GovApplicationResponse struct {
	ID           string     `json:"id"`
	ServiceCode  string     `json:"service_code"`
	ServiceName  string     `json:"service_name"`
	ReferenceNo  string     `json:"reference_no"`
	Status       string     `json:"status"`
	Result       string     `json:"result"`
	Note         string     `json:"note"`
	DecisionNote string     `json:"decision_note"`
	DueAt        *time.Time `json:"due_at"`
	SLABreached  bool       `json:"sla_breached"`
	Suspended    bool       `json:"suspended"`
	Assigned     bool       `json:"assigned"`
	Tacit        bool       `json:"tacit"`
	OutputRefID  *string    `json:"output_ref_id"`
	SubmittedAt  time.Time  `json:"submitted_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

// FromGovApplication нь ИРГЭНД харагдах хэлбэр. assigned_to / decided_by зэрэг
// албан хаагчийн ХУВИЙН таних мэдээллийг ил гаргахгүй — зөвхөн хариуцагчтай
// эсэх (assigned) л хангалттай.
func FromGovApplication(a domain.GovApplication) GovApplicationResponse {
	return GovApplicationResponse{
		ID: a.ID, ServiceCode: a.ServiceCode, ServiceName: a.ServiceName,
		ReferenceNo: a.ReferenceNo, Status: a.Status, Result: a.Result, Note: a.Note,
		DecisionNote: a.DecisionNote, DueAt: a.DueAt, SLABreached: a.SLABreached,
		Suspended: a.SuspendedAt != nil, Assigned: a.AssignedTo != nil, Tacit: a.Tacit,
		OutputRefID: a.OutputRefID, SubmittedAt: a.SubmittedAt, UpdatedAt: a.UpdatedAt,
	}
}

// ── Timeline / Officer queue ──────────────────────────────────────────────—

type GovApplicationEventResponse struct {
	ID         string    `json:"id"`
	ActorRole  string    `json:"actor_role"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Type       string    `json:"type"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToGovApplicationEventList нь timeline-г буцаана. ActorID-г ЗОРИУДААР
// оруулаагүй — иргэнд аль албан хаагч гэдгийг биш, ямар үүрэгтэй хүн үйлдэл
// хийснийг л харуулна.
func ToGovApplicationEventList(list []domain.GovApplicationEvent) []GovApplicationEventResponse {
	out := make([]GovApplicationEventResponse, 0, len(list))
	for _, e := range list {
		out = append(out, GovApplicationEventResponse{
			ID: e.ID, ActorRole: e.ActorRole, FromStatus: e.FromStatus,
			ToStatus: e.ToStatus, Type: e.Type, Detail: e.Detail, CreatedAt: e.CreatedAt,
		})
	}
	return out
}

type GovQueueStatsResponse struct {
	Open       int `json:"open"`
	Unassigned int `json:"unassigned"`
	Mine       int `json:"mine"`
	Overdue    int `json:"overdue"`
	DueSoon    int `json:"due_soon"`
}

func FromGovQueueStats(s domain.GovQueueStats) GovQueueStatsResponse {
	return GovQueueStatsResponse{
		Open: s.Open, Unassigned: s.Unassigned, Mine: s.Mine,
		Overdue: s.Overdue, DueSoon: s.DueSoon,
	}
}

// GovQueueItemResponse нь МЕНЕЖЕРТ харагдах хэлбэр — иргэнийхээс илүү
// талбартай (хэн хариуцаж байгаа, маягтын өгөгдөл).
type GovQueueItemResponse struct {
	GovApplicationResponse
	UserID     string          `json:"user_id"`
	AssignedTo *string         `json:"assigned_to"`
	AssignedAt *time.Time      `json:"assigned_at"`
	DecidedBy  *string         `json:"decided_by"`
	DecidedAt  *time.Time      `json:"decided_at"`
	Payload    json.RawMessage `json:"payload"`
}

func FromGovQueueItem(a domain.GovApplication) GovQueueItemResponse {
	payload := json.RawMessage(a.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	return GovQueueItemResponse{
		GovApplicationResponse: FromGovApplication(a),
		UserID:                 a.UserID,
		AssignedTo:             a.AssignedTo,
		AssignedAt:             a.AssignedAt,
		DecidedBy:              a.DecidedBy,
		DecidedAt:              a.DecidedAt,
		Payload:                payload,
	}
}

func ToGovQueueList(list []domain.GovApplication) []GovQueueItemResponse {
	out := make([]GovQueueItemResponse, 0, len(list))
	for _, a := range list {
		out = append(out, FromGovQueueItem(a))
	}
	return out
}

// GovQueueDetailResponse нь дарааллын нэг мөрийн дэлгэрэнгүй.
type GovQueueDetailResponse struct {
	Application GovQueueItemResponse          `json:"application"`
	Service     *GovServiceResponse           `json:"service"`
	Events      []GovApplicationEventResponse `json:"events"`
}

// GovApplyResponse нь хүсэлтийн үр дүн. auto_issued=true бол үйлчилгээ ШУУД
// биелсэн бөгөөд reference нь олгогдсон лавлагаа.
type GovApplyResponse struct {
	Application GovApplicationResponse `json:"application"`
	Reference   *GovReferenceResponse  `json:"reference"`
	AutoIssued  bool                   `json:"auto_issued"`
}

func ToGovApplicationList(list []domain.GovApplication) []GovApplicationResponse {
	out := make([]GovApplicationResponse, 0, len(list))
	for _, a := range list {
		out = append(out, FromGovApplication(a))
	}
	return out
}

// ── References ────────────────────────────────────────────────────────────—

type GovReferenceResponse struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	ReferenceNo string          `json:"reference_no"`
	Status      string          `json:"status"`
	IssuedAt    time.Time       `json:"issued_at"`
	ValidUntil  *time.Time      `json:"valid_until"`
	Data        json.RawMessage `json:"data"`
}

func FromGovReference(r domain.GovReference) GovReferenceResponse {
	data := json.RawMessage(r.Data)
	if len(data) == 0 {
		data = json.RawMessage("{}")
	}
	return GovReferenceResponse{
		ID: r.ID, Type: r.Type, Title: r.Title, ReferenceNo: r.ReferenceNo,
		Status: r.Status, IssuedAt: r.IssuedAt, ValidUntil: r.ValidUntil, Data: data,
	}
}

func ToGovReferenceList(list []domain.GovReference) []GovReferenceResponse {
	out := make([]GovReferenceResponse, 0, len(list))
	for _, r := range list {
		out = append(out, FromGovReference(r))
	}
	return out
}

// ── Notifications ─────────────────────────────────────────────────────────—

type GovNotificationResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Category  string    `json:"category"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

func ToGovNotificationList(list []domain.GovNotification) []GovNotificationResponse {
	out := make([]GovNotificationResponse, 0, len(list))
	for _, n := range list {
		out = append(out, GovNotificationResponse{
			ID: n.ID, Title: n.Title, Body: n.Body, Category: n.Category, Read: n.Read, CreatedAt: n.CreatedAt,
		})
	}
	return out
}

// ── Payments ──────────────────────────────────────────────────────────────—

type GovPaymentResponse struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Category  string     `json:"category"`
	Amount    int        `json:"amount"`
	Currency  string     `json:"currency"`
	Status    string     `json:"status"`
	DueDate   *time.Time `json:"due_date"`
	PaidAt    *time.Time `json:"paid_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func ToGovPaymentList(list []domain.GovPayment) []GovPaymentResponse {
	out := make([]GovPaymentResponse, 0, len(list))
	for _, p := range list {
		out = append(out, GovPaymentResponse{
			ID: p.ID, Title: p.Title, Category: p.Category, Amount: p.Amount, Currency: p.Currency,
			Status: p.Status, DueDate: p.DueDate, PaidAt: p.PaidAt, CreatedAt: p.CreatedAt,
		})
	}
	return out
}

// ── Appointments ──────────────────────────────────────────────────────────—

type GovAppointmentResponse struct {
	ID          string    `json:"id"`
	ServiceName string    `json:"service_name"`
	Agency      string    `json:"agency"`
	Location    string    `json:"location"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Status      string    `json:"status"`
	Note        string    `json:"note"`
}

func FromGovAppointment(a domain.GovAppointment) GovAppointmentResponse {
	return GovAppointmentResponse{
		ID: a.ID, ServiceName: a.ServiceName, Agency: a.Agency, Location: a.Location,
		ScheduledAt: a.ScheduledAt, Status: a.Status, Note: a.Note,
	}
}

func ToGovAppointmentList(list []domain.GovAppointment) []GovAppointmentResponse {
	out := make([]GovAppointmentResponse, 0, len(list))
	for _, a := range list {
		out = append(out, FromGovAppointment(a))
	}
	return out
}

// ── Overview ──────────────────────────────────────────────────────────────—

type GovOverviewResponse struct {
	OpenApplications     int                      `json:"open_applications"`
	UnreadNotifications  int                      `json:"unread_notifications"`
	UnpaidCount          int                      `json:"unpaid_count"`
	UnpaidAmount         int                      `json:"unpaid_amount"`
	UpcomingCount        int                      `json:"upcoming_count"`
	IssuedReferences     int                      `json:"issued_references"`
	RecentApplications   []GovApplicationResponse `json:"recent_applications"`
	UpcomingAppointments []GovAppointmentResponse `json:"upcoming_appointments"`
}

func FromGovOverview(o domain.GovOverview) GovOverviewResponse {
	return GovOverviewResponse{
		OpenApplications: o.OpenApplications, UnreadNotifications: o.UnreadNotifications,
		UnpaidCount: o.UnpaidCount, UnpaidAmount: o.UnpaidAmount, UpcomingCount: o.UpcomingCount,
		IssuedReferences:     o.IssuedReferences,
		RecentApplications:   ToGovApplicationList(o.RecentApplications),
		UpcomingAppointments: ToGovAppointmentList(o.UpcomingAppointments),
	}
}

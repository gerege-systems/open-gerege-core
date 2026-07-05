// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"encoding/json"
	"time"

	"template/internal/business/domain"
)

// ── Services ────────────────────────────────────────────────────────────────

type GovServiceResponse struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	Agency         string `json:"agency"`
	Description    string `json:"description"`
	Fee            int    `json:"fee"`
	ProcessingDays int    `json:"processing_days"`
	Online         bool   `json:"online"`
}

func ToGovServiceList(list []domain.GovService) []GovServiceResponse {
	out := make([]GovServiceResponse, 0, len(list))
	for _, s := range list {
		out = append(out, GovServiceResponse{
			ID: s.ID, Code: s.Code, Name: s.Name, Category: s.Category, Agency: s.Agency,
			Description: s.Description, Fee: s.Fee, ProcessingDays: s.ProcessingDays, Online: s.Online,
		})
	}
	return out
}

// ── Applications ──────────────────────────────────────────────────────────—

type GovApplicationResponse struct {
	ID          string     `json:"id"`
	ServiceName string     `json:"service_name"`
	ReferenceNo string     `json:"reference_no"`
	Status      string     `json:"status"`
	Note        string     `json:"note"`
	SubmittedAt time.Time  `json:"submitted_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

func FromGovApplication(a domain.GovApplication) GovApplicationResponse {
	return GovApplicationResponse{
		ID: a.ID, ServiceName: a.ServiceName, ReferenceNo: a.ReferenceNo,
		Status: a.Status, Note: a.Note, SubmittedAt: a.SubmittedAt, UpdatedAt: a.UpdatedAt,
	}
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

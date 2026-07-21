// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"encoding/json"
	"time"

	"template/internal/business/domain"
)

// raw нь jsonb []byte-г JSON дугтуй болгож дамжуулна (хоосныг null болгоно).
func raw(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

// ── Platforms / routes ───────────────────────────────────────────────────────

type RelayPlatformResponse struct {
	ID                string    `json:"id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	Direction         string    `json:"direction"`
	EndpointURL       string    `json:"endpoint_url"`
	SupervisorContact string    `json:"supervisor_contact"`
	WebhookSecret     string    `json:"webhook_secret"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
}

func FromRelayPlatform(p domain.RelayPlatform) RelayPlatformResponse {
	return RelayPlatformResponse{ID: p.ID, Code: p.Code, Name: p.Name, Direction: p.Direction, EndpointURL: p.EndpointURL,
		SupervisorContact: p.SupervisorContact, WebhookSecret: p.WebhookSecret, Enabled: p.Enabled, CreatedAt: p.CreatedAt}
}

func ToRelayPlatformList(list []domain.RelayPlatform) []RelayPlatformResponse {
	out := make([]RelayPlatformResponse, 0, len(list))
	for _, p := range list {
		out = append(out, FromRelayPlatform(p))
	}
	return out
}

type RelayRouteResponse struct {
	ID           string    `json:"id"`
	ServiceCode  string    `json:"service_code"`
	PlatformID   string    `json:"platform_id"`
	PlatformName string    `json:"platform_name"`
	SLAMinutes   int       `json:"sla_minutes"`
	CreatedAt    time.Time `json:"created_at"`
}

func FromRelayRoute(rt domain.RelayRoute) RelayRouteResponse {
	return RelayRouteResponse{ID: rt.ID, ServiceCode: rt.ServiceCode, PlatformID: rt.PlatformID,
		PlatformName: rt.PlatformName, SLAMinutes: rt.SLAMinutes, CreatedAt: rt.CreatedAt}
}

func ToRelayRouteList(list []domain.RelayRoute) []RelayRouteResponse {
	out := make([]RelayRouteResponse, 0, len(list))
	for _, rt := range list {
		out = append(out, FromRelayRoute(rt))
	}
	return out
}

// ── Requests / assignments / events ──────────────────────────────────────────

type RelayRequestResponse struct {
	ID             string          `json:"id"`
	SourcePlatform string          `json:"source_platform"`
	ExternalRef    string          `json:"external_ref"`
	ServiceCode    string          `json:"service_code"`
	Title          string          `json:"title"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	Priority       string          `json:"priority"`
	ReceivedAt     time.Time       `json:"received_at"`
	DueAt          time.Time       `json:"due_at"`
	Status         string          `json:"status"`
	Result         json.RawMessage `json:"result,omitempty"`
	FulfilledAt    *time.Time      `json:"fulfilled_at,omitempty"`
	BreachNotified bool            `json:"breach_notified"`
}

func FromRelayRequest(q domain.RelayRequest) RelayRequestResponse {
	return RelayRequestResponse{
		ID: q.ID, SourcePlatform: q.SourcePlatform, ExternalRef: q.ExternalRef, ServiceCode: q.ServiceCode,
		Title: q.Title, Payload: raw(q.Payload), Priority: q.Priority, ReceivedAt: q.ReceivedAt, DueAt: q.DueAt,
		Status: q.Status, Result: raw(q.Result), FulfilledAt: q.FulfilledAt, BreachNotified: q.BreachNotified,
	}
}

func ToRelayRequestList(list []domain.RelayRequest) []RelayRequestResponse {
	out := make([]RelayRequestResponse, 0, len(list))
	for _, q := range list {
		out = append(out, FromRelayRequest(q))
	}
	return out
}

type RelayAssignmentResponse struct {
	ID            string          `json:"id"`
	RequestID     string          `json:"request_id"`
	PlatformID    string          `json:"platform_id"`
	PlatformName  string          `json:"platform_name"`
	Status        string          `json:"status"`
	DueAt         time.Time       `json:"due_at"`
	DispatchedAt  *time.Time      `json:"dispatched_at,omitempty"`
	RespondedAt   *time.Time      `json:"responded_at,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
	RemindersSent int             `json:"reminders_sent"`
	Escalated     bool            `json:"escalated"`
}

func fromRelayAssignment(a domain.RelayAssignment) RelayAssignmentResponse {
	return RelayAssignmentResponse{
		ID: a.ID, RequestID: a.RequestID, PlatformID: a.PlatformID, PlatformName: a.PlatformName,
		Status: a.Status, DueAt: a.DueAt, DispatchedAt: a.DispatchedAt, RespondedAt: a.RespondedAt,
		Result: raw(a.Result), RemindersSent: a.RemindersSent, Escalated: a.Escalated,
	}
}

type RelayEventResponse struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"request_id"`
	AssignmentID *string   `json:"assignment_id,omitempty"`
	Type         string    `json:"type"`
	Detail       string    `json:"detail"`
	CreatedAt    time.Time `json:"created_at"`
}

func fromRelayEvent(e domain.RelayEvent) RelayEventResponse {
	return RelayEventResponse{ID: e.ID, RequestID: e.RequestID, AssignmentID: e.AssignmentID,
		Type: e.Type, Detail: e.Detail, CreatedAt: e.CreatedAt}
}

// ── Overview + detail ────────────────────────────────────────────────────────

type RelayStatusBucketResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type RelayPlatformStatResponse struct {
	PlatformID    string  `json:"platform_id"`
	PlatformName  string  `json:"platform_name"`
	Total         int     `json:"total"`
	Done          int     `json:"done"`
	Overdue       int     `json:"overdue"`
	Pending       int     `json:"pending"`
	CompliancePct float64 `json:"compliance_pct"`
}

type RelayOverviewResponse struct {
	ReceivedToday    int                         `json:"received_today"`
	InProgress       int                         `json:"in_progress"`
	Overdue          int                         `json:"overdue"`
	Fulfilled        int                         `json:"fulfilled"`
	Total            int                         `json:"total"`
	SLACompliancePct float64                     `json:"sla_compliance_pct"`
	AvgFulfillMins   int                         `json:"avg_fulfill_mins"`
	StatusBuckets    []RelayStatusBucketResponse `json:"status_buckets"`
	Platforms        []RelayPlatformStatResponse `json:"platforms"`
	RecentEvents     []RelayEventResponse        `json:"recent_events"`
}

func FromRelayOverview(o domain.RelayOverview) RelayOverviewResponse {
	buckets := make([]RelayStatusBucketResponse, 0, len(o.StatusBuckets))
	for _, b := range o.StatusBuckets {
		buckets = append(buckets, RelayStatusBucketResponse{Status: b.Status, Count: b.Count})
	}
	plats := make([]RelayPlatformStatResponse, 0, len(o.Platforms))
	for _, p := range o.Platforms {
		plats = append(plats, RelayPlatformStatResponse{PlatformID: p.PlatformID, PlatformName: p.PlatformName,
			Total: p.Total, Done: p.Done, Overdue: p.Overdue, Pending: p.Pending, CompliancePct: p.CompliancePct})
	}
	events := make([]RelayEventResponse, 0, len(o.RecentEvents))
	for _, e := range o.RecentEvents {
		events = append(events, fromRelayEvent(e))
	}
	return RelayOverviewResponse{
		ReceivedToday: o.ReceivedToday, InProgress: o.InProgress, Overdue: o.Overdue, Fulfilled: o.Fulfilled,
		Total: o.Total, SLACompliancePct: o.SLACompliancePct, AvgFulfillMins: o.AvgFulfillMins,
		StatusBuckets: buckets, Platforms: plats, RecentEvents: events,
	}
}

type RelayRequestDetailResponse struct {
	Request     RelayRequestResponse      `json:"request"`
	Assignments []RelayAssignmentResponse `json:"assignments"`
	Events      []RelayEventResponse      `json:"events"`
}

func FromRelayRequestDetail(d domain.RelayRequestDetail) RelayRequestDetailResponse {
	asg := make([]RelayAssignmentResponse, 0, len(d.Assignments))
	for _, a := range d.Assignments {
		asg = append(asg, fromRelayAssignment(a))
	}
	events := make([]RelayEventResponse, 0, len(d.Events))
	for _, e := range d.Events {
		events = append(events, fromRelayEvent(e))
	}
	return RelayRequestDetailResponse{Request: FromRelayRequest(d.Request), Assignments: asg, Events: events}
}

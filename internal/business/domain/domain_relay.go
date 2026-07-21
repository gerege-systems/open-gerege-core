// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// Platform-хоорондын үйлчилгээний хүсэлт дамжуулах + SLA хяналтын домэйн. Дээд
// platform-оос хугацаатай хүсэлт хүлээж авч, доод platform-ууд руу дамжуулж,
// заагдсан хугацаанд биелэлтийг хянаж/шахаж, хариуг цуглуулна. Эдгээр нь
// platform-хоорондын тохиргоо/telemetry (per-citizen биш) тул gateway-ийн адил
// RLS-гүй.

// RelayRequest-ийн статусууд.
const (
	RelayReqReceived   = "received"
	RelayReqDispatched = "dispatched"
	RelayReqInProgress = "in_progress"
	RelayReqFulfilled  = "fulfilled"
	RelayReqOverdue    = "overdue"
	RelayReqRejected   = "rejected"
)

// RelayAssignment-ийн статусууд.
const (
	RelayAsgPending      = "pending"
	RelayAsgAcknowledged = "acknowledged"
	RelayAsgInProgress   = "in_progress"
	RelayAsgDone         = "done"
	RelayAsgOverdue      = "overdue"
	RelayAsgRejected     = "rejected"
)

// relay_events-ийн төрлүүд (timeline + realtime feed).
const (
	RelayEvtReceived       = "received"
	RelayEvtDispatched     = "dispatched"
	RelayEvtReminded       = "reminded"
	RelayEvtEscalated      = "escalated"
	RelayEvtResponded      = "responded"
	RelayEvtFulfilled      = "fulfilled"
	RelayEvtOverdue        = "overdue"
	RelayEvtBreachNotified = "breach_notified"
	RelayEvtForwardedUp    = "forwarded_up" // дээд platform руу webhook-оор дамжуулав
)

// Platform-ын чиглэл: upstream (дээрээс хүсэлт ирж, бид түүнд хариу/дамжуулна)
// эсвэл downstream (бид доош хүсэлт дамжуулж, хариуг нь хүлээнэ).
const (
	RelayDirUpstream   = "upstream"
	RelayDirDownstream = "downstream"
)

// RelayReminderFractions нь SLA цонхны аль хувь дээр downstream-д сануулга
// (шахалт) илгээхийг заана.
var RelayReminderFractions = []float64{0.75, 0.90}

// RelayEscalateGrace нь assignment overdue болсноос хойш дээд шат руу (supervisor)
// автоматаар escalate хийхийн өмнөх нэмэлт хугацаа. Template default (production-д
// SLA-даа тааруулж уртасгаж болно).
const RelayEscalateGrace = 2 * time.Minute

// RelayPlatform нь дамжуулагч peer platform-ын бүртгэл — Direction-оор upstream
// (дээд) эсвэл downstream (доод) болохыг заана.
type RelayPlatform struct {
	ID                string
	Code              string
	Name              string
	Direction         string // upstream | downstream
	EndpointURL       string // webhook push хийх хаяг (demo-д дотоод loopback)
	SupervisorContact string // escalate хийх дээд шатны хаяг
	WebhookSecret     string // ирсэн/явах webhook-ийн HMAC гарын үсгийн нууц
	Enabled           bool
	CreatedAt         time.Time
}

// RelayRoute нь service_code → platform чиглүүлэлтийн дүрэм (target бүрийн SLA-тай).
type RelayRoute struct {
	ID           string
	ServiceCode  string
	PlatformID   string
	PlatformName string // join-оор дүүрнэ
	SLAMinutes   int
	CreatedAt    time.Time
}

// RelayRequest нь дээд platform-оос ирсэн хугацаатай (due_at) хүсэлт.
type RelayRequest struct {
	ID             string
	SourcePlatform string
	ExternalRef    string
	ServiceCode    string
	Title          string
	Payload        []byte // jsonb
	Priority       string
	ReceivedAt     time.Time
	DueAt          time.Time
	Status         string
	Result         []byte // jsonb (нэгтгэсэн хариу)
	FulfilledAt    *time.Time
	BreachNotified bool
	UpdatedAt      *time.Time
}

// RelayAssignment нь нэг downstream platform-д оногдсон дэд даалгавар.
type RelayAssignment struct {
	ID            string
	RequestID     string
	PlatformID    string
	PlatformName  string // join-оор дүүрнэ
	Status        string
	DueAt         time.Time
	DispatchedAt  *time.Time
	RespondedAt   *time.Time
	Result        []byte // jsonb
	RemindersSent int
	Escalated     bool
}

// RelayEvent нь хүсэлтийн timeline/feed-ийн нэг бичлэг.
type RelayEvent struct {
	ID           string
	RequestID    string
	AssignmentID *string
	Type         string
	Detail       string
	CreatedAt    time.Time
}

// RelayOverview нь realtime dashboard-ийн нэгтгэл.
type RelayOverview struct {
	ReceivedToday    int
	InProgress       int
	Overdue          int
	Fulfilled        int
	Total            int
	SLACompliancePct float64 // due_at дотор биелсэн хүсэлтийн хувь
	AvgFulfillMins   int
	StatusBuckets    []RelayStatusBucket
	Platforms        []RelayPlatformStat
	RecentEvents     []RelayEvent
}

type RelayStatusBucket struct {
	Status string
	Count  int
}

// RelayPlatformStat нь downstream platform тус бүрийн SLA гүйцэтгэл.
type RelayPlatformStat struct {
	PlatformID    string
	PlatformName  string
	Total         int
	Done          int
	Overdue       int
	Pending       int
	CompliancePct float64
}

// RelayRequestDetail нь нэг хүсэлт + assignment-ууд + event timeline.
type RelayRequestDetail struct {
	Request     RelayRequest
	Assignments []RelayAssignment
	Events      []RelayEvent
}

// RelayRemindersDue нь эхэлсэн болон due хугацааг өгвөл одоо (now) хэдэн сануулга
// илгээгдсэн байх ёстойг (RelayReminderFractions босгууд дээр тулгуурлан)
// буцаана. reminders_sent < энэ тоо бол шинэ сануулга шаардлагатай.
func RelayRemindersDue(start, dueAt, now time.Time) int {
	total := dueAt.Sub(start)
	if total <= 0 {
		return 0
	}
	frac := float64(now.Sub(start)) / float64(total)
	n := 0
	for _, f := range RelayReminderFractions {
		if frac >= f {
			n++
		}
	}
	return n
}

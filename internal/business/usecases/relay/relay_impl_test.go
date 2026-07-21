// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Relay usecase unit тест: Ingest routing/due_at, Respond fulfillment, SLASweep-
// ийн reminder/overdue/breach-once/escalate-once зан төлөв.
package relay

import (
	"context"
	"testing"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
)

// fakeRepo нь RelayRepository-ийн санах ойн хуурамч хэрэгжүүлэлт (sweep/ingest
// зан төлөвийг шалгахад хангалттай).
type fakeRepo struct {
	routes      []domain.RelayRoute
	dueSoon     []domain.RelayAssignment
	overdue     []domain.RelayAssignment
	events      []domain.RelayEvent
	reminders   int
	overdueMark int
	escalated   int
	breachCalls int
	breachFlip  bool // MarkBreachNotified буцаах утга
	createdReq  *domain.RelayRequest
	createdAsg  []domain.RelayAssignment
}

func (f *fakeRepo) RoutesForService(_ context.Context, code string) ([]domain.RelayRoute, error) {
	var out []domain.RelayRoute
	for _, r := range f.routes {
		if r.ServiceCode == code {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRepo) CreateRequestWithAssignments(_ context.Context, req *domain.RelayRequest, asg []domain.RelayAssignment) (domain.RelayRequest, []domain.RelayAssignment, error) {
	req.ID = "req-1"
	for i := range asg {
		asg[i].ID = "asg-" + string(rune('a'+i))
		asg[i].RequestID = req.ID
	}
	f.createdReq = req
	f.createdAsg = asg
	return *req, asg, nil
}
func (f *fakeRepo) MarkDispatched(context.Context, string) error { return nil }
func (f *fakeRepo) RespondAssignment(_ context.Context, _, status string, _ []byte) (domain.RelayRequest, bool, error) {
	return domain.RelayRequest{ID: "req-1"}, status == domain.RelayAsgDone, nil
}
func (f *fakeRepo) DueSoonAssignments(context.Context) ([]domain.RelayAssignment, error) {
	return f.dueSoon, nil
}
func (f *fakeRepo) OverdueAssignments(context.Context) ([]domain.RelayAssignment, error) {
	return f.overdue, nil
}
func (f *fakeRepo) MarkAssignmentOverdue(context.Context, string) error { f.overdueMark++; return nil }
func (f *fakeRepo) IncReminders(context.Context, string) error          { f.reminders++; return nil }
func (f *fakeRepo) MarkEscalated(context.Context, string) error         { f.escalated++; return nil }
func (f *fakeRepo) MarkRequestOverdue(context.Context, string) error    { return nil }
func (f *fakeRepo) MarkBreachNotified(context.Context, string) (bool, error) {
	f.breachCalls++
	return f.breachFlip, nil
}
func (f *fakeRepo) AppendEvent(_ context.Context, e *domain.RelayEvent) error {
	f.events = append(f.events, *e)
	return nil
}

// Дараах методуудыг тест ашиглахгүй тул минимал.
func (f *fakeRepo) ListPlatforms(context.Context) ([]domain.RelayPlatform, error) { return nil, nil }
func (f *fakeRepo) GetPlatformByCode(context.Context, string) (domain.RelayPlatform, error) {
	return domain.RelayPlatform{}, apperror.NotFound("platform not found")
}
func (f *fakeRepo) CreatePlatform(context.Context, *domain.RelayPlatform) (domain.RelayPlatform, error) {
	return domain.RelayPlatform{}, nil
}
func (f *fakeRepo) DeletePlatform(context.Context, string) error            { return nil }
func (f *fakeRepo) ListRoutes(context.Context) ([]domain.RelayRoute, error) { return f.routes, nil }
func (f *fakeRepo) CreateRoute(context.Context, *domain.RelayRoute) (domain.RelayRoute, error) {
	return domain.RelayRoute{}, nil
}
func (f *fakeRepo) DeleteRoute(context.Context, string) error { return nil }
func (f *fakeRepo) GetAssignment(context.Context, string) (domain.RelayAssignment, error) {
	return domain.RelayAssignment{}, nil
}
func (f *fakeRepo) Overview(context.Context) (domain.RelayOverview, error) {
	return domain.RelayOverview{}, nil
}
func (f *fakeRepo) ListRequests(context.Context, int) ([]domain.RelayRequest, error) { return nil, nil }
func (f *fakeRepo) GetRequestDetail(context.Context, string) (domain.RelayRequestDetail, error) {
	return domain.RelayRequestDetail{}, nil
}

func countEvents(events []domain.RelayEvent, typ string) int {
	n := 0
	for _, e := range events {
		if e.Type == typ {
			n++
		}
	}
	return n
}

func TestIngestRoutingAndDueCap(t *testing.T) {
	f := &fakeRepo{routes: []domain.RelayRoute{
		{ServiceCode: "svc", PlatformID: "p1", PlatformName: "P1", SLAMinutes: 30},
		{ServiceCode: "svc", PlatformID: "p2", PlatformName: "P2", SLAMinutes: 120},
	}}
	uc := NewUsecase(f)
	due := time.Now().Add(45 * time.Minute)
	_, err := uc.Ingest(context.Background(), IngestInput{ServiceCode: "svc", DueAt: &due})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(f.createdAsg) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(f.createdAsg))
	}
	// P2-ийн SLA (120м) нь хүсэлтийн due (45м)-ээр хязгаарлагдана.
	if f.createdAsg[1].DueAt.After(due) {
		t.Errorf("assignment due should be capped by request due")
	}
	if countEvents(f.events, domain.RelayEvtReceived) != 1 {
		t.Errorf("expected a received event")
	}
}

func TestIngestNoRoutes(t *testing.T) {
	uc := NewUsecase(&fakeRepo{})
	_, err := uc.Ingest(context.Background(), IngestInput{ServiceCode: "unknown"})
	if err == nil {
		t.Fatal("expected error when no routing configured")
	}
}

func TestRespondFulfilled(t *testing.T) {
	f := &fakeRepo{}
	uc := NewUsecase(f)
	if err := uc.Respond(context.Background(), "asg-a", RespondInput{Status: domain.RelayAsgDone}); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if countEvents(f.events, domain.RelayEvtResponded) != 1 || countEvents(f.events, domain.RelayEvtFulfilled) != 1 {
		t.Errorf("expected responded + fulfilled events, got %+v", f.events)
	}
}

func TestSweepReminders(t *testing.T) {
	now := time.Now()
	disp := now.Add(-80 * time.Second)
	f := &fakeRepo{dueSoon: []domain.RelayAssignment{
		// 100с цонхны 80% өнгөрсөн → 75% босго давсан (1 сануулга) хэрэгтэй.
		{ID: "a1", RequestID: "r1", PlatformName: "P", DispatchedAt: &disp, DueAt: now.Add(20 * time.Second), RemindersSent: 0},
	}}
	uc := NewUsecase(f)
	_ = uc.SLASweep(context.Background())
	if f.reminders != 1 {
		t.Errorf("expected 1 reminder, got %d", f.reminders)
	}
	if countEvents(f.events, domain.RelayEvtReminded) != 1 {
		t.Errorf("expected a reminded event")
	}
}

func TestSweepOverdueBreachEscalate(t *testing.T) {
	now := time.Now()
	f := &fakeRepo{
		breachFlip: true, // эхний удаа breach мэдэгдэнэ
		overdue: []domain.RelayAssignment{
			// Нэг хүсэлтийн 2 overdue assignment; grace давсан → escalate.
			{ID: "a1", RequestID: "r1", PlatformName: "P1", Status: domain.RelayAsgPending, DueAt: now.Add(-(domain.RelayEscalateGrace + time.Minute)), Escalated: false},
			{ID: "a2", RequestID: "r1", PlatformName: "P2", Status: domain.RelayAsgPending, DueAt: now.Add(-(domain.RelayEscalateGrace + time.Minute)), Escalated: false},
		},
	}
	uc := NewUsecase(f)
	_ = uc.SLASweep(context.Background())

	if f.overdueMark != 2 {
		t.Errorf("expected 2 assignments marked overdue, got %d", f.overdueMark)
	}
	if f.escalated != 2 {
		t.Errorf("expected 2 escalations, got %d", f.escalated)
	}
	// Breach нь хүсэлт тус бүрд НЭГ удаа (2 assignment, нэг request).
	if f.breachCalls != 1 {
		t.Errorf("expected breach notified once per request, got %d", f.breachCalls)
	}
	if countEvents(f.events, domain.RelayEvtBreachNotified) != 1 {
		t.Errorf("expected 1 breach_notified event")
	}
}

func TestRelayRemindersDue(t *testing.T) {
	start := time.Unix(0, 0)
	due := start.Add(100 * time.Second)
	if n := domain.RelayRemindersDue(start, due, start.Add(50*time.Second)); n != 0 {
		t.Errorf("50%% → 0 reminders, got %d", n)
	}
	if n := domain.RelayRemindersDue(start, due, start.Add(80*time.Second)); n != 1 {
		t.Errorf("80%% → 1 reminder, got %d", n)
	}
	if n := domain.RelayRemindersDue(start, due, start.Add(95*time.Second)); n != 2 {
		t.Errorf("95%% → 2 reminders, got %d", n)
	}
}

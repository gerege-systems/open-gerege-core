// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package relay

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/logger"
)

// simulateHoldDur нь demo simulator доод platform-ыг "ажиллаж байгаа мэт"
// харагдуулахаар хариу үүсгэхээсээ өмнө хүлээх хугацаа (dashboard flow-г
// байгалийн харагдуулна).
const simulateHoldDur = 20 * time.Second

type usecase struct {
	repo       repointerface.RelayRepository
	httpClient *http.Client
}

func NewUsecase(repo repointerface.RelayRepository) Usecase {
	return &usecase{repo: repo, httpClient: &http.Client{Timeout: 8 * time.Second}}
}

// event нь timeline/feed бичлэг нэмнэ (best-effort; хүсэлтийг блоклохгүй).
func (u *usecase) event(ctx context.Context, requestID string, assignmentID *string, typ, detail string) {
	if err := u.repo.AppendEvent(ctx, &domain.RelayEvent{RequestID: requestID, AssignmentID: assignmentID, Type: typ, Detail: detail}); err != nil {
		logger.ErrorWithContext(ctx, "relay: append event failed (non-fatal)", logger.Fields{"error": err.Error()})
	}
}

// ── Ingest / Dispatch / Respond ──────────────────────────────────────────────

func (u *usecase) Ingest(ctx context.Context, in IngestInput) (domain.RelayRequest, error) {
	code := strings.TrimSpace(in.ServiceCode)
	if code == "" {
		return domain.RelayRequest{}, apperror.BadRequest("service_code шаардлагатай")
	}
	routes, err := u.repo.RoutesForService(ctx, code)
	if err != nil {
		return domain.RelayRequest{}, apperror.InternalCause(fmt.Errorf("relay routes: %w", err))
	}
	if len(routes) == 0 {
		return domain.RelayRequest{}, apperror.BadRequest("энэ service_code-д чиглүүлэлт (routing) тохируулаагүй байна")
	}

	now := time.Now()
	var reqDue time.Time
	if in.DueAt != nil {
		reqDue = *in.DueAt
	}
	assignments := make([]domain.RelayAssignment, 0, len(routes))
	latest := now
	for _, rt := range routes {
		ad := now.Add(time.Duration(rt.SLAMinutes) * time.Minute)
		// Assignment-ийн SLA нь хүсэлтийн эцсийн хугацаанаас (reqDue) хэтэрч болохгүй.
		if !reqDue.IsZero() && ad.After(reqDue) {
			ad = reqDue
		}
		if ad.After(latest) {
			latest = ad
		}
		assignments = append(assignments, domain.RelayAssignment{PlatformID: rt.PlatformID, PlatformName: rt.PlatformName, DueAt: ad})
	}
	if reqDue.IsZero() {
		reqDue = latest
	}

	priority := strings.TrimSpace(in.Priority)
	if priority == "" {
		priority = "normal"
	}
	req := &domain.RelayRequest{
		SourcePlatform: in.SourcePlatform,
		ExternalRef:    in.ExternalRef,
		ServiceCode:    code,
		Title:          in.Title,
		Payload:        in.Payload,
		Priority:       priority,
		DueAt:          reqDue,
		Status:         domain.RelayReqDispatched, // Ingest дараа шууд дамжуулна
	}
	stored, storedAsg, err := u.repo.CreateRequestWithAssignments(ctx, req, assignments)
	if err != nil {
		return domain.RelayRequest{}, err
	}
	u.event(ctx, stored.ID, nil, domain.RelayEvtReceived,
		fmt.Sprintf("Хүсэлт хүлээн авлаа: %s — %d байгууллагад дамжуулна", code, len(storedAsg)))
	u.dispatch(ctx, stored, storedAsg)
	return stored, nil
}

// dispatch нь assignment бүрийг доод platform руу дамжуулна. Бодит endpoint-той
// platform руу HMAC гарын үсэгтэй webhook POST хийнэ; demo-д loopback (гадаад
// дуудлагагүй, simulator хариулна).
func (u *usecase) dispatch(ctx context.Context, req domain.RelayRequest, asg []domain.RelayAssignment) {
	// Downstream platform-уудыг id-гаар нь нэг удаа уншиж (endpoint/secret авахад).
	byID := map[string]domain.RelayPlatform{}
	if plats, err := u.repo.ListPlatforms(ctx); err == nil {
		for _, p := range plats {
			byID[p.ID] = p
		}
	}
	for i := range asg {
		if err := u.repo.MarkDispatched(ctx, asg[i].ID); err != nil {
			logger.ErrorWithContext(ctx, "relay: mark dispatched failed", logger.Fields{"error": err.Error(), "assignment": asg[i].ID})
			continue
		}
		u.event(ctx, asg[i].RequestID, &asg[i].ID, domain.RelayEvtDispatched, "Даалгавар дамжуулав: "+asg[i].PlatformName)
		if p, ok := byID[asg[i].PlatformID]; ok {
			u.deliverWebhook(ctx, p, domain.RelayWebhookEnvelope{
				Event: domain.RelayEvtDispatched, SourceCode: "self", ServiceCode: req.ServiceCode,
				ExternalRef: req.ExternalRef, Title: req.Title, Priority: req.Priority,
				Payload: req.Payload, DueAt: &asg[i].DueAt, SentAt: time.Now(),
			})
		}
	}
}

func (u *usecase) Respond(ctx context.Context, assignmentID string, in RespondInput) error {
	status := strings.TrimSpace(in.Status)
	if status != domain.RelayAsgDone && status != domain.RelayAsgRejected {
		return apperror.BadRequest("status нь done эсвэл rejected байх ёстой")
	}
	req, fulfilled, err := u.repo.RespondAssignment(ctx, assignmentID, status, in.Result)
	if err != nil {
		return err
	}
	u.event(ctx, req.ID, &assignmentID, domain.RelayEvtResponded, "Доод platform хариулав: "+status)
	if fulfilled {
		u.event(ctx, req.ID, nil, domain.RelayEvtFulfilled, "Бүх байгууллага хариулж, хүсэлт биелэгдлээ")
		// Эх нь бүртгэлтэй дээд platform бол нэгтгэсэн хариуг webhook-оор дээш илгээнэ.
		u.notifyUpstream(ctx, req, domain.RelayEvtFulfilled, "биелэгдлээ")
	}
	return nil
}

// ── SLA sweep (background worker-ийн нэг алхам) ───────────────────────────────

func (u *usecase) SLASweep(ctx context.Context) error {
	now := time.Now()

	// 1) Сануулга (шахалт) — SLA цонхны 75%/90% дээр downstream-д.
	if dueSoon, err := u.repo.DueSoonAssignments(ctx); err == nil {
		for _, a := range dueSoon {
			if a.DispatchedAt == nil {
				continue
			}
			need := domain.RelayRemindersDue(*a.DispatchedAt, a.DueAt, now)
			for a.RemindersSent < need {
				if err := u.repo.IncReminders(ctx, a.ID); err != nil {
					break
				}
				a.RemindersSent++
				u.event(ctx, a.RequestID, &a.ID, domain.RelayEvtReminded, "Сануулга илгээв: "+a.PlatformName+" — SLA хугацаа дөхөж байна")
			}
		}
	} else {
		logger.ErrorWithContext(ctx, "relay sweep: due-soon query failed", logger.Fields{"error": err.Error()})
	}

	// 2) Хугацаа хэтэрсэн — overdue тэмдэглэх, дээд шат руу escalate, дээд platform-д
	//    breach мэдэгдэх (хүсэлт тус бүрд нэг удаа).
	breachSeen := map[string]bool{}
	if overdue, err := u.repo.OverdueAssignments(ctx); err == nil {
		for _, a := range overdue {
			if a.Status != domain.RelayAsgOverdue {
				_ = u.repo.MarkAssignmentOverdue(ctx, a.ID)
				u.event(ctx, a.RequestID, &a.ID, domain.RelayEvtOverdue, "Хугацаа хэтэрлээ: "+a.PlatformName)
			}
			_ = u.repo.MarkRequestOverdue(ctx, a.RequestID)

			if !a.Escalated && now.After(a.DueAt.Add(domain.RelayEscalateGrace)) {
				if err := u.repo.MarkEscalated(ctx, a.ID); err == nil {
					u.event(ctx, a.RequestID, &a.ID, domain.RelayEvtEscalated, "Дээд шат (supervisor) руу escalate: "+a.PlatformName)
				}
			}

			if !breachSeen[a.RequestID] {
				breachSeen[a.RequestID] = true
				if flipped, err := u.repo.MarkBreachNotified(ctx, a.RequestID); err == nil && flipped {
					u.event(ctx, a.RequestID, nil, domain.RelayEvtBreachNotified, "Дээд platform-д SLA зөрчлийг мэдэгдэв")
					// Эх нь бүртгэлтэй дээд platform бол зөрчлийг webhook-оор дээш мэдэгдэнэ.
					if detail, derr := u.repo.GetRequestDetail(ctx, a.RequestID); derr == nil {
						u.notifyUpstream(ctx, detail.Request, domain.RelayEvtBreachNotified, "SLA зөрчил")
					}
				}
			}
		}
	} else {
		logger.ErrorWithContext(ctx, "relay sweep: overdue query failed", logger.Fields{"error": err.Error()})
	}
	return nil
}

// SimulateStep нь demo (scaffold) — dispatch хийгдсэнээс хойш simulateHoldDur
// хугацаа өнгөрсөн assignment-уудын ~60%-д доод platform-ын нэрийн өмнөөс "done"
// хариу үүсгэнэ. Үлдсэн нь хугацаа хэтэрч overdue/reminder/escalate урсгалыг
// dashboard дээр өөрөө харуулна.
func (u *usecase) SimulateStep(ctx context.Context) {
	now := time.Now()
	dueSoon, err := u.repo.DueSoonAssignments(ctx)
	if err != nil {
		return
	}
	for _, a := range dueSoon {
		if a.DispatchedAt == nil || now.Sub(*a.DispatchedAt) < simulateHoldDur {
			continue
		}
		if rand.Intn(100) < 60 { //nolint:gosec // demo simulator randomness, not security-sensitive
			_ = u.Respond(ctx, a.ID, RespondInput{Status: domain.RelayAsgDone, Result: []byte(`{"ok":true,"note":"demo fulfilled"}`)})
		}
	}
}

// simulateDemoWindow нь demo хүсэлтийн богино SLA цонх — dashboard дээр
// reminder/overdue/escalate урсгалыг хэдэн минутын дотор амьд харуулна.
const simulateDemoWindow = 90 * time.Second

func (u *usecase) SimulateIngest(ctx context.Context) {
	routes, err := u.repo.ListRoutes(ctx)
	if err != nil || len(routes) == 0 {
		return
	}
	seen := map[string]bool{}
	codes := make([]string, 0, len(routes))
	for _, rt := range routes {
		if !seen[rt.ServiceCode] {
			seen[rt.ServiceCode] = true
			codes = append(codes, rt.ServiceCode)
		}
	}
	code := codes[rand.Intn(len(codes))] //nolint:gosec // demo simulator, not security-sensitive
	due := time.Now().Add(simulateDemoWindow)
	_, _ = u.Ingest(ctx, IngestInput{
		SourcePlatform: "e-mongolia",
		ExternalRef:    fmt.Sprintf("DEMO-%05d", rand.Intn(100000)), //nolint:gosec // demo ref, not security-sensitive
		ServiceCode:    code,
		Title:          "Demo хүсэлт — " + code,
		Priority:       "normal",
		DueAt:          &due,
	})
}

// ── Dashboard + жагсаалт ─────────────────────────────────────────────────────

func (u *usecase) Overview(ctx context.Context) (domain.RelayOverview, error) {
	return u.repo.Overview(ctx)
}

func (u *usecase) ListRequests(ctx context.Context, limit int) ([]domain.RelayRequest, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return u.repo.ListRequests(ctx, limit)
}

func (u *usecase) GetRequest(ctx context.Context, id string) (domain.RelayRequestDetail, error) {
	return u.repo.GetRequestDetail(ctx, id)
}

// ── Platforms / routes (admin) ───────────────────────────────────────────────

func (u *usecase) ListPlatforms(ctx context.Context) ([]domain.RelayPlatform, error) {
	return u.repo.ListPlatforms(ctx)
}

func (u *usecase) CreatePlatform(ctx context.Context, in PlatformInput) (domain.RelayPlatform, error) {
	code := strings.TrimSpace(in.Code)
	name := strings.TrimSpace(in.Name)
	if code == "" || name == "" {
		return domain.RelayPlatform{}, apperror.BadRequest("code болон name шаардлагатай")
	}
	direction := strings.TrimSpace(in.Direction)
	if direction == "" {
		direction = domain.RelayDirDownstream
	}
	if direction != domain.RelayDirUpstream && direction != domain.RelayDirDownstream {
		return domain.RelayPlatform{}, apperror.BadRequest("direction нь upstream эсвэл downstream байх ёстой")
	}
	secret := strings.TrimSpace(in.WebhookSecret)
	if secret == "" {
		secret = domain.RelayNewWebhookSecret()
	}
	return u.repo.CreatePlatform(ctx, &domain.RelayPlatform{
		Code: code, Name: name, Direction: direction, EndpointURL: strings.TrimSpace(in.EndpointURL),
		SupervisorContact: strings.TrimSpace(in.SupervisorContact), WebhookSecret: secret, Enabled: in.Enabled,
	})
}

func (u *usecase) DeletePlatform(ctx context.Context, id string) error {
	return u.repo.DeletePlatform(ctx, id)
}

func (u *usecase) ListRoutes(ctx context.Context) ([]domain.RelayRoute, error) {
	return u.repo.ListRoutes(ctx)
}

func (u *usecase) CreateRoute(ctx context.Context, in RouteInput) (domain.RelayRoute, error) {
	code := strings.TrimSpace(in.ServiceCode)
	if code == "" || strings.TrimSpace(in.PlatformID) == "" {
		return domain.RelayRoute{}, apperror.BadRequest("service_code болон platform_id шаардлагатай")
	}
	sla := in.SLAMinutes
	if sla <= 0 {
		sla = 60
	}
	return u.repo.CreateRoute(ctx, &domain.RelayRoute{ServiceCode: code, PlatformID: in.PlatformID, SLAMinutes: sla})
}

func (u *usecase) DeleteRoute(ctx context.Context, id string) error {
	return u.repo.DeleteRoute(ctx, id)
}

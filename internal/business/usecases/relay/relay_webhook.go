// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/logger"
)

// deliverWebhook нь envelope-ийг peer platform-ын endpoint руу HMAC гарын үсэгтэй
// POST хийнэ. demo endpoint (хоосон/demo://) бол гадагш илгээхгүй (best-effort,
// хүсэлтийг блоклохгүй — алдааг зөвхөн логлоно).
func (u *usecase) deliverWebhook(ctx context.Context, p domain.RelayPlatform, env domain.RelayWebhookEnvelope) {
	if domain.RelayIsDemoEndpoint(p.EndpointURL) {
		return // demo: гадаад дуудлагагүй; simulator дотооддоо ажиллана
	}
	body, err := json.Marshal(env)
	if err != nil {
		logger.ErrorWithContext(ctx, "relay: marshal webhook failed", logger.Fields{"error": err.Error()})
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.EndpointURL, bytes.NewReader(body))
	if err != nil {
		logger.ErrorWithContext(ctx, "relay: build webhook request failed", logger.Fields{"error": err.Error(), "platform": p.Code})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(domain.RelayWebhookSourceHeader, env.SourceCode)
	req.Header.Set(domain.RelayWebhookEventHeader, env.Event)
	req.Header.Set(domain.RelayWebhookSigHeader, domain.RelaySignWebhook(p.WebhookSecret, body))

	resp, err := u.httpClient.Do(req)
	if err != nil {
		logger.ErrorWithContext(ctx, "relay: webhook delivery failed", logger.Fields{"error": err.Error(), "platform": p.Code})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		logger.ErrorWithContext(ctx, "relay: webhook non-2xx", logger.Fields{"status": resp.StatusCode, "platform": p.Code})
	}
}

// ReceiveWebhook нь бүртгэлтэй peer platform-оос ирсэн webhook-ийг code-оор нь олж,
// HMAC гарын үсгээр баталгаажуулаад, envelope-ийг шинэ хүсэлт болгон ingest хийнэ.
// Дээд (upstream) болон доод (downstream) аль ч чиглэлээс хүсэлт хүлээж авна.
func (u *usecase) ReceiveWebhook(ctx context.Context, sourceCode, signature string, body []byte) (domain.RelayRequest, error) {
	sourceCode = strings.TrimSpace(sourceCode)
	if sourceCode == "" {
		return domain.RelayRequest{}, apperror.BadRequest(domain.RelayWebhookSourceHeader + " header шаардлагатай")
	}
	p, err := u.repo.GetPlatformByCode(ctx, sourceCode)
	if err != nil {
		return domain.RelayRequest{}, apperror.Unauthorized("тодорхойгүй эх platform")
	}
	if !p.Enabled {
		return domain.RelayRequest{}, apperror.Forbidden("эх platform идэвхгүй байна")
	}
	if !domain.RelayVerifyWebhook(p.WebhookSecret, signature, body) {
		return domain.RelayRequest{}, apperror.Unauthorized("webhook гарын үсэг таарахгүй байна")
	}
	var env domain.RelayWebhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return domain.RelayRequest{}, apperror.BadRequest("webhook бие буруу байна")
	}
	return u.Ingest(ctx, IngestInput{
		SourcePlatform: p.Code,
		ExternalRef:    env.ExternalRef,
		ServiceCode:    env.ServiceCode,
		Title:          env.Title,
		Payload:        env.Payload,
		Priority:       env.Priority,
		DueAt:          env.DueAt,
	})
}

// ForwardUp нь хүсэлтийг сонгосон дээд (upstream) platform руу webhook-оор дамжуулна.
func (u *usecase) ForwardUp(ctx context.Context, requestID, platformCode string) error {
	platformCode = strings.TrimSpace(platformCode)
	if requestID == "" || platformCode == "" {
		return apperror.BadRequest("request_id болон platform_code шаардлагатай")
	}
	p, err := u.repo.GetPlatformByCode(ctx, platformCode)
	if err != nil {
		return err
	}
	if p.Direction != domain.RelayDirUpstream {
		return apperror.BadRequest("зөвхөн дээд (upstream) platform руу дамжуулна")
	}
	detail, err := u.repo.GetRequestDetail(ctx, requestID)
	if err != nil {
		return err
	}
	u.forwardUpTo(ctx, p, detail.Request, domain.RelayEvtForwardedUp,
		fmt.Sprintf("Хүсэлтийг дээд platform руу дамжуулав: %s", p.Name))
	return nil
}

// forwardUpTo нь request-ийг тухайн дээд platform руу webhook-оор илгээж, timeline-д
// event нэмнэ (best-effort).
func (u *usecase) forwardUpTo(ctx context.Context, p domain.RelayPlatform, req domain.RelayRequest, event, detail string) {
	u.deliverWebhook(ctx, p, domain.RelayWebhookEnvelope{
		Event:       event,
		SourceCode:  "self",
		ServiceCode: req.ServiceCode,
		ExternalRef: req.ExternalRef,
		Title:       req.Title,
		Priority:    req.Priority,
		Payload:     req.Payload,
		Result:      req.Result,
		DueAt:       &req.DueAt,
		SentAt:      time.Now(),
	})
	u.event(ctx, req.ID, nil, event, detail)
}

// notifyUpstream нь хүсэлтийн эх (source_platform) нь бүртгэлтэй дээд platform бол
// түүнд webhook илгээнэ (breach/fulfilled тайлагнах). Бүртгэлгүй бол чимээгүй өнгөрнө.
func (u *usecase) notifyUpstream(ctx context.Context, req domain.RelayRequest, event, detail string) {
	code := strings.TrimSpace(req.SourcePlatform)
	if code == "" {
		return
	}
	p, err := u.repo.GetPlatformByCode(ctx, code)
	if err != nil || p.Direction != domain.RelayDirUpstream || !p.Enabled {
		return // эх platform бүртгэлгүй/дээд биш — webhook илгээхгүй (демо loopback гэх мэт)
	}
	u.deliverWebhook(ctx, p, domain.RelayWebhookEnvelope{
		Event:       event,
		SourceCode:  "self",
		ServiceCode: req.ServiceCode,
		ExternalRef: req.ExternalRef,
		Title:       req.Title,
		Result:      req.Result,
		SentAt:      time.Now(),
	})
	_ = detail
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package verify нь GeregeCloud Verify API (verify.gecloud.mn)-ийн client юм.
// Энэ алсын үйлчилгээ OTP-ийг өөрөө үүсгэж, bcrypt-аар хадгалж, имэйл/SMS-ээр
// илгээж, brute-force-ыг өөрөө хязгаарладаг. Иймд template нь дотоодын code
// generator / SMTP mailer-ийн оронд зөвхөн request_id-ийн амьдрах хугацааг л
// (Redis-д) хянадаг.
//
//	POST /verify/send   {to, channel}            → {request_id}
//	POST /verify/check  {request_id, code}       → {status: "approved"}
//	Auth: X-API-Key: gck_live_...
package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotApproved нь /verify/check код буруу/хугацаа дууссан (status != approved)
// үед буцдаг sentinel. Дуудагч үүнийг "буруу OTP" (4xx) гэж тайлбарлана;
// бусад алдаа нь дотоод/сүлжээний асуудал (5xx) гэж үзнэ.
var ErrNotApproved = errors.New("verify: code not approved")

// Sender нь OTP илгээх/шалгах хийсвэрлэл — тестэд хуурамчаар тавихад хялбар.
type Sender interface {
	// Send нь OTP-ийг to (имэйл/утас) руу channel-аар илгээж request_id буцаана.
	// channel хоосон бол client-ийн өгөгдмөл (config) channel ашиглана.
	Send(ctx context.Context, to, channel string) (requestID string, err error)
	// Check нь request_id + code-г баталгаажуулна. Зөвшөөрөгдсөн бол nil,
	// буруу/хугацаа дууссан бол ErrNotApproved, бусад тохиолдолд алдаа.
	Check(ctx context.Context, requestID, code string) error
}

const (
	defaultBase    = "https://verify.gecloud.mn/v1"
	defaultChannel = "email"
	maxRespBytes   = 64 << 10
)

// Client нь GeregeCloud Verify API-руу залгадаг HTTP client.
type Client struct {
	base    string
	apiKey  string
	channel string
	http    *http.Client
}

// NewClient нь Verify client үүсгэнэ. base/channel хоосон бол өгөгдмөл утга авна.
// apiKey хоосон бол Send/Check нь "тохируулаагүй" алдаа буцаана (template нь
// gecloud-гүйгээр boot хийгдэх боломжтой хэвээр).
func NewClient(base, apiKey, channel string) *Client {
	if base == "" {
		base = defaultBase
	}
	if channel == "" {
		channel = defaultChannel
	}
	return &Client{
		base:    strings.TrimRight(base, "/"),
		apiKey:  apiKey,
		channel: channel,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Send(ctx context.Context, to, channel string) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("verify: API key not configured (VERIFY_API_KEY)")
	}
	if channel == "" {
		channel = c.channel
	}
	raw, status, err := c.post(ctx, "/verify/send", map[string]string{"to": to, "channel": channel})
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("verify send: status %d: %s", status, snippet(raw))
	}
	var out struct {
		RequestID string `json:"request_id"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil || out.RequestID == "" {
		return "", fmt.Errorf("verify send: empty/invalid request_id: %s", snippet(raw))
	}
	return out.RequestID, nil
}

func (c *Client) Check(ctx context.Context, requestID, code string) error {
	if c.apiKey == "" {
		return errors.New("verify: API key not configured (VERIFY_API_KEY)")
	}
	raw, status, err := c.post(ctx, "/verify/check", map[string]string{"request_id": requestID, "code": code})
	if err != nil {
		return err
	}
	// Сервер талын алдаа (5xx) нь дотоод асуудал — дахин оролдохыг зөвшөөрнө.
	if status >= 500 {
		return fmt.Errorf("verify check: status %d: %s", status, snippet(raw))
	}
	var out struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(raw, &out)
	if status < 300 && out.Status == "approved" {
		return nil
	}
	// 2xx-non-approved эсвэл 4xx (буруу/хугацаа дууссан код) → ErrNotApproved.
	return ErrNotApproved
}

func (c *Client) post(ctx context.Context, path string, body any) ([]byte, int, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, fmt.Errorf("verify: build request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("verify: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	return raw, resp.StatusCode, nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package eid нь гадаад eID identity provider (IdP)-ийн Relying Party (RP)
// client юм. Энэ template нь RP-ийн үүрэг гүйцэтгэдэг: QR/deep-link нэвтрэлтийг
// эхлүүлж (initiate), дараа нь session-ийг long-poll-оор хүлээж, амжилттай
// (COMPLETE) болоход IdP-ийн баталгаажуулсан иргэний identity-г хүлээн авдаг.
//
//	POST {base}/auth/qr/initiate  {display_text, callback_url, nonce} → 201 {session_id, ...}
//	GET  {base}/auth/session/{id}?timeout_ms=25000                    → 200 {state, identity?, ...}
//	Auth/header: X-RP-Client: MN/COM/.../template-web
//
// IdP нь TLS-ээр хамгаалагдсан, эрх бүхий (authoritative) эх сурвалж тул
// COMPLETE хариунд итгэнэ. Гарын үсгийг (signature) сертификатын эсрэг шалгах
// нь ирээдүйн сонголттой сайжруулалт — одоогоор хатуу татгалздаггүй.
package eid

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

// Terminal төлвүүдийн sentinel алдаанууд. IdP session-ийг хугацаа дуусгасан
// (EXPIRED) эсвэл хэрэглэгч татгалзсан (REFUSED) үед дуудагч эдгээрийг цэвэр
// хэрэглэгчийн мессеж рүү буулгаж болно.
var (
	// ErrSessionExpired нь session хугацаа дууссан (terminal) үед буцна.
	ErrSessionExpired = errors.New("eid: session expired")
	// ErrSessionRefused нь хэрэглэгч нэвтрэлтийг татгалзсан (terminal) үед буцна.
	ErrSessionRefused = errors.New("eid: session refused")
)

// Terminal төлвүүдийн нэрс. Эдгээрээс бусад аливаа state (жишээ нь RUNNING,
// PENDING) нь "хараахан дуусаагүй" гэж тооцогдоно.
const (
	StateComplete = "COMPLETE"
	StateExpired  = "EXPIRED"
	StateRefused  = "REFUSED"
)

const (
	defaultBase     = "https://api.eidgerege.mn/rp/v1"
	defaultRPClient = "MN/COM/6235972/template-web"
	maxRespBytes    = 256 << 10
)

// Identity нь IdP-ийн баталгаажуулсан иргэний таних мэдээлэл юм.
type Identity struct {
	NationalID  string `json:"national_id"`
	CivilID     string `json:"civil_id"`
	GivenName   string `json:"given_name"`
	Surname     string `json:"surname"`
	GivenNameEn string `json:"given_name_en"`
	SurnameEn   string `json:"surname_en"`
	FullName    string `json:"full_name"`
	KYCLevel    string `json:"kyc_level"`
}

// StartResult нь initiate хариуны клиентэд харагдах хэсэг юм. session_secret
// зэрэг нууцыг ил гаргадаггүй — IdP-ийн дотоод хэрэг.
type StartResult struct {
	SessionID        string
	VerificationCode string
	ExpiresAt        string
	DeviceLinkURL    string
}

// SessionResult нь session poll-ийн үр дүн. Identity нь зөвхөн COMPLETE үед
// дүүрэн байна.
type SessionResult struct {
	State    string
	Identity *Identity
}

// Client нь eID RP урсгалуудын хийсвэрлэл — тестэд хуурамчаар тавихад хялбар.
type Client interface {
	// QRInitiate нь QR/deep-link нэвтрэлтийг эхлүүлж session мэдээллийг буцаана.
	QRInitiate(ctx context.Context, displayText, callbackURL, nonce string) (*StartResult, error)
	// Session нь session-ийн төлвийг long-poll-оор асууна (timeoutMs хүртэл).
	// COMPLETE үед Identity-тэй; EXPIRED/REFUSED үед холбогдох sentinel алдаа
	// буцаах нь дуудагчид хялбар байж болох ч энд төлвийг буцааж, дуудагчид
	// шийдвэрлүүлэхээр үлдээв (Refresh/login урсгалын хэв маягтай нийцнэ).
	Session(ctx context.Context, sessionID string, timeoutMs int) (*SessionResult, error)
}

// client нь eID RP API руу залгадаг HTTP client (pkg/verify-ийн хэв маягтай).
type client struct {
	base     string
	rpClient string
	http     *http.Client
}

// NewClient нь eID RP client үүсгэнэ. base/rpClient хоосон бол өгөгдмөл утга
// авна. Poll нь 25с хүртэл long-poll хийдэг тул HTTP timeout-ийг 30с болгов.
func NewClient(base, rpClient string) Client {
	if base == "" {
		base = defaultBase
	}
	if rpClient == "" {
		rpClient = defaultRPClient
	}
	return &client{
		base:     strings.TrimRight(base, "/"),
		rpClient: rpClient,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *client) QRInitiate(ctx context.Context, displayText, callbackURL, nonce string) (*StartResult, error) {
	body := map[string]string{
		"display_text": displayText,
		"callback_url": callbackURL,
		"nonce":        nonce,
	}
	raw, status, err := c.post(ctx, "/auth/qr/initiate", body)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid initiate: status %d: %s", status, snippet(raw))
	}
	// session_secret / auth_code_hashable зэрэг нууцыг зориуд тэмдэглэхгүй,
	// log-д гаргахгүй — зөвхөн клиентэд хэрэгтэй талбаруудыг задлан авна.
	var out struct {
		SessionID        string `json:"session_id"`
		VerificationCode string `json:"verification_code"`
		ExpiresAt        string `json:"expires_at"`
		DeviceLinkURL    string `json:"device_link_url"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil || out.SessionID == "" {
		return nil, fmt.Errorf("eid initiate: empty/invalid session_id: %s", snippet(raw))
	}
	return &StartResult{
		SessionID:        out.SessionID,
		VerificationCode: out.VerificationCode,
		ExpiresAt:        out.ExpiresAt,
		DeviceLinkURL:    out.DeviceLinkURL,
	}, nil
}

func (c *client) Session(ctx context.Context, sessionID string, timeoutMs int) (*SessionResult, error) {
	if sessionID == "" {
		return nil, errors.New("eid: empty session_id")
	}
	path := fmt.Sprintf("/auth/session/%s?timeout_ms=%d", sessionID, timeoutMs)
	raw, status, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid session: status %d: %s", status, snippet(raw))
	}
	var out struct {
		State    string    `json:"state"`
		Identity *Identity `json:"identity"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil || out.State == "" {
		return nil, fmt.Errorf("eid session: invalid response: %s", snippet(raw))
	}
	return &SessionResult{State: out.State, Identity: out.Identity}, nil
}

func (c *client) post(ctx context.Context, path string, body any) ([]byte, int, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build request: %w", err)
	}
	c.setHeaders(req)
	return c.do(req)
}

func (c *client) get(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build request: %w", err)
	}
	c.setHeaders(req)
	return c.do(req)
}

// setHeaders нь бүх хүсэлтэд RP client таних header болон JSON content-type-г
// тавина.
func (c *client) setHeaders(req *http.Request) {
	req.Header.Set("X-RP-Client", c.rpClient)
	req.Header.Set("Content-Type", "application/json")
}

func (c *client) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: http: %w", err)
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

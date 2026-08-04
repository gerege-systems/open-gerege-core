// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ssoeidauth нь eID НЭВТРЭЛТИЙГ төвийн SSO-гоор дамжуулан гүйцэтгэх
// client. eid.AuthClient-ийг хэрэгжүүлдэг тул eidmongolia.mn RP креденшлтэй
// шууд client-ийн ОРОНД шууд тавигдана — дуудагч usecase-үүд ялгааг мэдэхгүй.
//
// ЯАГААД: платформ бүрд eID RP креденшл олгох нь бодит биш (креденшл бүр
// eidmongolia талд тусад нь бүртгэгддэг). Төвийн SSO нь креденшлээ эзэмшээд
// бүртгэлтэй апп (RP)-даа /v1/eid-auth/* прокси-оор нээж өгнө.
//
// ТАНИЛТ: иргэн энэ үед хараахан танигдаагүй тул иргэний токен байхгүй —
// апп ӨӨРИЙН client_credentials токеноор дуудна. Токеныг санах ойд хадгалж,
// хугацаа дуусахаас өмнө шинэчилнэ (нэг л удаа зэрэг авахаар түгжинэ).
package ssoeidauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gerege-systems/open-gerege-core/pkg/eid"
)

const (
	maxRespBytes = 1 << 20 // 1 MiB
	// tokenSkew — токеныг дуусахаас энэ хугацааны өмнө шинэчилнэ.
	tokenSkew = 30 * time.Second
	// pollTimeout — long-poll-ийн HTTP timeout нь SSO талын хүлээлтээс (25с)
	// урт байх ёстой.
	pollTimeout = 40 * time.Second
)

// ErrNotGranted нь SSO дээр энэ апп-д svc:eid-auth эрх олгогдоогүй (403).
var ErrNotGranted = errors.New("ssoeidauth: application is not granted svc:eid-auth")

// ErrProxyDisabled нь SSO дээр "eid-auth" gateway service унтраалттай (503).
var ErrProxyDisabled = errors.New("ssoeidauth: eID auth proxy disabled at SSO (503)")

// Config — proxy client-ийн тохиргоо.
type Config struct {
	// BaseURL — прокси зам, жишээ https://sso.gerege.mn/rp/eid-auth.
	BaseURL string
	// TokenURL — SSO-ийн OAuth2 token endpoint (client_credentials).
	TokenURL string
	// ClientID/ClientSecret — энэ платформын SSO дахь бүртгэл.
	ClientID     string
	ClientSecret string
	// Scope — хоосон бол "svc:eid-auth".
	Scope string
}

// Configured нь заавал шаардлагатай талбарууд бөглөгдсөн эсэхийг хэлнэ.
func (c Config) Configured() bool {
	return strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.TokenURL) != "" &&
		strings.TrimSpace(c.ClientID) != "" &&
		strings.TrimSpace(c.ClientSecret) != ""
}

// Client нь eid.AuthClient-ийг SSO прокси дээр хэрэгжүүлнэ.
type Client struct {
	cfg  Config
	http *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

// New нь client үүсгэнэ. Config.Configured() худал бол дуудагч энэ client-ийг
// хэрэглэхгүй байх ёстой (шууд eID зам руу буцна).
func New(cfg Config) *Client {
	if strings.TrimSpace(cfg.Scope) == "" {
		cfg.Scope = "svc:eid-auth"
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return &Client{cfg: cfg, http: &http.Client{Timeout: pollTimeout}}
}

// Compile-time шалгалт: eid.AuthClient-ийн гэрээг бүрэн хангана.
var _ eid.AuthClient = (*Client)(nil)

// QRInitiate — QR/device-link нэвтрэлт эхлүүлнэ. nonce нь SSO талд үүсдэг тул
// энд ашиглагдахгүй (интерфейсийн нийцлийн төлөө үлдээв).
func (c *Client) QRInitiate(ctx context.Context, _, callbackURL, _ string) (*eid.StartResult, error) {
	var out wireStart
	if err := c.post(ctx, "/start", map[string]string{"callbackUrl": callbackURL}, &out); err != nil {
		return nil, err
	}
	return out.toDomain(), nil
}

// Initiate — иргэний РД-аар push нэвтрэлт эхлүүлнэ. displayText нь SSO-ийн
// тохиргооноос ирдэг тул энд дамжуулахгүй.
func (c *Client) Initiate(ctx context.Context, nationalID, _, callbackURL string) (*eid.StartResult, error) {
	var out wireStart
	body := map[string]string{"national_id": nationalID}
	if callbackURL != "" {
		body["callbackUrl"] = callbackURL
	}
	if err := c.post(ctx, "/start-id", body, &out); err != nil {
		return nil, err
	}
	return out.toDomain(), nil
}

// Session — session-ий төлвийг асууна. timeoutMs нь SSO талд тогтмол тул
// дамжуулахгүй.
func (c *Client) Session(ctx context.Context, sessionID string, _ int) (*eid.SessionResult, error) {
	var out wirePoll
	if err := c.post(ctx, "/poll", map[string]string{"session_id": sessionID}, &out); err != nil {
		return nil, err
	}
	return out.toDomain(), nil
}

// ── wire бүтцүүд (SSO-ий snake_case DTO-той тааруулсан) ──

type wireStart struct {
	SessionID        string `json:"session_id"`
	DeviceLinkURL    string `json:"device_link_url"`
	VerificationCode string `json:"verification_code"`
	ExpiresAt        string `json:"expires_at"`
}

func (w wireStart) toDomain() *eid.StartResult {
	return &eid.StartResult{
		SessionID:        w.SessionID,
		DeviceLinkURL:    w.DeviceLinkURL,
		VerificationCode: w.VerificationCode,
		ExpiresAt:        w.ExpiresAt,
	}
}

type wireIdentity struct {
	CivilID        string `json:"civil_id"`
	NationalID     string `json:"national_id"`
	GivenName      string `json:"given_name"`
	Surname        string `json:"surname"`
	GivenNameEn    string `json:"given_name_en"`
	SurnameEn      string `json:"surname_en"`
	FullName       string `json:"full_name"`
	KYCLevel       string `json:"kyc_level"`
	DocumentNumber string `json:"document_number"`
}

type wirePoll struct {
	State    string        `json:"state"`
	Identity *wireIdentity `json:"identity"`
}

func (w wirePoll) toDomain() *eid.SessionResult {
	res := &eid.SessionResult{State: w.State}
	if w.Identity != nil {
		res.Identity = &eid.Identity{
			CivilID:        w.Identity.CivilID,
			NationalID:     w.Identity.NationalID,
			GivenName:      w.Identity.GivenName,
			Surname:        w.Identity.Surname,
			GivenNameEn:    w.Identity.GivenNameEn,
			SurnameEn:      w.Identity.SurnameEn,
			FullName:       w.Identity.FullName,
			KYCLevel:       w.Identity.KYCLevel,
			DocumentNumber: w.Identity.DocumentNumber,
		}
	}
	return res
}

// envelope нь v1.BaseResponse-ийн хэрэгтэй хэсэг.
type envelope struct {
	Data json.RawMessage `json:"data"`
}

// post нь аппын токеноор JSON POST хийж, {data} доторх payload-ыг задална.
// Токен 401-ээр няцаагдвал НЭГ удаа шинэчилж дахин оролдоно (SSO дээр түлхүүр
// эргэсэн/токен эрт хүчингүй болсон тохиолдол).
func (c *Client) post(ctx context.Context, path string, body map[string]string, out any) error {
	err := c.postOnce(ctx, path, body, out, false)
	if errors.Is(err, errTokenRejected) {
		return c.postOnce(ctx, path, body, out, true)
	}
	return err
}

// errTokenRejected нь дотоод дохио — гадагш гарахгүй.
var errTokenRejected = errors.New("ssoeidauth: token rejected")

func (c *Client) postOnce(ctx context.Context, path string, body map[string]string, out any, forceNewToken bool) error {
	token, err := c.accessToken(ctx, forceNewToken)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ssoeidauth request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if err != nil {
		return fmt.Errorf("ssoeidauth read: %w", err)
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		c.invalidate()
		return errTokenRejected
	case res.StatusCode == http.StatusForbidden:
		return ErrNotGranted
	case res.StatusCode == http.StatusServiceUnavailable:
		return ErrProxyDisabled
	case res.StatusCode >= 400 && res.StatusCode < 500:
		// РД буруу зэрэг оролтын алдаа — дуудагч usecase-ууд үүнийг
		// хэрэглэгчид зориулсан мессеж рүү буулгадаг.
		return fmt.Errorf("%w: status %d", eid.ErrInitiateRejected, res.StatusCode)
	case res.StatusCode < 200 || res.StatusCode >= 300:
		return fmt.Errorf("ssoeidauth: status %d", res.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("ssoeidauth decode envelope: %w", err)
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("ssoeidauth decode data: %w", err)
	}
	return nil
}

func (c *Client) invalidate() {
	c.mu.Lock()
	c.token, c.expires = "", time.Time{}
	c.mu.Unlock()
}

// accessToken нь хүчинтэй аппын токеныг буцаана (хэрэгтэй бол шинээр авна).
func (c *Client) accessToken(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && time.Now().Before(c.expires.Add(-tokenSkew)) {
		return c.token, nil
	}

	form := url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {c.cfg.Scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	// Нууц үгийг body-д биш, HTTP Basic-аар илгээнэ (RFC 6749 §2.3.1-ийн
	// зөвлөсөн хэлбэр; прокси логт задрахгүй).
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ssoeidauth token request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if err != nil {
		return "", fmt.Errorf("ssoeidauth token read: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("ssoeidauth token: status %d", res.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", fmt.Errorf("ssoeidauth token decode: %w", err)
	}
	if strings.TrimSpace(tok.AccessToken) == "" {
		return "", errors.New("ssoeidauth: token хоосон ирлээ")
	}
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	c.token = tok.AccessToken
	c.expires = time.Now().Add(ttl)
	return c.token, nil
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ssoeidproxy нь dgov SSO (sso.dgov.mn)-ий eID proxy service-ийн client.
// SSO нь /rp/eid/* (дотор /api/v1/eid/*) дор бүртгэгдсэн апп (RP)-уудад иргэний
// PKI самбарыг ДАМЖУУЛАН үзүүлдэг: апп нь хэрэглэгчийнхээ SSO access token-оор
// дуудахад SSO өөрийн eidmongolia RP creds-ээр өгөгдлийг татаж өгнө. Тиймээс
// энэ апп-д eID RP credential эзэмших шаардлагагүй.
//
//	GET {base}/summary       — dashboard-ын нэгдсэн тоо
//	GET {base}/certificates  — гэрчилгээ + статусын тоо
//	GET {base}/devices        — холбоотой төхөөрөмжүүд
//	GET {base}/activity        — RP-scoped auth/sign түүх
//
// base жишээ: https://sso.dgov.mn/rp/eid. Хариу нь {data: <snake_case DTO>}
// дугтуйтай — wire бүтцээр задалж, pkg/eid-ийн домэйн төрлүүд рүү буулгана.
package ssoeidproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"template/pkg/eid"
)

const maxRespBytes = 1 << 20 // 1 MiB

// ErrTokenExpired нь proxy 401 буцаах (access token хүчингүй/дууссан) үед буцна —
// дуудагч refresh хийсний дараа ч 401 бол хэрэглэгчийг дахин нэвтрүүлнэ.
var ErrTokenExpired = errors.New("ssoeidproxy: access token rejected (401)")

// ErrProxyDisabled нь SSO дээр "eid-proxy" gateway service унтраалттай (503) үед
// буцна.
var ErrProxyDisabled = errors.New("ssoeidproxy: eID proxy disabled at SSO (503)")

// Client нь SSO eID proxy-г дуудна.
type Client struct {
	base string
	http *http.Client
}

// New нь base URL (жишээ https://sso.dgov.mn/rp/eid)-ээр client үүсгэнэ.
func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// envelope нь v1.BaseResponse-ийн task-д хэрэгтэй хэсэг (internal-аас хамааралгүй
// байхын тулд энд тодорхойлов).
type envelope struct {
	Data json.RawMessage `json:"data"`
}

// get нь Bearer token-оор GET хийж, {data} доторх payload-ыг out руу задална.
// 401→ErrTokenExpired, 403→eid.ErrPKINotPermitted, 503→ErrProxyDisabled,
// 404→ out тэг утгатай үлдэнэ (алдаа биш).
func (c *Client) get(ctx context.Context, accessToken, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ssoeidproxy request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if err != nil {
		return fmt.Errorf("ssoeidproxy read: %w", err)
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return ErrTokenExpired
	case res.StatusCode == http.StatusForbidden:
		return eid.ErrPKINotPermitted
	case res.StatusCode == http.StatusServiceUnavailable:
		return ErrProxyDisabled
	case res.StatusCode == http.StatusNotFound:
		return nil // өгөгдөл олдсонгүй — out тэг утгатай
	case res.StatusCode < 200 || res.StatusCode >= 300:
		return fmt.Errorf("ssoeidproxy: status %d", res.StatusCode)
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("ssoeidproxy decode envelope: %w", err)
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("ssoeidproxy decode data: %w", err)
	}
	return nil
}

// ── wire бүтцүүд (SSO-ий snake_case DTO-той тааруулсан) ──

type wireCertCounts struct {
	Valid     int `json:"valid"`
	Revoked   int `json:"revoked"`
	Expired   int `json:"expired"`
	Suspended int `json:"suspended"`
	Total     int `json:"total"`
}

func (w wireCertCounts) toDomain() eid.CertCounts {
	return eid.CertCounts{Valid: w.Valid, Revoked: w.Revoked, Expired: w.Expired, Suspended: w.Suspended, Total: w.Total}
}

type wireActivityCounts struct {
	Authentication int `json:"authentication"`
	Signature      int `json:"signature"`
}

func ts(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

type wireSummary struct {
	GivenName           string             `json:"given_name"`
	Surname             string             `json:"surname"`
	Certificates        wireCertCounts     `json:"certificates"`
	Activity            wireActivityCounts `json:"activity"`
	DevicesActive       int                `json:"devices_active"`
	DevicesTotal        int                `json:"devices_total"`
	RepresentationCount int                `json:"representation_count"`
}

// Summary нь GET {base}/summary.
func (c *Client) Summary(ctx context.Context, accessToken string) (*eid.PersonSummary, error) {
	var w wireSummary
	if err := c.get(ctx, accessToken, "/summary", &w); err != nil {
		return nil, err
	}
	return &eid.PersonSummary{
		GivenName:           w.GivenName,
		Surname:             w.Surname,
		Certificates:        w.Certificates.toDomain(),
		Activity:            eid.ActivityCounts{Authentication: w.Activity.Authentication, Signature: w.Activity.Signature},
		DevicesActive:       w.DevicesActive,
		DevicesTotal:        w.DevicesTotal,
		RepresentationCount: w.RepresentationCount,
	}, nil
}

type wireCert struct {
	DocumentNumber   string     `json:"document_number"`
	Type             string     `json:"type"`
	SerialNumber     string     `json:"serial_number"`
	CertificateLevel string     `json:"certificate_level"`
	Status           string     `json:"status"`
	NotBefore        *time.Time `json:"not_before"`
	NotAfter         *time.Time `json:"not_after"`
	IssuerDn         string     `json:"issuer_dn"`
}

type wireCertificates struct {
	Counts       wireCertCounts `json:"counts"`
	Certificates []wireCert     `json:"certificates"`
}

// Certificates нь GET {base}/certificates.
func (c *Client) Certificates(ctx context.Context, accessToken string) (*eid.PersonCertificates, error) {
	var w wireCertificates
	if err := c.get(ctx, accessToken, "/certificates", &w); err != nil {
		return nil, err
	}
	certs := make([]eid.PersonCertItem, 0, len(w.Certificates))
	for _, x := range w.Certificates {
		certs = append(certs, eid.PersonCertItem{
			DocumentNumber: x.DocumentNumber, Type: x.Type, SerialNumber: x.SerialNumber,
			CertificateLevel: x.CertificateLevel, Status: x.Status,
			NotBefore: ts(x.NotBefore), NotAfter: ts(x.NotAfter), IssuerDn: x.IssuerDn,
		})
	}
	return &eid.PersonCertificates{Counts: w.Counts.toDomain(), Certificates: certs}, nil
}

type wireDevice struct {
	DocumentNumber string         `json:"document_number"`
	Platform       string         `json:"platform"`
	EnrolledAt     *time.Time     `json:"enrolled_at"`
	Active         bool           `json:"active"`
	DeactivatedAt  *time.Time     `json:"deactivated_at"`
	Extra          map[string]any `json:"extra"` // SSO proxy-ийн дамжуулсан нэмэлт талбарууд
}

type wireDevices struct {
	Devices     []wireDevice `json:"devices"`
	ActiveCount int          `json:"active_count"`
	Total       int          `json:"total"`
}

// Devices нь GET {base}/devices.
func (c *Client) Devices(ctx context.Context, accessToken string) (*eid.PersonDevices, error) {
	var w wireDevices
	if err := c.get(ctx, accessToken, "/devices", &w); err != nil {
		return nil, err
	}
	devices := make([]eid.PersonDeviceItem, 0, len(w.Devices))
	for _, x := range w.Devices {
		devices = append(devices, eid.PersonDeviceItem{
			DocumentNumber: x.DocumentNumber, Platform: x.Platform,
			EnrolledAt: ts(x.EnrolledAt), Active: x.Active, DeactivatedAt: x.DeactivatedAt,
			Extra: x.Extra,
		})
	}
	return &eid.PersonDevices{Devices: devices, ActiveCount: w.ActiveCount, Total: w.Total}, nil
}

type wireActivityItem struct {
	SessionID string         `json:"session_id"`
	Flow      string         `json:"flow"`
	Outcome   string         `json:"outcome"`
	DocText   string         `json:"doc_text"`
	Timestamp *time.Time     `json:"timestamp"`
	Extra     map[string]any `json:"extra"` // SSO proxy-ийн дамжуулсан нэмэлт талбарууд
}

type wireActivity struct {
	Counts   wireActivityCounts `json:"counts"`
	Sessions []wireActivityItem `json:"sessions"`
	Total    int                `json:"total"`
}

// Activity нь GET {base}/activity?limit&offset.
func (c *Client) Activity(ctx context.Context, accessToken string, limit, offset int) (*eid.PersonActivity, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	path := "/activity"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var w wireActivity
	if err := c.get(ctx, accessToken, path, &w); err != nil {
		return nil, err
	}
	sessions := make([]eid.PersonActivityItem, 0, len(w.Sessions))
	for _, x := range w.Sessions {
		sessions = append(sessions, eid.PersonActivityItem{
			SessionID: x.SessionID, Flow: x.Flow, Outcome: x.Outcome,
			DocText: x.DocText, Timestamp: ts(x.Timestamp), Extra: x.Extra,
		})
	}
	return &eid.PersonActivity{
		Counts:   eid.ActivityCounts{Authentication: w.Counts.Authentication, Signature: w.Counts.Signature},
		Sessions: sessions,
		Total:    w.Total,
	}, nil
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package xyp нь Gerege Verify (xyp.dgov.mn) байгууллагын лавлагаа API-гийн client.
// Улсын бүртгэлээс (ХУР) байгууллагын мэдээллийг реал-тайм авдаг — RP нь HTTP Basic
// Auth-аар (client_id:client_secret) хандана. Энэ үйлчилгээ зөвхөн эрх бүхий client-д л
// мэдээлэл өгдөг тул креденшлийг зөвхөн серверийн тал (BFF/backend) хадгална.
//
//	POST /v1/org/lookup  {reg_no}  → {found, organization:{...}}
//	Auth: Authorization: Basic base64(client_id:client_secret)
package xyp

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

// ErrNotConfigured — client_id/secret тохируулаагүй (template нь XYP-гүйгээр boot хийж болно).
var ErrNotConfigured = errors.New("xyp: client credentials not configured (XYP_CLIENT_ID/XYP_CLIENT_SECRET)")

// ErrNotFound — тухайн бүртгэлийн дугаартай байгууллага олдсонгүй (found=false).
var ErrNotFound = errors.New("xyp: organization not found")

const (
	defaultBase  = "https://xyp.dgov.mn"
	maxRespBytes = 128 << 10
)

// Founder — байгууллагын үүсгэн байгуулагч (иргэн эсвэл хуулийн этгээд).
type Founder struct {
	Name         string `json:"name"`
	RegNo        string `json:"reg_no"`
	Type         string `json:"type"` // Иргэн | Хуулийн этгээд
	SharePercent string `json:"share_percent"`
}

// StakeHolder — байгууллагын хувь эзэмшигч / ТУЗ-ийн гишүүн.
type StakeHolder struct {
	Name     string `json:"name"`
	RegNo    string `json:"reg_no"`
	Position string `json:"position"`
}

// Organization — /v1/org/lookup-ийн байгууллагын блок.
type Organization struct {
	RegNo        string        `json:"reg_no"`
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Capital      string        `json:"capital"`
	CEO          string        `json:"ceo"`
	CEORegNo     string        `json:"ceo_reg_no"`
	CEOPosition  string        `json:"ceo_position"`
	Phone        string        `json:"phone"`
	Address      string        `json:"address"`
	Industry     []string      `json:"industry"`
	Founders     []Founder     `json:"founders"`
	StakeHolders []StakeHolder `json:"stake_holders"`
}

// Lookuper — байгууллагын лавлагааны хийсвэрлэл (тестэд mock тавихад хялбар).
type Lookuper interface {
	// Lookup нь reg_no-гоор байгууллагын мэдээллийг буцаана. Олдоогүй бол ErrNotFound.
	Lookup(ctx context.Context, regNo string) (*Organization, error)
}

// Client нь xyp.dgov.mn руу залгадаг HTTP client.
type Client struct {
	base         string
	clientID     string
	clientSecret string
	http         *http.Client
}

// NewClient нь XYP client үүсгэнэ. base хоосон бол өгөгдмөл (https://xyp.dgov.mn).
// creds хоосон бол Lookup нь ErrNotConfigured буцаана (boot-ыг эвдэхгүй).
func NewClient(base, clientID, clientSecret string) *Client {
	if base == "" {
		base = defaultBase
	}
	return &Client{
		base:         strings.TrimRight(base, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Lookup — POST /v1/org/lookup {reg_no}.
func (c *Client) Lookup(ctx context.Context, regNo string) (*Organization, error) {
	if c.clientID == "" || c.clientSecret == "" {
		return nil, ErrNotConfigured
	}
	regNo = strings.TrimSpace(regNo)
	if regNo == "" {
		return nil, errors.New("xyp: reg_no хоосон байна")
	}
	raw, status, err := c.post(ctx, "/v1/org/lookup", map[string]string{"reg_no": regNo})
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if status >= 300 {
		return nil, fmt.Errorf("xyp lookup: status %d: %s", status, snippet(raw))
	}
	var out struct {
		Found        bool          `json:"found"`
		Organization *Organization `json:"organization"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil {
		return nil, fmt.Errorf("xyp lookup: invalid response: %s", snippet(raw))
	}
	if !out.Found || out.Organization == nil {
		return nil, ErrNotFound
	}
	return out.Organization, nil
}

func (c *Client) post(ctx context.Context, path string, body any) (respBody []byte, status int, err error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, fmt.Errorf("xyp: build request: %w", err)
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("xyp: http: %w", err)
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

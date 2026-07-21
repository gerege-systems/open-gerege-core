// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package google нь Google OAuth2 (OpenID Connect) client юм — SDK-гүй, REST.
// Consent URL үүсгэх, authorization code-ийг token руу солих, id_token-оос
// иргэний таних мэдээллийг (sub/email/name) авах үүрэгтэй. id_token нь Google-ийн
// token endpoint-оос TLS-ээр шууд ирдэг тул түүний payload-д итгэж болно (JWKS
// шалгалт хэрэггүй — man-in-the-middle-ийг TLS хаадаг).
package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	authEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenEndpoint = "https://oauth2.googleapis.com/token"
	maxRespBytes  = 64 << 10
)

// ErrNotConfigured нь client_id/secret тохируулаагүй үед буцна.
var ErrNotConfigured = errors.New("google: OAuth тохируулаагүй (GOOGLE_CLIENT_ID/SECRET)")

// User нь Google id_token-оос авсан иргэний таних мэдээлэл.
type User struct {
	Sub           string // Google-ийн давтагдашгүй account id
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// Client нь Google OAuth2 client.
type Client struct {
	clientID     string
	clientSecret string
	http         *http.Client
}

// NewClient нь Google OAuth client үүсгэнэ.
func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured нь client_id + secret хоёулаа тохируулагдсан эсэхийг буцаана.
func (c *Client) Configured() bool {
	return c.clientID != "" && c.clientSecret != ""
}

// AuthCodeURL нь Google consent дэлгэцийн URL-г буцаана. state нь CSRF-ээс
// хамгаалах (BFF-д cookie-той тулгагдана), redirectURI нь Google Cloud
// console-д бүртгэсэн callback байх ёстой.
func (c *Client) AuthCodeURL(state, redirectURI string) string {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")
	return authEndpoint + "?" + q.Encode()
}

// Exchange нь authorization code-ийг token руу солиж, id_token-оос User-г задална.
func (c *Client) Exchange(ctx context.Context, code, redirectURI string) (*User, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("google: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: token http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google: token exchange failed: status %d: %s", resp.StatusCode, snippet(raw))
	}

	var tok struct {
		IDToken string `json:"id_token"`
	}
	if jErr := json.Unmarshal(raw, &tok); jErr != nil || tok.IDToken == "" {
		return nil, fmt.Errorf("google: no id_token in response")
	}
	return parseIDToken(tok.IDToken)
}

// parseIDToken нь id_token (JWT)-ийн payload-оос User-г задална. Google token
// endpoint-оос TLS-ээр ирсэн тул гарын үсэг шалгахгүй.
func parseIDToken(idToken string) (*User, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("google: malformed id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("google: id_token payload decode: %w", err)
	}
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if jErr := json.Unmarshal(payload, &claims); jErr != nil || claims.Sub == "" {
		return nil, fmt.Errorf("google: invalid id_token claims")
	}
	return &User{
		Sub: claims.Sub, Email: claims.Email, EmailVerified: claims.EmailVerified,
		Name: claims.Name, Picture: claims.Picture,
	}, nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

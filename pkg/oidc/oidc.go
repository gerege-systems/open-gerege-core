// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package oidc нь dgov SSO (sso.dgov.mn, Ory Hydra) OIDC Authorization Code
// урсгалын минимал client. Endpoint-ууд issuer-ээс (discovery-тэй ижил) гарна:
//
//	{issuer}/oauth2/auth   — authorization endpoint (browser redirect)
//	{issuer}/oauth2/token  — token endpoint (code → access/id/refresh)
//	{issuer}/userinfo      — claims (sub, name, given_name, family_name, email)
//
// Client нь confidential (token_endpoint_auth_method=client_secret_basic).
// id_token-ийн RS256 гарын үсгийг JWKS-ээр шалгахын оронд claims-ыг /userinfo-
// оос (access token-оор, шууд TLS дуудлагаар) уншина — issuer-тэй шууд, итгэмжит.
package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxRespBytes = 1 << 20 // 1 MiB

// Client нь нэг registered OIDC client-ийн тохиргоог агуулна.
type Client struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURI  string
	scope        string
	http         *http.Client
}

// NewClient нь issuer (жишээ https://sso.dgov.mn) болон client creds-ээр OIDC
// client үүсгэнэ. scope хоосон бол "openid profile email" default.
func NewClient(issuer, clientID, clientSecret, redirectURI, scope string) *Client {
	if strings.TrimSpace(scope) == "" {
		scope = "openid profile email"
	}
	return &Client{
		issuer:       strings.TrimRight(issuer, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		scope:        scope,
		http:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured нь client-ийн бүрэн тохируулагдсан эсэхийг (SSO нэвтрэлт идэвхтэй
// эсэх) мэдээлнэ. Аль нэг талбар хоосон бол SSO урсгал inert.
func (c *Client) Configured() bool {
	return c.issuer != "" && c.clientID != "" && c.clientSecret != "" && c.redirectURI != ""
}

// AuthCodeURL нь browser-ийг чиглүүлэх /oauth2/auth URL-ийг state (+nonce)-тэй
// байгуулна.
func (c *Client) AuthCodeURL(state, nonce string) string {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("response_type", "code")
	q.Set("scope", c.scope)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("state", state)
	if nonce != "" {
		q.Set("nonce", nonce)
	}
	return c.issuer + "/oauth2/auth?" + q.Encode()
}

// tokenResponse нь /oauth2/token-ийн хариу (хэрэгтэй талбарууд).
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Tokens нь token endpoint-ийн бүрэн хариу — access/id/refresh + access token-ий
// хүчинтэй хугацаа (секунд). SSO eID proxy-г дуудахад refresh_token-ыг хадгалж,
// хугацаа дуусахад шинэчилнэ (offline_access scope шаардана).
type Tokens struct {
	AccessToken  string
	IDToken      string
	RefreshToken string
	ExpiresIn    int
}

// postToken нь confidential client-ийн (client_secret_basic) token endpoint
// дуудлагыг гүйцэтгэж, хариуг задлана. grant тус бүрийн form-ыг дуудагч бэлдэнэ.
func (c *Client) postToken(ctx context.Context, form url.Values) (tokenResponse, error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.issuer+"/oauth2/token", strings.NewReader(form.Encode()))
	if reqErr != nil {
		return tokenResponse{}, reqErr
	}
	req.SetBasicAuth(url.QueryEscape(c.clientID), url.QueryEscape(c.clientSecret))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, doErr := c.http.Do(req)
	if doErr != nil {
		return tokenResponse{}, fmt.Errorf("sso token request: %w", doErr)
	}
	defer func() { _ = res.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if readErr != nil {
		return tokenResponse{}, fmt.Errorf("sso token read: %w", readErr)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("sso token endpoint returned %d", res.StatusCode)
	}
	var tr tokenResponse
	if jErr := json.Unmarshal(body, &tr); jErr != nil {
		return tokenResponse{}, fmt.Errorf("sso token decode: %w", jErr)
	}
	if tr.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("sso token response missing access_token")
	}
	return tr, nil
}

// Exchange нь authorization code-ийг access token + id token болгож солино
// (client_secret_basic HTTP Basic auth). id_token нь RP-initiated logout-ийн
// id_token_hint-д хэрэглэгдэнэ (SSO дээр session дуусгах).
func (c *Client) Exchange(ctx context.Context, code string) (accessToken, idToken string, err error) {
	t, err := c.ExchangeFull(ctx, code)
	if err != nil {
		return "", "", err
	}
	return t.AccessToken, t.IDToken, nil
}

// ExchangeFull нь Exchange-ийн адил боловч refresh_token + expires_in-ийг мөн
// буцаана (SSO eID proxy-д зориулж токен хадгалахад хэрэгтэй).
func (c *Client) ExchangeFull(ctx context.Context, code string) (Tokens, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.redirectURI)
	tr, err := c.postToken(ctx, form)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: tr.AccessToken, IDToken: tr.IDToken, RefreshToken: tr.RefreshToken, ExpiresIn: tr.ExpiresIn}, nil
}

// Refresh нь refresh_token-оор шинэ access token (болон эргэлддэг шинэ
// refresh_token) авна. Hydra нь refresh token-ыг эргүүлдэг тул буцаасан шинэ
// RefreshToken-ыг заавал дахин хадгална.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	tr, err := c.postToken(ctx, form)
	if err != nil {
		return Tokens{}, err
	}
	// Hydra эргүүлэхгүй бол хуучин refresh_token-ыг хадгалж үлдэнэ.
	if tr.RefreshToken == "" {
		tr.RefreshToken = refreshToken
	}
	return Tokens{AccessToken: tr.AccessToken, IDToken: tr.IDToken, RefreshToken: tr.RefreshToken, ExpiresIn: tr.ExpiresIn}, nil
}

// ExchangePKCE нь PUBLIC client (PKCE, token_endpoint_auth_method=none)-ийн
// authorization code-ийг access + id token болгож солино. Exchange-ээс ялгаатай
// нь: HTTP Basic auth / client_secret БАЙХГҮЙ; client_id, code_verifier-ийг
// form-д (public client) илгээнэ. redirectURI нь native client-д бүртгэгдсэн
// (жишээ geregetemp://oauth2/callback) байх ёстой. c.issuer л хэрэглэгдэнэ —
// confidential client-ийн creds талбарууд ашиглагдахгүй.
func (c *Client) ExchangePKCE(ctx context.Context, clientID, code, codeVerifier, redirectURI string) (accessToken, idToken string, err error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", codeVerifier)

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.issuer+"/oauth2/token", strings.NewReader(form.Encode()))
	if reqErr != nil {
		return "", "", reqErr
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, doErr := c.http.Do(req)
	if doErr != nil {
		return "", "", fmt.Errorf("sso token request: %w", doErr)
	}
	defer func() { _ = res.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if readErr != nil {
		return "", "", fmt.Errorf("sso token read: %w", readErr)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", "", fmt.Errorf("sso token endpoint returned %d", res.StatusCode)
	}
	var tr tokenResponse
	if jErr := json.Unmarshal(body, &tr); jErr != nil {
		return "", "", fmt.Errorf("sso token decode: %w", jErr)
	}
	if tr.AccessToken == "" {
		return "", "", fmt.Errorf("sso token response missing access_token")
	}
	return tr.AccessToken, tr.IDToken, nil
}

// LogoutURL нь RP-initiated logout (end_session_endpoint) URL-ийг байгуулна —
// browser-ийг тийш чиглүүлэхэд SSO (Hydra) дээрх session дуусч, дараа нь
// postLogout руу буцна. idTokenHint (сонголт) нь баталгаажуулалт/skip-д тусална.
func (c *Client) LogoutURL(idTokenHint, postLogout string) string {
	q := url.Values{}
	if postLogout != "" {
		q.Set("post_logout_redirect_uri", postLogout)
	}
	if idTokenHint != "" {
		q.Set("id_token_hint", idTokenHint)
	}
	u := c.issuer + "/oauth2/sessions/logout"
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}

// LogoutURLFor нь idTokenHint-тэй logout URL-ийг client-ийн redirect_uri-аас
// гаргасан post-logout (scheme://host/) руугаа буцахаар байгуулна. Энэ нь SSO
// client-д бүртгэгдсэн post_logout_redirect_uri-тай таарах ёстой.
func (c *Client) LogoutURLFor(idTokenHint string) string {
	post := ""
	if u, err := url.Parse(c.redirectURI); err == nil && u.Scheme != "" && u.Host != "" {
		post = u.Scheme + "://" + u.Host + "/"
	}
	return c.LogoutURL(idTokenHint, post)
}

// UserInfo нь /userinfo-оос иргэний claims-ыг буцаана. sso.dgov.mn нь eID-ээр
// нэвтэрсэн иргэнд name/given_name/family_name-г (кирилл) буцаадаг; email/
// national_id нь тухайн scope/урсгалд байхгүй байж болзошгүй.
type UserInfo struct {
	Sub        string `json:"sub"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	// Латин (англи) нэр — SSO нь profile scope-д given_name_en/family_name_en
	// claim-аар буцаадаг (иргэний eID identity-ээс). RP-д латин нэрийг харуулна.
	GivenNameEn   string `json:"given_name_en"`
	FamilyNameEn  string `json:"family_name_en"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	// nationalid scope-ийн claims (SSO client-д тухайн scope байвал):
	// NationalID = регистрийн дугаар (регно), RegisterNumber = иргэний
	// бүртгэлийн дугаар (civil id). Эдгээрээр байгаа eID хэрэглэгчтэй тааруулна.
	NationalID     string `json:"national_id"`
	RegisterNumber string `json:"register_number"`
	// Google холболт — provider (dan) дээр иргэн Google-ээр нэвтэрсэн/холбосон бол
	// буцаана. Эдгээрээр энэ апп дээр "Google холбогдсон" төлөвийг тусгана.
	GoogleSub     string `json:"google_sub"`
	GoogleEmail   string `json:"google_email"`
	GoogleName    string `json:"google_name"`
	GooglePicture string `json:"google_picture"`
}

// UserInfo нь access token-оор /userinfo дуудна.
func (c *Client) UserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.issuer+"/userinfo", http.NoBody)
	if err != nil {
		return UserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("sso userinfo request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if err != nil {
		return UserInfo{}, fmt.Errorf("sso userinfo read: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return UserInfo{}, fmt.Errorf("sso userinfo returned %d", res.StatusCode)
	}
	var ui UserInfo
	if err := json.Unmarshal(body, &ui); err != nil {
		return UserInfo{}, fmt.Errorf("sso userinfo decode: %w", err)
	}
	if ui.Sub == "" {
		return UserInfo{}, fmt.Errorf("sso userinfo missing sub")
	}
	return ui, nil
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package hydra нь dan.dgov.mn-ийг OIDC provider болгож ажиллуулах Ory Hydra-ийн
// admin REST API-ийн ашигладаг хэсгийг stdlib-only байдлаар багцалдаг. Login/
// consent/logout challenge-уудыг accept/reject хийх, OAuth2 client-уудыг CRUD
// хийх (developer.dgov.mn апп удирдлага), consent session-уудыг жагсаах/цуцлах,
// token introspection зэргийг хамарна. sso-dgov-mn-ий internal/hydra-аас
// шилжүүлэв.
package hydra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Admin нь Hydra admin API (default http://hydra:4445)-ийн клиент. Admin plane
// нь ХЭЗЭЭ Ч public-д гарах ёсгүй — зөвхөн compose сүлжээ дотор.
type Admin struct {
	baseURL string
	http    *http.Client
}

func NewAdmin(baseURL string) *Admin {
	return &Admin{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// --- Login ---

type OAuth2Client struct {
	ClientID               string         `json:"client_id"`
	ClientName             string         `json:"client_name,omitempty"`
	Audience               []string       `json:"audience,omitempty"`
	Scope                  string         `json:"scope,omitempty"`
	RedirectURIs           []string       `json:"redirect_uris,omitempty"`
	PostLogoutRedirectURIs []string       `json:"post_logout_redirect_uris,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type LoginRequest struct {
	Challenge      string       `json:"challenge"`
	Skip           bool         `json:"skip"`
	Subject        string       `json:"subject"`
	Client         OAuth2Client `json:"client"`
	RequestedScope []string     `json:"requested_scope"`
	RequestURL     string       `json:"request_url"`
	RequestedACR   []string     `json:"requested_acr,omitempty"`
}

type LoginAccept struct {
	Subject     string         `json:"subject"`
	Remember    bool           `json:"remember"`
	RememberFor int            `json:"remember_for"`
	ACR         string         `json:"acr,omitempty"`
	AMR         []string       `json:"amr,omitempty"`
	Context     map[string]any `json:"context,omitempty"`
}

type RedirectResp struct {
	RedirectTo string `json:"redirect_to"`
}

func (a *Admin) GetLoginRequest(ctx context.Context, challenge string) (*LoginRequest, error) {
	var out LoginRequest
	err := a.do(ctx, http.MethodGet,
		"/admin/oauth2/auth/requests/login?"+url.Values{"login_challenge": {challenge}}.Encode(),
		nil, &out)
	return &out, err
}

func (a *Admin) AcceptLogin(ctx context.Context, challenge string, body LoginAccept) (string, error) {
	var out RedirectResp
	err := a.do(ctx, http.MethodPut,
		"/admin/oauth2/auth/requests/login/accept?"+url.Values{"login_challenge": {challenge}}.Encode(),
		body, &out)
	return out.RedirectTo, err
}

func (a *Admin) RejectLogin(ctx context.Context, challenge string, errCode, errDescription string) (string, error) {
	body := map[string]string{
		"error":             errCode,
		"error_description": errDescription,
	}
	var out RedirectResp
	err := a.do(ctx, http.MethodPut,
		"/admin/oauth2/auth/requests/login/reject?"+url.Values{"login_challenge": {challenge}}.Encode(),
		body, &out)
	return out.RedirectTo, err
}

// --- Consent ---

type ConsentRequest struct {
	Challenge         string         `json:"challenge"`
	Skip              bool           `json:"skip"`
	Subject           string         `json:"subject"`
	Client            OAuth2Client   `json:"client"`
	RequestedScope    []string       `json:"requested_scope"`
	RequestedAudience []string       `json:"requested_access_token_audience,omitempty"`
	Context           map[string]any `json:"context,omitempty"`
}

type ConsentAccept struct {
	GrantScope               []string       `json:"grant_scope"`
	GrantAccessTokenAudience []string       `json:"grant_access_token_audience,omitempty"`
	Session                  ConsentSession `json:"session"`
	Remember                 bool           `json:"remember"`
	RememberFor              int            `json:"remember_for"`
}

type ConsentSession struct {
	AccessToken map[string]any `json:"access_token,omitempty"`
	IDToken     map[string]any `json:"id_token,omitempty"`
}

func (a *Admin) GetConsentRequest(ctx context.Context, challenge string) (*ConsentRequest, error) {
	var out ConsentRequest
	err := a.do(ctx, http.MethodGet,
		"/admin/oauth2/auth/requests/consent?"+url.Values{"consent_challenge": {challenge}}.Encode(),
		nil, &out)
	return &out, err
}

func (a *Admin) AcceptConsent(ctx context.Context, challenge string, body ConsentAccept) (string, error) {
	var out RedirectResp
	err := a.do(ctx, http.MethodPut,
		"/admin/oauth2/auth/requests/consent/accept?"+url.Values{"consent_challenge": {challenge}}.Encode(),
		body, &out)
	return out.RedirectTo, err
}

func (a *Admin) RejectConsent(ctx context.Context, challenge string, errCode, errDescription string) (string, error) {
	body := map[string]string{
		"error":             errCode,
		"error_description": errDescription,
	}
	var out RedirectResp
	err := a.do(ctx, http.MethodPut,
		"/admin/oauth2/auth/requests/consent/reject?"+url.Values{"consent_challenge": {challenge}}.Encode(),
		body, &out)
	return out.RedirectTo, err
}

// --- Logout ---

type LogoutRequest struct {
	Subject     string       `json:"subject"`
	SID         string       `json:"sid"`
	RPInitiated bool         `json:"rp_initiated"`
	RequestURL  string       `json:"request_url"`
	Client      OAuth2Client `json:"client"`
}

func (a *Admin) GetLogoutRequest(ctx context.Context, challenge string) (*LogoutRequest, error) {
	var out LogoutRequest
	err := a.do(ctx, http.MethodGet,
		"/admin/oauth2/auth/requests/logout?"+url.Values{"logout_challenge": {challenge}}.Encode(),
		nil, &out)
	return &out, err
}

func (a *Admin) AcceptLogout(ctx context.Context, challenge string) (string, error) {
	var out RedirectResp
	err := a.do(ctx, http.MethodPut,
		"/admin/oauth2/auth/requests/logout/accept?"+url.Values{"logout_challenge": {challenge}}.Encode(),
		nil, &out)
	return out.RedirectTo, err
}

// --- Token introspection ---

type IntrospectionResponse struct {
	Active    bool     `json:"active"`
	ClientID  string   `json:"client_id,omitempty"`
	Sub       string   `json:"sub,omitempty"`
	Scope     string   `json:"scope,omitempty"`
	TokenUse  string   `json:"token_use,omitempty"`
	Aud       []string `json:"aud,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
}

// Introspect нь POST /admin/oauth2/introspect (RFC 7662)-г дуудна. Public
// /oauth2/introspect нь client auth шаарддаг тул admin endpoint-ыг ашиглана —
// бид аль хэдийн admin plane дээр байгаа.
func (a *Admin) Introspect(ctx context.Context, token string) (*IntrospectionResponse, error) {
	// Token-ыг URL-encode хийнэ: escape хийгдээгүй `&`/`=` нь introspection
	// хүсэлтэд нэмэлт form параметр тарьж болзошгүй.
	form := url.Values{"token": {token}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/admin/oauth2/introspect", strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hydra introspect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hydra introspect: %s — %s", resp.Status, string(b))
	}
	var out IntrospectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Client CRUD (developer.dgov.mn апп удирдлагад ашиглана) ---

// ClientCreate нь POST /admin/clients-ийн бидний ашигладаг дэд олонлог. Hydra
// хамаагүй том объект хүлээж авдаг; бид зөвхөн ашиглах талбаруудыг тавина.
type ClientCreate struct {
	ClientID                string         `json:"client_id"`
	ClientName              string         `json:"client_name"`
	ClientSecret            string         `json:"client_secret"`
	GrantTypes              []string       `json:"grant_types"`
	ResponseTypes           []string       `json:"response_types"`
	Scope                   string         `json:"scope"`
	RedirectURIs            []string       `json:"redirect_uris"`
	PostLogoutRedirectURIs  []string       `json:"post_logout_redirect_uris,omitempty"`
	TokenEndpointAuthMethod string         `json:"token_endpoint_auth_method"`
	SubjectType             string         `json:"subject_type"`
	Metadata                map[string]any `json:"metadata,omitempty"`

	// OIDC coordinated logout (Back-Channel / Front-Channel Logout).
	BackchannelLogoutURI             string `json:"backchannel_logout_uri,omitempty"`
	BackchannelLogoutSessionRequired bool   `json:"backchannel_logout_session_required,omitempty"`
	FrontchannelLogoutURI            string `json:"frontchannel_logout_uri,omitempty"`

	// DPoPBoundAccessTokens нь token-уудыг per-request proof-of-possession
	// key-д sender-constrain хийнэ (RFC 9449 / RFC 9700).
	DPoPBoundAccessTokens bool `json:"dpop_bound_access_tokens,omitempty"`
}

// ClientUpdate нь PUT /admin/clients/{id}-ийг тусгана. Hydra-ийн PUT нь
// replace-style тул бүрэн record-ыг буцааж илгээнэ (орхигдсон талбар default руу).
type ClientUpdate = ClientCreate

// ClientListEntry нь GET /admin/clients-ийн буцаадаг shape-ийн бидний ашиглах
// талбарууд.
type ClientListEntry struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	PostLogoutRedirectURIs  []string `json:"post_logout_redirect_uris,omitempty"`
	SubjectType             string   `json:"subject_type,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	CreatedAt               string   `json:"created_at,omitempty"`
	UpdatedAt               string   `json:"updated_at,omitempty"`
}

// ListClients нь бүртгэлтэй бүх OAuth2 client-ыг буцаана (эхний хуудас, 200).
func (a *Admin) ListClients(ctx context.Context) ([]ClientListEntry, error) {
	var out []ClientListEntry
	err := a.do(ctx, http.MethodGet, "/admin/clients?page_size=200", nil, &out)
	return out, err
}

// GetClient нь нэг client-ыг авна. Hydra нь read дээр secret-ыг орхидог.
func (a *Admin) GetClient(ctx context.Context, clientID string) (*ClientListEntry, error) {
	var out ClientListEntry
	err := a.do(ctx, http.MethodGet, "/admin/clients/"+clientID, nil, &out)
	return &out, err
}

// CreateClient нь OAuth2 client бүртгэнэ. Үүсгэсэн client-ыг буцаана (Hydra нь
// secret-ыг зөвхөн үүсгэх үед л буцаадаг).
func (a *Admin) CreateClient(ctx context.Context, body ClientCreate) (*ClientCreate, error) {
	var out ClientCreate
	err := a.do(ctx, http.MethodPost, "/admin/clients", body, &out)
	return &out, err
}

// UpdateClient нь бүртгэлтэй client record-ыг орлуулна. Бүрэн desired state
// дамжуул — орхигдсон нь default руу буцна.
func (a *Admin) UpdateClient(ctx context.Context, clientID string, body ClientUpdate) (*ClientCreate, error) {
	var out ClientCreate
	err := a.do(ctx, http.MethodPut, "/admin/clients/"+clientID, body, &out)
	return &out, err
}

// DeleteClient нь client-ыг Hydra-аас устгана. Олгосон token-ууд дуустал
// хүчинтэй; refresh шууд амжилтгүй болно.
func (a *Admin) DeleteClient(ctx context.Context, clientID string) error {
	return a.do(ctx, http.MethodDelete, "/admin/clients/"+clientID, nil, nil)
}

// --- Consent sessions (subject тус бүрийн идэвхтэй grant-ууд) ---

type ConsentSessionEntry struct {
	ConsentRequest struct {
		Subject   string       `json:"subject"`
		Client    OAuth2Client `json:"client"`
		GrantedAt string       `json:"handled_at,omitempty"`
	} `json:"consent_request"`
	GrantScope  []string `json:"grant_scope"`
	RememberFor int      `json:"remember_for"`
}

// ListConsentSessions нь subject-д Hydra санаж буй consent grant-уудыг буцаана.
func (a *Admin) ListConsentSessions(ctx context.Context, subject string) ([]ConsentSessionEntry, error) {
	var out []ConsentSessionEntry
	err := a.do(ctx, http.MethodGet,
		"/admin/oauth2/auth/sessions/consent?"+url.Values{"subject": {subject}}.Encode(),
		nil, &out)
	return out, err
}

// RevokeConsentSession нь subject-ийн нэг client-ийн consent-ыг устгана. Хоосон
// client дамжуулбал subject-ийн БҮХ consent-ыг устгана.
func (a *Admin) RevokeConsentSession(ctx context.Context, subject, client string) error {
	q := url.Values{"subject": {subject}}
	if client != "" {
		q.Set("client", client)
	} else {
		q.Set("all", "true")
	}
	return a.do(ctx, http.MethodDelete,
		"/admin/oauth2/auth/sessions/consent?"+q.Encode(),
		nil, nil)
}

// RevokeLoginSession нь subject-ийн OP-side login session-ыг дуусгана — дараагийн
// auth хүсэлт бүрэн eID round-trip хийнэ.
func (a *Admin) RevokeLoginSession(ctx context.Context, subject string) error {
	return a.do(ctx, http.MethodDelete,
		"/admin/oauth2/auth/sessions/login?"+url.Values{"subject": {subject}}.Encode(),
		nil, nil)
}

// HasScope нь introspection хариу `want` scope-ыг олгосон эсэхийг мэдээлнэ.
func (r *IntrospectionResponse) HasScope(want string) bool {
	for _, s := range strings.Fields(r.Scope) {
		if s == want {
			return true
		}
	}
	return false
}

// --- transport ---

func (a *Admin) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("hydra admin %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hydra admin %s %s: %s — %s", method, path, resp.Status, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

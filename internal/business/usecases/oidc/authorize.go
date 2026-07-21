// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	usersuc "template/internal/business/usecases/users"
	"template/internal/datasources/rls"
)

// Хугацаанууд. Authorization code нь боломжийн хэрээр богино байх ёстой —
// browser-ийн хаягийн мөр, referrer, лог зэрэгт үлдэх боломжтой (RFC 9700 §2.1.1).
const (
	ChallengeTTL = 15 * time.Minute
	AuthCodeTTL  = 60 * time.Second
	ConsentTTL   = 30 * 24 * time.Hour
)

// AuthorizeRequest нь `/oauth2/auth`-ийн задлагдсан параметрүүд.
type AuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
	Prompt              string
}

// AuthorizeError нь RP руу буцаах ёстой протоколын алдаа (RFC 6749 §4.1.2.1).
//
// RedirectURI нь ЗӨВХӨН client-ийн бүртгэлтэй ЯГ тулгагдсаны ДАРАА дүүрнэ.
// Хоосон бол дуудагч ХЭЗЭЭ Ч redirect хийхгүй, алдааг шууд харуулна. Ингэснээр
// "баталгаажаагүй хаяг руу чиглүүлэх" алдаа нь бүтцийн хувьд боломжгүй болно:
// handler нь өөрт ирсэн түүхий req.RedirectURI-г огт хардаггүй.
type AuthorizeError struct {
	Code        string // invalid_request | unauthorized_client | invalid_scope | ...
	Description string
	RedirectURI string
	State       string
}

func (e *AuthorizeError) Error() string { return e.Code + ": " + e.Description }

// CanRedirect нь алдааг RP руу буцаах боломжтой эсэхийг заана.
func (e *AuthorizeError) CanRedirect() bool { return e.RedirectURI != "" }

// RedirectURL нь RP руу буцаах алдааны бүтэн URL.
func (e *AuthorizeError) RedirectURL() string {
	return redirectWith(e.RedirectURI, map[string]string{
		"error":             e.Code,
		"error_description": e.Description,
		"state":             e.State,
	})
}

// Service нь authorize/consent урсгалын протоколын логик.
type Service struct {
	clients clientStore
	flow    flowStore
	issuer  string
	// keys / users нь ЗӨВХӨН id_token гаргахад хэрэгтэй. authorize урсгал
	// тэдгээргүйгээр ажиллана (тест хийхэд хялбар).
	keys  *KeyManager
	users userLookup
}

// userLookup нь id_token-ий claims-д иргэний бүртгэлийг уншина.
type userLookup interface {
	GetByID(ctx context.Context, req usersuc.GetByIDRequest) (usersuc.GetByIDResponse, error)
}

type clientStore interface {
	Get(ctx context.Context, clientID string) (domain.OAuthClient, error)
}

type flowStore interface {
	CreateChallenge(ctx context.Context, c domain.OAuthChallenge) error
	Challenge(ctx context.Context, kind, challenge string) (domain.OAuthChallenge, error)
	DecideChallenge(ctx context.Context, challenge, subject string, granted []string) error
	Consent(ctx context.Context, subject, clientID string) ([]string, error)
	SaveConsent(ctx context.Context, subject, clientID string, scopes []string, ttl time.Duration) error
	RevokeConsent(ctx context.Context, subject, clientID string) error
	CreateCode(ctx context.Context, c domain.OAuthAuthCode) error
	ConsumeCode(ctx context.Context, codeHash []byte) (domain.OAuthAuthCode, bool, error)
	StoreTokens(ctx context.Context, at domain.OAuthAccessToken, rt *domain.OAuthRefreshToken) error
	ConsumeRefreshToken(ctx context.Context, tokenHash []byte) (domain.OAuthRefreshToken, bool, error)
	RevokeFamily(ctx context.Context, familyID string) error
	RevokeForSubjectClient(ctx context.Context, subject, clientID string) error
	AccessToken(ctx context.Context, tokenHash []byte) (domain.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, tokenHash []byte, clientID string) (bool, error)
	RevokeRefreshToken(ctx context.Context, tokenHash []byte, clientID string) (bool, error)
}

func NewService(clients clientStore, flow flowStore, issuer string) *Service {
	return &Service{clients: clients, flow: flow, issuer: strings.TrimRight(issuer, "/")}
}

// WithTokenIssuing нь token гаргах чадварыг (id_token гарын үсэг + иргэний
// бүртгэл) залгана. Тусад нь байгаа шалтгаан: authorize/consent урсгал эдгээрээс
// хамаардаггүй тул тэднийг түлхүүргүйгээр тестлэх боломжтой.
func (s *Service) WithTokenIssuing(keys *KeyManager, users userLookup) *Service {
	s.keys = keys
	s.users = users
	return s
}

// flowCtx нь протоколын төлөвийн (challenge / code / token / consent) query-д
// RLS-ийн "service" үүргийг тавина.
//
// ЯАГААД ЭНД, route дээр БИШ: эдгээр хүснэгтэд хандах нь дуудагчаас үл хамаарна.
// `/oauth2/*` нь нэвтрээгүй дуудагдана, харин `/api/v1/provider/*/accept` нь
// authMiddleware-ийн ард ажилладаг бөгөөд тэр нь identity-г "user" болгож дардаг
// тул route-д суулгасан ServiceRLSContext чимээгүй хүчингүй болно. Хамаарлыг
// хэрэгцээтэй газарт нь тавьснаар route-ын дараалал өөрчлөгдөхөд эвдрэхгүй.
//
// Энэ нь эрхийг ӨРГӨТГӨХГҮЙ: протоколын шалгалтууд (challenge-ийн subject,
// client-ийн эзэмшил, PKCE) нь Go давхаргад хийгддэг; RLS нь энд өөр иргэний
// мөрийг хамгаалах хэрэгсэл биш, харин эдгээр хүснэгт рүү зөвхөн энэ урсгал
// хандаж байгааг батлах давхарга юм.
func flowCtx(ctx context.Context) context.Context { return rls.WithService(ctx) }

// Authorize нь `/oauth2/auth`-ийн хүсэлтийг шалгаж, login challenge үүсгээд
// нэвтрэх хуудас руу чиглүүлэх challenge-ыг буцаана.
//
// Шалгалтын ДАРААЛАЛ санаатай: client болон redirect_uri-г ЭХЭЛЖ шалгана, учир
// нь тэдгээр нь зөв болтол алдааг RP руу буцаах аргагүй (буцаах хаяг нь өөрөө
// баталгаажаагүй). Зөвхөн тэдний дараа л бусад алдааг RP руу чиглүүлж болно.
func (s *Service) Authorize(ctx context.Context, req AuthorizeRequest) (challenge string, client domain.OAuthClient, err error) {
	if strings.TrimSpace(req.ClientID) == "" {
		return "", client, &AuthorizeError{Code: "invalid_request", Description: "client_id is required"}
	}

	client, err = s.clients.Get(ctx, req.ClientID)
	if err != nil {
		if apperror.IsNotFound(err) {
			return "", client, &AuthorizeError{Code: "invalid_client", Description: "unknown client"}
		}
		return "", client, err
	}
	if !client.Enabled {
		return "", client, &AuthorizeError{Code: "unauthorized_client", Description: "client is disabled"}
	}

	// redirect_uri нь ЯГ бүртгэгдсэн байх ёстой. Хоосон бол ч татгалзана —
	// "цорын ганц бүртгэгдсэнийг нь ав" гэсэн тайвшрал нь алдаанд хүргэдэг.
	if !client.MatchRedirectURI(req.RedirectURI) {
		return "", client, &AuthorizeError{Code: "invalid_request", Description: "redirect_uri is not registered for this client"}
	}

	// ЭНДЭЭС ХОЙШ алдааг RP руу буцаана.
	if req.ResponseType != "code" {
		return "", client, &AuthorizeError{Code: "unsupported_response_type", Description: "only response_type=code is supported", RedirectURI: req.RedirectURI, State: req.State}
	}
	if !client.HasGrant(domain.GrantAuthorizationCode) {
		return "", client, &AuthorizeError{Code: "unauthorized_client", Description: "client may not use the authorization code grant", RedirectURI: req.RedirectURI, State: req.State}
	}

	// PKCE. Public client-д ЗААВАЛ; confidential client-д өгсөн бол шалгана.
	// `plain` арга нь хамгаалалт өгдөггүй тул огт зөвшөөрөхгүй (RFC 9700 §2.1.1).
	if req.CodeChallenge != "" {
		if req.CodeChallengeMethod != "S256" {
			return "", client, &AuthorizeError{Code: "invalid_request", Description: "code_challenge_method must be S256", RedirectURI: req.RedirectURI, State: req.State}
		}
	} else if client.IsPublic() {
		return "", client, &AuthorizeError{Code: "invalid_request", Description: "code_challenge is required for public clients", RedirectURI: req.RedirectURI, State: req.State}
	}

	requested := splitScope(req.Scope)
	granted := client.FilterAllowedScopes(requested)
	if len(granted) == 0 {
		return "", client, &AuthorizeError{Code: "invalid_scope", Description: "none of the requested scopes are allowed for this client", RedirectURI: req.RedirectURI, State: req.State}
	}

	challenge = randomToken()
	if err := s.flow.CreateChallenge(flowCtx(ctx), domain.OAuthChallenge{
		Challenge:           challenge,
		Kind:                domain.ChallengeLogin,
		ClientID:            client.ClientID,
		RequestedScopes:     granted,
		RedirectURI:         req.RedirectURI,
		State:               req.State,
		Nonce:               req.Nonce,
		ResponseType:        req.ResponseType,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		Prompt:              req.Prompt,
		ExpiresAt:           time.Now().Add(ChallengeTTL),
	}); err != nil {
		return "", client, err
	}
	return challenge, client, nil
}

// LoginChallenge нь хүчинтэй login challenge-ыг буцаана.
func (s *Service) LoginChallenge(ctx context.Context, challenge string) (domain.OAuthChallenge, error) {
	return s.flow.Challenge(flowCtx(ctx), domain.ChallengeLogin, challenge)
}

// ConsentChallenge нь хүчинтэй consent challenge-ыг буцаана.
func (s *Service) ConsentChallenge(ctx context.Context, challenge string) (domain.OAuthChallenge, error) {
	return s.flow.Challenge(flowCtx(ctx), domain.ChallengeConsent, challenge)
}

// AcceptLogin нь иргэнийг тухайн login challenge-д баталгаажуулж, consent
// challenge үүсгэнэ. Аль хэдийн санагдсан зөвшөөрөл байвал Skip=true болно.
func (s *Service) AcceptLogin(ctx context.Context, challenge, subject string) (consentChallenge string, skip bool, err error) {
	login, err := s.flow.Challenge(flowCtx(ctx), domain.ChallengeLogin, challenge)
	if err != nil {
		return "", false, err
	}
	if err := s.flow.DecideChallenge(flowCtx(ctx), challenge, subject, login.RequestedScopes); err != nil {
		return "", false, err
	}

	// Өмнө нь олгосон зөвшөөрөл хүссэн scope-ыг БҮРЭН хамарч байвал л алгасна.
	remembered, err := s.flow.Consent(flowCtx(ctx), subject, login.ClientID)
	if err != nil {
		return "", false, err
	}
	skip = coversAll(remembered, login.RequestedScopes)

	consentChallenge = randomToken()
	if err := s.flow.CreateChallenge(flowCtx(ctx), domain.OAuthChallenge{
		Challenge:           consentChallenge,
		Kind:                domain.ChallengeConsent,
		ClientID:            login.ClientID,
		Subject:             subject,
		RequestedScopes:     login.RequestedScopes,
		RedirectURI:         login.RedirectURI,
		State:               login.State,
		Nonce:               login.Nonce,
		ResponseType:        login.ResponseType,
		CodeChallenge:       login.CodeChallenge,
		CodeChallengeMethod: login.CodeChallengeMethod,
		Prompt:              login.Prompt,
		Skip:                skip,
		ExpiresAt:           time.Now().Add(ChallengeTTL),
	}); err != nil {
		return "", false, err
	}
	return consentChallenge, skip, nil
}

// AcceptConsent нь олгосон scope-оор authorization code гаргаж, RP руу буцах
// бүтэн URL-ыг буцаана.
//
// subject нь challenge дээрх subject-тэй ТААРАХ ёстой — өөр иргэний нээлттэй
// challenge-ыг өөрийн session-ээр дуусгах боломжийг хаана.
func (s *Service) AcceptConsent(ctx context.Context, challenge, subject string, grantScope []string) (string, error) {
	c, err := s.flow.Challenge(flowCtx(ctx), domain.ChallengeConsent, challenge)
	if err != nil {
		return "", err
	}
	if c.Subject != subject {
		return "", apperror.Forbidden("consent challenge belongs to a different user")
	}

	// Олгож болох scope нь хүссэнээс ХЭТРЭХГҮЙ (эрх өсгөх боломжгүй).
	granted := intersect(c.RequestedScopes, grantScope)
	if len(grantScope) == 0 {
		granted = c.RequestedScopes // UI юу ч заагаагүй → бүгдийг
	}
	if len(granted) == 0 {
		return "", apperror.BadRequest("no scope was granted")
	}

	if err := s.flow.DecideChallenge(flowCtx(ctx), challenge, subject, granted); err != nil {
		return "", err
	}
	if err := s.flow.SaveConsent(flowCtx(ctx), subject, c.ClientID, granted, ConsentTTL); err != nil {
		return "", err
	}

	code := randomToken()
	if err := s.flow.CreateCode(flowCtx(ctx), domain.OAuthAuthCode{
		CodeHash:            hashToken(code),
		ClientID:            c.ClientID,
		Subject:             subject,
		Scopes:              granted,
		RedirectURI:         c.RedirectURI,
		Nonce:               c.Nonce,
		CodeChallenge:       c.CodeChallenge,
		CodeChallengeMethod: c.CodeChallengeMethod,
		AuthTime:            time.Now(),
		ExpiresAt:           time.Now().Add(AuthCodeTTL),
	}); err != nil {
		return "", err
	}

	return redirectWith(c.RedirectURI, map[string]string{"code": code, "state": c.State}), nil
}

// Reject нь урсгалыг зогсоож, алдааг RP руу буцаах URL-ыг үүсгэнэ.
func (s *Service) Reject(ctx context.Context, kind, challenge, reason string) (string, error) {
	c, err := s.flow.Challenge(flowCtx(ctx), kind, challenge)
	if err != nil {
		return "", err
	}
	if err := s.flow.DecideChallenge(flowCtx(ctx), challenge, c.Subject, nil); err != nil {
		return "", err
	}
	if reason == "" {
		reason = "the request was denied"
	}
	return redirectWith(c.RedirectURI, map[string]string{
		"error":             "access_denied",
		"error_description": reason,
		"state":             c.State,
	}), nil
}

// ErrorRedirect нь RP руу буцаах алдааны URL-ыг үүсгэнэ.
func ErrorRedirect(redirectURI, state string, e *AuthorizeError) string {
	return redirectWith(redirectURI, map[string]string{
		"error":             e.Code,
		"error_description": e.Description,
		"state":             state,
	})
}

// ── туслахууд ────────────────────────────────────────────────────────────────

// redirectWith нь параметрүүдийг redirect_uri-ийн ОДОО БАЙГАА query дээр нэмнэ
// (RP-ийн өөрийн query-г устгахгүй).
func redirectWith(redirectURI string, params map[string]string) string {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return redirectURI
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func splitScope(s string) []string { return strings.Fields(s) }

// intersect нь a-д БАЙГАА b-ийн элементүүдийг a-гийн дарааллаар буцаана.
func intersect(a, b []string) []string {
	want := make(map[string]bool, len(b))
	for _, s := range b {
		want[s] = true
	}
	out := make([]string, 0, len(a))
	for _, s := range a {
		if want[s] {
			out = append(out, s)
		}
	}
	return out
}

// coversAll нь have нь want-ийн БҮХ элементийг агуулж байгаа эсэхийг шалгана.
func coversAll(have, want []string) bool {
	if len(want) == 0 {
		return false
	}
	set := make(map[string]bool, len(have))
	for _, s := range have {
		set[s] = true
	}
	for _, s := range want {
		if !set[s] {
			return false
		}
	}
	return true
}

// randomToken нь 32 байт криптографийн санамсаргүй утгыг base64url болгоно.
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("oidc: crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashToken нь нууц утгыг хадгалахын өмнө sha256-аар хэшилнэ. Утга нь өндөр
// энтропитой санамсаргүй тул давсны хэрэг байхгүй (энэ нь нууц үг биш).
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// VerifyPKCE нь code_verifier нь хадгалсан challenge-тай тохирч байгааг шалгана.
func VerifyPKCE(codeChallenge, method, verifier string) bool {
	if codeChallenge == "" {
		return verifier == "" // PKCE ашиглаагүй урсгал
	}
	if method != "S256" || verifier == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]) == codeChallenge
}

// ── RP-initiated logout ──────────────────────────────────────────────────────

// StartLogout нь `/oauth2/sessions/logout`-ийн хүсэлтээс logout challenge
// үүсгэнэ. post_logout_redirect_uri өгсөн бол тухайн client-д БҮРТГЭГДСЭН байх
// ёстой — эс бөгөөс logout-ийг дурын хаяг руу чиглүүлэх open redirect болно.
func (s *Service) StartLogout(ctx context.Context, clientID, idTokenHint, postLogoutRedirectURI, state string) (string, error) {
	// RP-үүд ихэвчлэн `client_id` биш `id_token_hint` илгээдэг (OIDC RP-Initiated
	// Logout §3 нь түүнийг ЗӨВЛӨДӨГ). Hint-ээс client-ыг гаргаж авна; гарын үсэг
	// нь баталгаажсан тул өөр апп-ийн нэрийн өмнөөс logout эхлүүлэх боломжгүй.
	subject := ""
	if clientID == "" && idTokenHint != "" {
		var err error
		clientID, subject, err = s.parseIDTokenHint(ctx, idTokenHint)
		if err != nil {
			return "", err
		}
	}

	redirect := ""
	if postLogoutRedirectURI != "" {
		if clientID == "" {
			return "", apperror.BadRequest("client_id or id_token_hint is required with post_logout_redirect_uri")
		}
		client, err := s.clients.Get(ctx, clientID)
		if err != nil {
			return "", err
		}
		if !client.MatchPostLogoutRedirectURI(postLogoutRedirectURI) {
			return "", apperror.BadRequest("post_logout_redirect_uri is not registered for this client")
		}
		redirect = postLogoutRedirectURI
	}

	challenge := randomToken()
	if err := s.flow.CreateChallenge(flowCtx(ctx), domain.OAuthChallenge{
		Challenge:             challenge,
		Kind:                  domain.ChallengeLogout,
		ClientID:              clientID,
		Subject:               subject,
		State:                 state,
		PostLogoutRedirectURI: redirect,
		ExpiresAt:             time.Now().Add(ChallengeTTL),
	}); err != nil {
		return "", err
	}
	return challenge, nil
}

// AcceptLogout нь logout challenge-ыг дуусгаж, буцах хаягийг өгнө. Бүртгэгдсэн
// post_logout_redirect_uri байхгүй бол issuer-ийн нүүр рүү буцаана.
func (s *Service) AcceptLogout(ctx context.Context, challenge string) (string, error) {
	c, err := s.flow.Challenge(flowCtx(ctx), domain.ChallengeLogout, challenge)
	if err != nil {
		return "", err
	}
	if err := s.flow.DecideChallenge(flowCtx(ctx), challenge, c.Subject, nil); err != nil {
		return "", err
	}
	if c.PostLogoutRedirectURI == "" {
		return s.issuer + "/", nil
	}
	return redirectWith(c.PostLogoutRedirectURI, map[string]string{"state": c.State}), nil
}

// LogoutChallenge нь хүчинтэй logout challenge-ыг буцаана.
func (s *Service) LogoutChallenge(ctx context.Context, challenge string) (domain.OAuthChallenge, error) {
	return s.flow.Challenge(flowCtx(ctx), domain.ChallengeLogout, challenge)
}

// S256Challenge нь code_verifier-ээс PKCE-ийн S256 challenge-ыг гаргана.
// RP-ийн тал хийдэг тооцоо; тест болон хэрэгслүүдэд хэрэгтэй.
func S256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

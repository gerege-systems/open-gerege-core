// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"template/internal/apperror"
	"template/internal/business/domain"
	usersuc "template/internal/business/usecases/users"
	"template/internal/datasources/rls"
	"template/pkg/secrethash"
)

// Token-ийн наслалт. Access token богино (RP-үүд refresh-ээр сунгана); refresh
// нь Hydra-гийн өгөгдмөлтэй ижил 30 хоног.
const (
	AccessTokenTTL  = time.Hour
	RefreshTokenTTL = 30 * 24 * time.Hour
	IDTokenTTL      = time.Hour
	// ScopeOfflineAccess байхгүй бол refresh token гаргахгүй (OIDC §11).
	ScopeOfflineAccess = "offline_access"
)

// TokenError нь RFC 6749 §5.2-ийн token endpoint-ийн алдаа.
type TokenError struct {
	Code        string // invalid_request | invalid_client | invalid_grant | ...
	Description string
	// Status нь HTTP статус (invalid_client → 401, бусад → 400).
	Status int
}

func (e *TokenError) Error() string { return e.Code + ": " + e.Description }

func badGrant(desc string) *TokenError {
	return &TokenError{Code: "invalid_grant", Description: desc, Status: 400}
}

// TokenRequest нь `/oauth2/token`-ийн задлагдсан параметрүүд.
type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	CodeVerifier string
	RefreshToken string
	Scope        string
	// Client-ийн танилт: Basic header-ээс эсвэл биетээс.
	ClientID     string
	ClientSecret string
	// SecretFromBasic нь итгэмжлэл Authorization: Basic-ээс ирсэн эсэхийг заана.
	SecretFromBasic bool
}

// TokenResponse нь амжилттай token хариу (RFC 6749 §5.1).
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope"`
}

// Token нь `/oauth2/token`-ийг үйлчилнэ.
func (s *Service) Token(ctx context.Context, req TokenRequest) (*TokenResponse, error) {
	client, err := s.AuthenticateClient(ctx, req)
	if err != nil {
		return nil, err
	}

	switch req.GrantType {
	case domain.GrantAuthorizationCode:
		return s.exchangeCode(ctx, client, req)
	case domain.GrantRefreshToken:
		return s.refresh(ctx, client, req)
	case domain.GrantClientCredentials:
		return s.clientCredentials(ctx, client, req)
	default:
		return nil, &TokenError{Code: "unsupported_grant_type", Description: "unsupported grant_type", Status: 400}
	}
}

// AuthenticateClient нь client-ийг бүртгэгдсэн auth method-оор нь баталгаажуулна.
//
// Client-ийн зарласан арга нь ХАТУУ — `client_secret_basic`-тэй client биеттэй
// secret илгээвэл татгалзана. Ингэснээр аргыг доошлуулах (downgrade) оролдлого
// боломжгүй.
func (s *Service) AuthenticateClient(ctx context.Context, req TokenRequest) (domain.OAuthClient, error) {
	var client domain.OAuthClient
	if req.ClientID == "" {
		return client, &TokenError{Code: "invalid_client", Description: "client_id is required", Status: 401}
	}

	client, err := s.clients.Get(ctx, req.ClientID)
	if err != nil {
		if apperror.IsNotFound(err) {
			return client, &TokenError{Code: "invalid_client", Description: "unknown client", Status: 401}
		}
		return client, err
	}
	if !client.Enabled {
		return client, &TokenError{Code: "invalid_client", Description: "client is disabled", Status: 401}
	}

	switch client.TokenEndpointAuthMethod {
	case domain.AuthMethodNone:
		// Public client — secret байх ЁСГҮЙ. Хамгаалалт нь PKCE.
		if req.ClientSecret != "" {
			return client, &TokenError{Code: "invalid_client", Description: "public client must not send a secret", Status: 401}
		}
		return client, nil

	case domain.AuthMethodBasic:
		if !req.SecretFromBasic {
			return client, &TokenError{Code: "invalid_client", Description: "client must authenticate with HTTP Basic", Status: 401}
		}
	case domain.AuthMethodPost:
		if req.SecretFromBasic {
			return client, &TokenError{Code: "invalid_client", Description: "client must authenticate with client_secret_post", Status: 401}
		}
	default:
		return client, &TokenError{Code: "invalid_client", Description: "unsupported client authentication method", Status: 401}
	}

	if req.ClientSecret == "" || client.SecretHash == "" {
		return client, &TokenError{Code: "invalid_client", Description: "invalid client credentials", Status: 401}
	}
	ok, err := secrethash.Verify(client.SecretHash, req.ClientSecret)
	if err != nil || !ok {
		// Формат танигдахгүй байсан ч "буруу итгэмжлэл" гэж хариулна — дотоод
		// байдлыг ил гаргахгүй (fail-closed).
		return client, &TokenError{Code: "invalid_client", Description: "invalid client credentials", Status: 401}
	}
	return client, nil
}

// exchangeCode нь authorization code-ыг token болгож солино.
func (s *Service) exchangeCode(ctx context.Context, client domain.OAuthClient, req TokenRequest) (*TokenResponse, error) {
	if !client.HasGrant(domain.GrantAuthorizationCode) {
		return nil, &TokenError{Code: "unauthorized_client", Description: "client may not use this grant", Status: 400}
	}
	if req.Code == "" {
		return nil, &TokenError{Code: "invalid_request", Description: "code is required", Status: 400}
	}

	code, alreadyUsed, err := s.flow.ConsumeCode(flowCtx(ctx), hashToken(req.Code))
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, badGrant("authorization code is invalid")
		}
		if apperror.Is(err, apperror.ErrTypeBadRequest) {
			return nil, badGrant("authorization code has expired")
		}
		return nil, err
	}

	// Дахин ашиглалт: код нэгэнт солигдсон байна. Түүгээр гаргасан бүх token-ыг
	// цуцална — код алдагдсан бол халдагчийн авсан session ажиллахгүй болно
	// (RFC 6749 §4.1.2, RFC 9700 §2.1.1).
	if alreadyUsed {
		if code.Subject != "" {
			// Код ямар token гаргасныг холбосон бичлэг байхгүй (гэр бүл нь
			// гаргах мөчид үүсдэг) тул тухайн иргэн+апп-ийн бүхнийг цуцална.
			_ = s.flow.RevokeForSubjectClient(flowCtx(ctx), code.Subject, code.ClientID)
		}
		return nil, badGrant("authorization code has already been used")
	}

	// Код нь ЯГ энэ client-д олгогдсон байх ёстой.
	if code.ClientID != client.ClientID {
		return nil, badGrant("authorization code was issued to another client")
	}
	// redirect_uri нь authorize үеийнхтэй ижил байх ёстой (RFC 6749 §4.1.3).
	if req.RedirectURI != code.RedirectURI {
		return nil, badGrant("redirect_uri does not match the authorization request")
	}
	if !VerifyPKCE(code.CodeChallenge, code.CodeChallengeMethod, req.CodeVerifier) {
		return nil, badGrant("code_verifier does not match the code_challenge")
	}

	return s.issue(ctx, client, code.Subject, code.Scopes, code.Nonce, code.AuthTime, "")
}

// refresh нь refresh token-ыг эргүүлж шинэ хосыг гаргана.
func (s *Service) refresh(ctx context.Context, client domain.OAuthClient, req TokenRequest) (*TokenResponse, error) {
	if !client.HasGrant(domain.GrantRefreshToken) {
		return nil, &TokenError{Code: "unauthorized_client", Description: "client may not use this grant", Status: 400}
	}
	if req.RefreshToken == "" {
		return nil, &TokenError{Code: "invalid_request", Description: "refresh_token is required", Status: 400}
	}

	rt, reused, err := s.flow.ConsumeRefreshToken(flowCtx(ctx), hashToken(req.RefreshToken))
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, badGrant("refresh token is invalid")
		}
		if apperror.Is(err, apperror.ErrTypeBadRequest) {
			return nil, badGrant("refresh token has expired")
		}
		return nil, err
	}

	// Хэрэглэгдсэн refresh token дахин ирлээ = хулгайн шинж. Гэр бүлийг бүхэлд
	// нь цуцална: хууль ёсны эзэн ч, халдагч ч дахин нэвтрэх шаардлагатай болно
	// (RFC 9700 §4.14.2).
	if reused {
		_ = s.flow.RevokeFamily(flowCtx(ctx), rt.FamilyID)
		return nil, badGrant("refresh token has already been used")
	}
	if rt.ClientID != client.ClientID {
		_ = s.flow.RevokeFamily(flowCtx(ctx), rt.FamilyID)
		return nil, badGrant("refresh token was issued to another client")
	}

	// Scope-ыг НАРИЙСГАЖ болно, өргөтгөж БОЛОХГҮЙ (RFC 6749 §6).
	scopes := rt.Scopes
	if req.Scope != "" {
		narrowed := intersect(rt.Scopes, splitScope(req.Scope))
		if len(narrowed) == 0 {
			return nil, &TokenError{Code: "invalid_scope", Description: "requested scope exceeds the original grant", Status: 400}
		}
		scopes = narrowed
	}

	return s.issue(ctx, client, rt.Subject, scopes, rt.Nonce, rt.AuthTime, rt.FamilyID)
}

// clientCredentials нь хэрэглэгчгүй (m2m) token гаргана.
func (s *Service) clientCredentials(ctx context.Context, client domain.OAuthClient, req TokenRequest) (*TokenResponse, error) {
	if !client.HasGrant(domain.GrantClientCredentials) {
		return nil, &TokenError{Code: "unauthorized_client", Description: "client may not use this grant", Status: 400}
	}
	if client.IsPublic() {
		return nil, &TokenError{Code: "invalid_client", Description: "public clients may not use client_credentials", Status: 401}
	}

	scopes := client.Scopes
	if req.Scope != "" {
		scopes = client.FilterAllowedScopes(splitScope(req.Scope))
		if len(scopes) == 0 {
			return nil, &TokenError{Code: "invalid_scope", Description: "none of the requested scopes are allowed", Status: 400}
		}
	}

	access := randomToken()
	expires := time.Now().Add(AccessTokenTTL)
	if err := s.flow.StoreTokens(flowCtx(ctx), domain.OAuthAccessToken{
		TokenHash: hashToken(access),
		ClientID:  client.ClientID,
		Scopes:    scopes,
		ExpiresAt: expires,
	}, nil); err != nil {
		return nil, err
	}
	// client_credentials-д хэрэглэгч байхгүй тул id_token ч, refresh ч байхгүй.
	return &TokenResponse{
		AccessToken: access,
		TokenType:   "bearer",
		ExpiresIn:   int64(AccessTokenTTL.Seconds()),
		Scope:       joinScope(scopes),
	}, nil
}

// issue нь access (+refresh, +id_token) хосыг гаргаж хадгална.
//
// family нь хоосон бол шинэ эргэлтийн гэр бүл эхэлнэ; эс бөгөөс өмнөхийг
// үргэлжлүүлнэ (эргэлт).
func (s *Service) issue(ctx context.Context, client domain.OAuthClient, subject string, scopes []string, nonce string, authTime time.Time, family string) (*TokenResponse, error) {
	access := randomToken()
	now := time.Now()
	expires := now.Add(AccessTokenTTL)

	// offline_access-гүй бол refresh token гаргахгүй (OIDC Core §11).
	wantRefresh := client.HasGrant(domain.GrantRefreshToken) && containsString(scopes, ScopeOfflineAccess)
	if family == "" && wantRefresh {
		family = uuid.NewString()
	}

	at := domain.OAuthAccessToken{
		TokenHash:     hashToken(access),
		ClientID:      client.ClientID,
		Subject:       subject,
		Scopes:        scopes,
		RefreshFamily: family,
		ExpiresAt:     expires,
	}

	var refresh string
	var rt *domain.OAuthRefreshToken
	if wantRefresh {
		refresh = randomToken()
		rt = &domain.OAuthRefreshToken{
			TokenHash: hashToken(refresh),
			FamilyID:  family,
			ClientID:  client.ClientID,
			Subject:   subject,
			Scopes:    scopes,
			Nonce:     nonce,
			AuthTime:  authTime,
			ExpiresAt: now.Add(RefreshTokenTTL),
		}
	}

	if err := s.flow.StoreTokens(flowCtx(ctx), at, rt); err != nil {
		return nil, err
	}

	resp := &TokenResponse{
		AccessToken:  access,
		TokenType:    "bearer",
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        joinScope(scopes),
	}

	// id_token нь ЗӨВХӨН openid scope-той үед (OIDC Core §3.1.3.3).
	if containsString(scopes, scopeOpenID) {
		idToken, err := s.mintIDToken(ctx, client, subject, scopes, nonce, authTime)
		if err != nil {
			return nil, err
		}
		resp.IDToken = idToken
	}
	return resp, nil
}

// mintIDToken нь иргэний claims-ыг угсарч RS256-аар гарын үсэг зурна.
func (s *Service) mintIDToken(ctx context.Context, client domain.OAuthClient, subject string, scopes []string, nonce string, authTime time.Time) (string, error) {
	if s.keys == nil || s.users == nil {
		return "", apperror.InternalCause(fmt.Errorf("oidc: id_token requires a key manager and user lookup"))
	}
	// Хэрэглэгчийг заавал уншина — уншиж чадахгүй бол token гаргахгүй
	// (fail-closed). Эс бөгөөс RP claims-гүй token авч хэрэглэгчийг таньж чадахгүй.
	//
	// Token endpoint нь нэвтрээгүй дуудагдана (client итгэмжлэлээр) тул context-д
	// RLS identity байхгүй. `users` хүснэгт RLS-тэй учир identity-гүйгээр уншвал
	// бүх мөр хаагдаж "user not found" болно. Токеныг ЯГ энэ subject-д гаргаж
	// байгаа тул түүний ӨӨРИЙН мөрийг user үүргээр уншина (users_self бодлого) —
	// service үүрэг өгвөл бүх хэрэглэгч нээгдэх тул хэрэггүй өргөн эрх болно.
	resp, err := s.users.GetByID(rls.WithUser(ctx, subject), usersuc.GetByIDRequest{ID: subject})
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("oidc: load user for id_token: %w", err))
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": s.issuer,
		"sub": subject,
		"aud": client.ClientID,
		"iat": now.Unix(),
		"exp": now.Add(IDTokenTTL).Unix(),
	}
	if !authTime.IsZero() {
		claims["auth_time"] = authTime.Unix()
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	for k, v := range ClaimsForScopes(scopes, resp.User) {
		claims[k] = v
	}

	kid, priv, err := s.keys.Signer(ctx)
	if err != nil {
		return "", err
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("oidc: sign id_token: %w", err))
	}
	return signed, nil
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func joinScope(scopes []string) string {
	out := ""
	for i, s := range scopes {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

// parseIDTokenHint нь RP-ийн logout дээр өгсөн id_token_hint-ээс аль client,
// аль иргэний тухай яриад байгааг гаргаж авна (OIDC RP-Initiated Logout §3).
//
// Гарын үсгийг ЗААВАЛ шалгана — эс бөгөөс дурын хүн `aud`-аа сонгосон hint
// зохиож, өөр апп-ийн нэрийн өмнөөс logout эхлүүлэх боломжтой болно.
//
// Хугацаа дууссаныг ЗӨВШӨӨРНӨ: hint нь ӨНГӨРСӨН session-ий тухай сануулга тул
// хүчинтэй байх шаардлагагүй (спекц үүнийг тусгайлан зөвшөөрдөг). Тиймээс
// хугацааны шалгалтыг унтраасан — энэ нь эрх олгодоггүй, зөвхөн аль client
// гэдгийг заадаг ба буцах хаяг нь тэр client-ийн бүртгэлтэй тулгагдсаар байна.
func (s *Service) parseIDTokenHint(ctx context.Context, hint string) (clientID, subject string, err error) {
	if s.keys == nil {
		return "", "", apperror.BadRequest("id_token_hint is not supported")
	}
	tok, err := jwt.Parse(hint, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("id_token_hint has no kid")
		}
		return s.keys.PublicKey(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(s.issuer),
		jwt.WithoutClaimsValidation(), // exp-ийг санаатайгаар алгасна
	)
	if err != nil || !tok.Valid {
		return "", "", apperror.BadRequest("id_token_hint could not be verified")
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", apperror.BadRequest("id_token_hint has unexpected claims")
	}
	// `iss`-ийг гараар шалгана: WithoutClaimsValidation нь WithIssuer-ыг ч
	// унтраадаг тул энд заавал тулгана.
	if iss, _ := claims["iss"].(string); iss != s.issuer {
		return "", "", apperror.BadRequest("id_token_hint was issued by someone else")
	}
	subject, _ = claims["sub"].(string)
	switch aud := claims["aud"].(type) {
	case string:
		clientID = aud
	case []any:
		if len(aud) > 0 {
			clientID, _ = aud[0].(string)
		}
	}
	if clientID == "" {
		return "", "", apperror.BadRequest("id_token_hint has no audience")
	}
	return clientID, subject, nil
}

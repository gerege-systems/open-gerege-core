// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"context"
	"errors"

	"template/internal/apperror"
	"template/internal/business/domain"
	usersuc "template/internal/business/usecases/users"
	"template/internal/datasources/rls"
)

var errUserLookupMissing = errors.New("oidc: userinfo requires a user lookup")

// TokenInfo нь access token-ийн шалгасан төлөв.
type TokenInfo struct {
	Active    bool     `json:"active"`
	Scope     string   `json:"scope,omitempty"`
	ClientID  string   `json:"client_id,omitempty"`
	Subject   string   `json:"sub,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	TokenType string   `json:"token_type,omitempty"`
	Scopes    []string `json:"-"`
}

// Introspect нь access token-ыг шалгана (RFC 7662).
//
// caller нь дуудаж буй client-ийн ID. ӨӨР client-ийн token-ыг шалгах боломжгүй
// (RFC 7662 §2.1) — эс бөгөөс token-ыг хаанаас нэгээс олсон хэн ч түүний эзэн
// иргэний тогтвортой `sub`, аль RP-д харьяалагдахыг нь болон хүчинтэй эсэхийг
// мэдэх болно. caller хоосон бол ДОТООД дуудлага (bearer middleware) гэж үзнэ.
//
// Танигдаагүй/дууссан/цуцлагдсан/өөр client-ийн token бүхэнд ялгаагүй
// `{"active": false}` буцаана — шалтгааныг нь ялгаж хэлэхгүй.
func (s *Service) Introspect(ctx context.Context, caller, token string) TokenInfo {
	if token == "" {
		return TokenInfo{Active: false}
	}
	at, err := s.flow.AccessToken(flowCtx(ctx), hashToken(token))
	if err != nil {
		return TokenInfo{Active: false}
	}
	if caller != "" && at.ClientID != caller {
		return TokenInfo{Active: false}
	}
	return TokenInfo{
		Active:    true,
		Scope:     joinScope(at.Scopes),
		Scopes:    at.Scopes,
		ClientID:  at.ClientID,
		Subject:   at.Subject,
		ExpiresAt: at.ExpiresAt.Unix(),
		TokenType: "Bearer",
	}
}

// Userinfo нь access token-ий эзэн иргэний claims-ыг буцаана (OIDC Core §5.3).
func (s *Service) Userinfo(ctx context.Context, token string) (map[string]any, error) {
	// Дотоод дуудлага: userinfo-г token өөрөө эрхшээдэг тул caller хоосон.
	info := s.Introspect(ctx, "", token)
	if !info.Active {
		return nil, apperror.Unauthorized("invalid or expired access token")
	}
	// client_credentials token-д хэрэглэгч байхгүй тул userinfo утгагүй.
	if info.Subject == "" {
		return nil, apperror.Forbidden("token has no subject")
	}
	if !containsString(info.Scopes, scopeOpenID) {
		return nil, apperror.Forbidden("token does not carry the openid scope")
	}
	if s.users == nil {
		return nil, apperror.InternalCause(errUserLookupMissing)
	}

	// mintIDToken-той ижил шалтгаан: userinfo нь access token-оор эрхшээгддэг тул
	// context-д RLS identity байхгүй. Token-ий эзний ӨӨРИЙН мөрийг user үүргээр.
	resp, err := s.users.GetByID(rls.WithUser(ctx, info.Subject), usersuc.GetByIDRequest{ID: info.Subject})
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	claims := ClaimsForScopes(info.Scopes, resp.User)
	claims["sub"] = info.Subject
	return claims, nil
}

// Revoke нь token-ыг цуцална (RFC 7009).
//
// RFC-ийн дагуу танигдаагүй token ч АМЖИЛТТАЙ гэж хариулна — client нь token
// хүчинтэй байсан эсэхийг мэдэх ёсгүй. Гэхдээ ӨӨР client-ийн token-ыг цуцлах
// боломжгүй (эзэмшлийг шалгана).
func (s *Service) Revoke(ctx context.Context, client domain.OAuthClient, token, hint string) error {
	if token == "" {
		return nil
	}
	h := hashToken(token)

	// Hint-ийг эхэлж оролдоод, олдохгүй бол нөгөөг нь — hint нь зөвлөмж төдий.
	try := []string{"access_token", "refresh_token"}
	if hint == "refresh_token" {
		try = []string{"refresh_token", "access_token"}
	}
	for _, kind := range try {
		var found bool
		var err error
		if kind == "access_token" {
			found, err = s.flow.RevokeAccessToken(flowCtx(ctx), h, client.ClientID)
		} else {
			found, err = s.flow.RevokeRefreshToken(flowCtx(ctx), h, client.ClientID)
		}
		if err != nil {
			return err
		}
		if found {
			return nil
		}
	}
	return nil // танигдсангүй — RFC-ийн дагуу амжилттай
}

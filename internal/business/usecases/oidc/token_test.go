// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	usersuc "template/internal/business/usecases/users"
	"template/pkg/secrethash"
)

// ── token-д зориулсан хуурамч хадгалалт ─────────────────────────────────────

type tokenFlow struct {
	*fakeFlow
	codes      map[string]domain.OAuthAuthCode
	usedCodes  map[string]bool
	refresh    map[string]domain.OAuthRefreshToken
	usedRT     map[string]bool
	revokedFam map[string]bool
	revokedSC  map[string]bool
	revokedAT  map[string]bool
	access     []domain.OAuthAccessToken
	stored     []domain.OAuthRefreshToken
}

func newTokenFlow() *tokenFlow {
	return &tokenFlow{
		fakeFlow: newFakeFlow(),
		codes:    map[string]domain.OAuthAuthCode{}, usedCodes: map[string]bool{},
		refresh: map[string]domain.OAuthRefreshToken{}, usedRT: map[string]bool{},
		revokedFam: map[string]bool{}, revokedSC: map[string]bool{}, revokedAT: map[string]bool{},
	}
}

func (f *tokenFlow) CreateCode(_ context.Context, c domain.OAuthAuthCode) error {
	f.codes[string(c.CodeHash)] = c
	return nil
}

func (f *tokenFlow) ConsumeCode(_ context.Context, h []byte) (domain.OAuthAuthCode, bool, error) {
	c, ok := f.codes[string(h)]
	if !ok {
		return domain.OAuthAuthCode{}, false, apperror.NotFound("authorization code not found")
	}
	if f.usedCodes[string(h)] {
		return c, true, nil
	}
	if time.Now().After(c.ExpiresAt) {
		return c, false, apperror.BadRequest("authorization code expired")
	}
	f.usedCodes[string(h)] = true
	return c, false, nil
}

func (f *tokenFlow) StoreTokens(_ context.Context, at domain.OAuthAccessToken, rt *domain.OAuthRefreshToken) error {
	f.access = append(f.access, at)
	if rt != nil {
		f.refresh[string(rt.TokenHash)] = *rt
		f.stored = append(f.stored, *rt)
	}
	return nil
}

func (f *tokenFlow) ConsumeRefreshToken(_ context.Context, h []byte) (domain.OAuthRefreshToken, bool, error) {
	rt, ok := f.refresh[string(h)]
	if !ok {
		return domain.OAuthRefreshToken{}, false, apperror.NotFound("refresh token not found")
	}
	if f.usedRT[string(h)] || f.revokedFam[rt.FamilyID] {
		return rt, true, nil
	}
	if time.Now().After(rt.ExpiresAt) {
		return rt, false, apperror.BadRequest("refresh token expired")
	}
	f.usedRT[string(h)] = true
	return rt, false, nil
}

func (f *tokenFlow) RevokeFamily(_ context.Context, familyID string) error {
	f.revokedFam[familyID] = true
	return nil
}

func (f *tokenFlow) RevokeForSubjectClient(_ context.Context, subject, clientID string) error {
	f.revokedSC[subject+"|"+clientID] = true
	return nil
}

func (f *tokenFlow) AccessToken(_ context.Context, h []byte) (domain.OAuthAccessToken, error) {
	for _, at := range f.access {
		if string(at.TokenHash) == string(h) && !f.revokedAT[string(h)] && time.Now().Before(at.ExpiresAt) {
			return at, nil
		}
	}
	return domain.OAuthAccessToken{}, apperror.NotFound("token not found")
}

func (f *tokenFlow) RevokeAccessToken(_ context.Context, h []byte, clientID string) (bool, error) {
	for _, at := range f.access {
		if string(at.TokenHash) == string(h) && at.ClientID == clientID {
			f.revokedAT[string(h)] = true
			return true, nil
		}
	}
	return false, nil
}

func (f *tokenFlow) RevokeRefreshToken(_ context.Context, h []byte, clientID string) (bool, error) {
	rt, ok := f.refresh[string(h)]
	if !ok || rt.ClientID != clientID {
		return false, nil
	}
	f.revokedFam[rt.FamilyID] = true
	return true, nil
}

type fakeUsers struct{ err error }

func (f *fakeUsers) GetByID(context.Context, usersuc.GetByIDRequest) (usersuc.GetByIDResponse, error) {
	if f.err != nil {
		return usersuc.GetByIDResponse{}, f.err
	}
	return usersuc.GetByIDResponse{User: domain.User{
		FirstName: "Бат", LastName: "Дорж", Email: "bat@example.mn",
	}}, nil
}

const testClientSecret = "a-well-known-client-secret"

func confidentialClient(t *testing.T) domain.OAuthClient {
	t.Helper()
	h, err := secrethash.Hash(testClientSecret)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	c := webClient()
	c.SecretHash = h
	c.Scopes = append(c.Scopes, ScopeOfflineAccess)
	return c
}

func tokenService(t *testing.T, c domain.OAuthClient) (*Service, *tokenFlow) {
	t.Helper()
	flow := newTokenFlow()
	km, err := NewKeyManager(&memKeys{}, "token-test-encryption-key")
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if err := km.EnsureKey(context.Background()); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	svc := NewService(&fakeClients{c: c}, flow, "https://sso.dgov.mn").WithTokenIssuing(km, &fakeUsers{})
	return svc, flow
}

func tokErr(t *testing.T, err error) *TokenError {
	t.Helper()
	var e *TokenError
	if !errors.As(err, &e) {
		t.Fatalf("expected a *TokenError, got %T (%v)", err, err)
	}
	return e
}

// issueCode нь authorize→consent урсгалыг гүйцээж code-ыг буцаана.
func issueCode(t *testing.T, s *Service, flow *tokenFlow, scope string) (code, verifier string) {
	t.Helper()
	verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))

	req := validRequest()
	req.Scope = scope
	req.CodeChallenge = base64.RawURLEncoding.EncodeToString(sum[:])
	req.CodeChallengeMethod = "S256"

	ctx := context.Background()
	ch, _, err := s.Authorize(ctx, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	consentCh, _, err := s.AcceptLogin(ctx, ch, testSubject)
	if err != nil {
		t.Fatalf("AcceptLogin: %v", err)
	}
	redirect, err := s.AcceptConsent(ctx, consentCh, testSubject, nil)
	if err != nil {
		t.Fatalf("AcceptConsent: %v", err)
	}
	idx := strings.Index(redirect, "code=")
	if idx < 0 {
		t.Fatalf("no code in redirect %q", redirect)
	}
	code = redirect[idx+5:]
	if amp := strings.Index(code, "&"); amp >= 0 {
		code = code[:amp]
	}
	return code, verifier
}

func codeRequest(code, verifier string) TokenRequest {
	return TokenRequest{
		GrantType:       domain.GrantAuthorizationCode,
		Code:            code,
		RedirectURI:     "https://ring.dgov.mn/sso/callback",
		CodeVerifier:    verifier,
		ClientID:        "ring-dgov-mn",
		ClientSecret:    testClientSecret,
		SecretFromBasic: true,
	}
}

// ── тестүүд ──────────────────────────────────────────────────────────────────

func TestCodeExchangeIssuesTokens(t *testing.T) {
	c := confidentialClient(t)
	s, flow := tokenService(t, c)
	code, verifier := issueCode(t, s, flow, "openid profile offline_access")

	resp, err := s.Token(context.Background(), codeRequest(code, verifier))
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if resp.AccessToken == "" || resp.TokenType != "bearer" {
		t.Fatalf("bad token response: %+v", resp)
	}
	if resp.IDToken == "" {
		t.Fatal("openid was requested so an id_token must be returned")
	}
	if resp.RefreshToken == "" {
		t.Fatal("offline_access was granted so a refresh token must be returned")
	}
	if resp.ExpiresIn <= 0 {
		t.Fatal("expires_in must be positive")
	}

	// Access token нь ЗӨВХӨН hash хэлбэрээр хадгалагдана.
	if len(flow.access) != 1 {
		t.Fatalf("expected one stored access token, got %d", len(flow.access))
	}
	want := sha256.Sum256([]byte(resp.AccessToken))
	if string(flow.access[0].TokenHash) != string(want[:]) {
		t.Fatal("the stored access token is not the sha256 of the issued one")
	}
}

// offline_access хүсээгүй бол refresh token гарахгүй (OIDC §11).
func TestNoRefreshTokenWithoutOfflineAccess(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	code, verifier := issueCode(t, s, flow, "openid profile")

	resp, err := s.Token(context.Background(), codeRequest(code, verifier))
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if resp.RefreshToken != "" {
		t.Fatal("a refresh token must not be issued without offline_access")
	}
}

// Код хоёр дахь удаа ирвэл татгалзаад, тухайн иргэн+апп-ийн token-ыг цуцална.
func TestAuthorizationCodeReplayRevokesTokens(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	code, verifier := issueCode(t, s, flow, "openid offline_access")
	ctx := context.Background()

	if _, err := s.Token(ctx, codeRequest(code, verifier)); err != nil {
		t.Fatalf("first exchange: %v", err)
	}

	_, err := s.Token(ctx, codeRequest(code, verifier))
	if e := tokErr(t, err); e.Code != "invalid_grant" {
		t.Fatalf("replayed code gave %q, want invalid_grant", e.Code)
	}
	if !flow.revokedSC[testSubject+"|ring-dgov-mn"] {
		t.Fatal("replaying a code must revoke the tokens it produced")
	}
}

func TestCodeExchangeRequiresMatchingRedirectURI(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	code, verifier := issueCode(t, s, flow, "openid")

	req := codeRequest(code, verifier)
	req.RedirectURI = "https://ring.dgov.mn/other"
	if e := tokErr(t, mustErr(t, s, req)); e.Code != "invalid_grant" {
		t.Fatalf("error = %q, want invalid_grant", e.Code)
	}
}

func TestCodeExchangeRequiresCorrectPKCEVerifier(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	code, verifier := issueCode(t, s, flow, "openid")

	req := codeRequest(code, verifier+"tampered")
	if e := tokErr(t, mustErr(t, s, req)); e.Code != "invalid_grant" {
		t.Fatalf("error = %q, want invalid_grant", e.Code)
	}
}

func TestClientAuthentication(t *testing.T) {
	c := confidentialClient(t)

	t.Run("wrong secret is refused", func(t *testing.T) {
		s, flow := tokenService(t, c)
		code, verifier := issueCode(t, s, flow, "openid")
		req := codeRequest(code, verifier)
		req.ClientSecret = "not-the-secret"
		if e := tokErr(t, mustErr(t, s, req)); e.Code != "invalid_client" || e.Status != 401 {
			t.Fatalf("error = %+v, want invalid_client/401", e)
		}
	})

	t.Run("missing secret is refused", func(t *testing.T) {
		s, flow := tokenService(t, c)
		code, verifier := issueCode(t, s, flow, "openid")
		req := codeRequest(code, verifier)
		req.ClientSecret = ""
		if e := tokErr(t, mustErr(t, s, req)); e.Code != "invalid_client" {
			t.Fatalf("error = %q", e.Code)
		}
	})

	// client_secret_basic-тэй client биеттэй secret илгээвэл татгалзана —
	// баталгаажуулах аргыг доошлуулах оролдлогыг хаана.
	t.Run("declared auth method is enforced", func(t *testing.T) {
		s, flow := tokenService(t, c)
		code, verifier := issueCode(t, s, flow, "openid")
		req := codeRequest(code, verifier)
		req.SecretFromBasic = false
		if e := tokErr(t, mustErr(t, s, req)); e.Code != "invalid_client" {
			t.Fatalf("error = %q; a basic-auth client must not be accepted via the body", e.Code)
		}
	})

	t.Run("public client must not send a secret", func(t *testing.T) {
		pub := webClient()
		pub.TokenEndpointAuthMethod = domain.AuthMethodNone
		s, flow := tokenService(t, pub)
		code, verifier := issueCode(t, s, flow, "openid")
		req := codeRequest(code, verifier)
		req.SecretFromBasic = false
		if e := tokErr(t, mustErr(t, s, req)); e.Code != "invalid_client" {
			t.Fatalf("error = %q", e.Code)
		}
	})
}

// Эргэлт: хуучин refresh token дахин ирвэл БҮХ гэр бүлийг цуцална.
func TestRefreshRotationDetectsReuse(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	ctx := context.Background()
	code, verifier := issueCode(t, s, flow, "openid offline_access")

	first, err := s.Token(ctx, codeRequest(code, verifier))
	if err != nil {
		t.Fatalf("code exchange: %v", err)
	}

	refreshReq := TokenRequest{
		GrantType: domain.GrantRefreshToken, RefreshToken: first.RefreshToken,
		ClientID: "ring-dgov-mn", ClientSecret: testClientSecret, SecretFromBasic: true,
	}
	second, err := s.Token(ctx, refreshReq)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("the refresh token must rotate")
	}

	// Хуучныг дахин ашиглах = хулгайн шинж.
	if e := tokErr(t, mustErr(t, s, refreshReq)); e.Code != "invalid_grant" {
		t.Fatalf("error = %q", e.Code)
	}
	fam := flow.stored[0].FamilyID
	if !flow.revokedFam[fam] {
		t.Fatal("reusing a rotated refresh token must revoke the whole family")
	}

	// Шинэ token нь гэр бүл цуцлагдсаны дараа ажиллахгүй.
	newReq := refreshReq
	newReq.RefreshToken = second.RefreshToken
	if _, err := s.Token(ctx, newReq); err == nil {
		t.Fatal("after a family revocation even the newest refresh token must stop working")
	}
}

func TestRefreshCannotWidenScope(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	ctx := context.Background()
	code, verifier := issueCode(t, s, flow, "openid offline_access")

	first, err := s.Token(ctx, codeRequest(code, verifier))
	if err != nil {
		t.Fatalf("code exchange: %v", err)
	}

	widen := TokenRequest{
		GrantType: domain.GrantRefreshToken, RefreshToken: first.RefreshToken,
		Scope:    "openid profile email svc:eid-proxy",
		ClientID: "ring-dgov-mn", ClientSecret: testClientSecret, SecretFromBasic: true,
	}
	resp, err := s.Token(ctx, widen)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if strings.Contains(resp.Scope, "svc:eid-proxy") || strings.Contains(resp.Scope, "profile") {
		t.Fatalf("refresh widened the grant to %q", resp.Scope)
	}
}

func TestClientCredentialsHasNoUserOrRefresh(t *testing.T) {
	c := confidentialClient(t)
	c.GrantTypes = []string{domain.GrantClientCredentials}
	s, _ := tokenService(t, c)

	resp, err := s.Token(context.Background(), TokenRequest{
		GrantType: domain.GrantClientCredentials,
		ClientID:  "ring-dgov-mn", ClientSecret: testClientSecret, SecretFromBasic: true,
	})
	if err != nil {
		t.Fatalf("client_credentials: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatal("no access token")
	}
	if resp.IDToken != "" {
		t.Fatal("client_credentials has no user, so it must not produce an id_token")
	}
	if resp.RefreshToken != "" {
		t.Fatal("client_credentials must not produce a refresh token")
	}
}

func TestUnsupportedGrantType(t *testing.T) {
	s, _ := tokenService(t, confidentialClient(t))
	err := mustErr(t, s, TokenRequest{
		GrantType: "password",
		ClientID:  "ring-dgov-mn", ClientSecret: testClientSecret, SecretFromBasic: true,
	})
	if e := tokErr(t, err); e.Code != "unsupported_grant_type" {
		t.Fatalf("error = %q", e.Code)
	}
}

// id_token үүсгэхэд иргэний бүртгэл уншигдахгүй бол token гаргахгүй (fail-closed).
func TestIDTokenFailsClosedWhenUserCannotBeLoaded(t *testing.T) {
	flow := newTokenFlow()
	km, _ := NewKeyManager(&memKeys{}, "token-test-encryption-key")
	if err := km.EnsureKey(context.Background()); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	c := confidentialClient(t)
	s := NewService(&fakeClients{c: c}, flow, "https://sso.dgov.mn").
		WithTokenIssuing(km, &fakeUsers{err: errors.New("db down")})

	code, verifier := issueCode(t, s, flow, "openid profile")
	if _, err := s.Token(context.Background(), codeRequest(code, verifier)); err == nil {
		t.Fatal("a token must not be issued when the user record cannot be read")
	}
}

func mustErr(t *testing.T, s *Service, req TokenRequest) error {
	t.Helper()
	_, err := s.Token(context.Background(), req)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

// ── introspect / userinfo / revoke ───────────────────────────────────────────

func TestIntrospectRevealsNothingForUnknownTokens(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	code, verifier := issueCode(t, s, flow, "openid offline_access")
	ctx := context.Background()

	resp, err := s.Token(ctx, codeRequest(code, verifier))
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	live := s.Introspect(ctx, "", resp.AccessToken)
	if !live.Active || live.Subject != testSubject || live.ClientID != "ring-dgov-mn" {
		t.Fatalf("a live token should introspect as active: %+v", live)
	}

	// Танигдаагүй, хоосон, гуйвуулсан token бүгд ижилхэн "active: false" —
	// шалтгааныг нь ялгаж хэлэхгүй.
	for _, tok := range []string{"", "not-a-token", resp.AccessToken + "x"} {
		if got := s.Introspect(ctx, "", tok); got.Active || got.Subject != "" || got.ClientID != "" {
			t.Fatalf("introspecting %q leaked state: %+v", tok, got)
		}
	}
}

func TestUserinfoRequiresOpenIDAndASubject(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	ctx := context.Background()

	t.Run("openid token works", func(t *testing.T) {
		code, verifier := issueCode(t, s, flow, "openid profile")
		resp, err := s.Token(ctx, codeRequest(code, verifier))
		if err != nil {
			t.Fatalf("Token: %v", err)
		}
		claims, err := s.Userinfo(ctx, resp.AccessToken)
		if err != nil {
			t.Fatalf("Userinfo: %v", err)
		}
		if claims["sub"] != testSubject {
			t.Fatalf("sub = %v, want the subject the token was issued for", claims["sub"])
		}
		if claims["name"] == nil {
			t.Fatal("profile was granted so the name claim should be present")
		}
	})

	t.Run("token without openid is refused", func(t *testing.T) {
		code, verifier := issueCode(t, s, flow, "profile")
		resp, err := s.Token(ctx, codeRequest(code, verifier))
		if err != nil {
			t.Fatalf("Token: %v", err)
		}
		if _, err := s.Userinfo(ctx, resp.AccessToken); err == nil {
			t.Fatal("userinfo must require the openid scope")
		}
	})

	t.Run("invalid token is refused", func(t *testing.T) {
		if _, err := s.Userinfo(ctx, "nope"); err == nil {
			t.Fatal("userinfo must reject an unknown token")
		}
	})
}

// client_credentials token-д хэрэглэгч байхгүй тул userinfo өгөх ёсгүй.
func TestUserinfoRefusesSubjectlessTokens(t *testing.T) {
	c := confidentialClient(t)
	c.GrantTypes = []string{domain.GrantClientCredentials}
	s, _ := tokenService(t, c)
	ctx := context.Background()

	resp, err := s.Token(ctx, TokenRequest{
		GrantType: domain.GrantClientCredentials,
		ClientID:  "ring-dgov-mn", ClientSecret: testClientSecret, SecretFromBasic: true,
	})
	if err != nil {
		t.Fatalf("client_credentials: %v", err)
	}
	if _, err := s.Userinfo(ctx, resp.AccessToken); err == nil {
		t.Fatal("a token with no subject must not yield userinfo")
	}
}

func TestRevokeOnlyAffectsTheOwningClient(t *testing.T) {
	c := confidentialClient(t)
	s, flow := tokenService(t, c)
	ctx := context.Background()
	code, verifier := issueCode(t, s, flow, "openid offline_access")

	resp, err := s.Token(ctx, codeRequest(code, verifier))
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	// Өөр client-ийн нэрийн өмнөөс цуцлах оролдлого нөлөөлөх ёсгүй.
	other := c
	other.ClientID = "someone-else"
	if err := s.Revoke(ctx, other, resp.AccessToken, "access_token"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if !s.Introspect(ctx, "", resp.AccessToken).Active {
		t.Fatal("another client must not be able to revoke this token")
	}

	// Эзэн client цуцлахад хүчингүй болно.
	if err := s.Revoke(ctx, c, resp.AccessToken, "access_token"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if s.Introspect(ctx, "", resp.AccessToken).Active {
		t.Fatal("the owning client's revocation must take effect")
	}
}

// RFC 7009 §2.2 — танигдаагүй token ч алдаа биш.
func TestRevokeUnknownTokenSucceeds(t *testing.T) {
	c := confidentialClient(t)
	s, _ := tokenService(t, c)
	if err := s.Revoke(context.Background(), c, "never-existed", ""); err != nil {
		t.Fatalf("revoking an unknown token must not error: %v", err)
	}
}

// Өөр client-ийн token-ыг шалгах боломжгүй — эс бөгөөс token-ыг хаанаас нэгээс
// олсон хэн ч эзний тогтвортой `sub`-ыг мэдэх болно (RFC 7662 §2.1).
func TestIntrospectIsScopedToTheCallingClient(t *testing.T) {
	s, flow := tokenService(t, confidentialClient(t))
	ctx := context.Background()
	code, verifier := issueCode(t, s, flow, "openid")

	resp, err := s.Token(ctx, codeRequest(code, verifier))
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	if got := s.Introspect(ctx, "ring-dgov-mn", resp.AccessToken); !got.Active {
		t.Fatal("the owning client must be able to introspect its own token")
	}

	other := s.Introspect(ctx, "some-other-app", resp.AccessToken)
	if other.Active || other.Subject != "" || other.ClientID != "" || other.Scope != "" {
		t.Fatalf("another client learned something about this token: %+v", other)
	}

	// Дотоод дуудагч (bearer middleware) caller хоосноор бүрэн хариу авна.
	if got := s.Introspect(ctx, "", resp.AccessToken); !got.Active || got.Subject != testSubject {
		t.Fatalf("the internal caller should still resolve the token: %+v", got)
	}
}

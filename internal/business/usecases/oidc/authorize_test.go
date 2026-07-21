// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
)

// ── тестийн хуурамч хадгалалт ────────────────────────────────────────────────

type fakeClients struct{ c domain.OAuthClient }

func (f *fakeClients) Get(_ context.Context, id string) (domain.OAuthClient, error) {
	if f.c.ClientID != id {
		return domain.OAuthClient{}, apperror.NotFound("application not found")
	}
	return f.c, nil
}

type fakeFlow struct {
	challenges map[string]domain.OAuthChallenge
	consents   map[string][]string
	codes      []domain.OAuthAuthCode
}

func newFakeFlow() *fakeFlow {
	return &fakeFlow{challenges: map[string]domain.OAuthChallenge{}, consents: map[string][]string{}}
}

func (f *fakeFlow) CreateChallenge(_ context.Context, c domain.OAuthChallenge) error {
	f.challenges[c.Kind+":"+c.Challenge] = c
	return nil
}

func (f *fakeFlow) Challenge(_ context.Context, kind, ch string) (domain.OAuthChallenge, error) {
	c, ok := f.challenges[kind+":"+ch]
	if !ok || c.DecidedAt != nil || time.Now().After(c.ExpiresAt) {
		return domain.OAuthChallenge{}, apperror.NotFound("challenge not found or already used")
	}
	return c, nil
}

func (f *fakeFlow) DecideChallenge(_ context.Context, ch, subject string, granted []string) error {
	for k, c := range f.challenges {
		if c.Challenge != ch {
			continue
		}
		if c.DecidedAt != nil {
			return apperror.NotFound("challenge not found or already used")
		}
		now := time.Now()
		c.DecidedAt = &now
		if subject != "" {
			c.Subject = subject
		}
		c.GrantedScopes = granted
		f.challenges[k] = c
		return nil
	}
	return apperror.NotFound("challenge not found or already used")
}

func (f *fakeFlow) Consent(_ context.Context, subject, clientID string) ([]string, error) {
	return f.consents[subject+"|"+clientID], nil
}

func (f *fakeFlow) SaveConsent(_ context.Context, subject, clientID string, scopes []string, _ time.Duration) error {
	f.consents[subject+"|"+clientID] = scopes
	return nil
}

func (f *fakeFlow) RevokeConsent(_ context.Context, subject, clientID string) error {
	delete(f.consents, subject+"|"+clientID)
	return nil
}

func (f *fakeFlow) CreateCode(_ context.Context, c domain.OAuthAuthCode) error {
	f.codes = append(f.codes, c)
	return nil
}

// Доорх token-ийн методуудыг authorize-ийн тестүүд дуудахгүй — интерфейсийг
// хангахын тулд л байна (token урсгалыг token_test.go-гийн tokenFlow тестлэнэ).
func (f *fakeFlow) ConsumeCode(context.Context, []byte) (domain.OAuthAuthCode, bool, error) {
	return domain.OAuthAuthCode{}, false, apperror.NotFound("authorization code not found")
}

func (f *fakeFlow) StoreTokens(context.Context, domain.OAuthAccessToken, *domain.OAuthRefreshToken) error {
	return nil
}

func (f *fakeFlow) ConsumeRefreshToken(context.Context, []byte) (domain.OAuthRefreshToken, bool, error) {
	return domain.OAuthRefreshToken{}, false, apperror.NotFound("refresh token not found")
}

func (f *fakeFlow) RevokeFamily(context.Context, string) error { return nil }

func (f *fakeFlow) RevokeForSubjectClient(context.Context, string, string) error { return nil }

func (f *fakeFlow) AccessToken(context.Context, []byte) (domain.OAuthAccessToken, error) {
	return domain.OAuthAccessToken{}, apperror.NotFound("token not found")
}

func (f *fakeFlow) RevokeAccessToken(context.Context, []byte, string) (bool, error) {
	return false, nil
}

func (f *fakeFlow) RevokeRefreshToken(context.Context, []byte, string) (bool, error) {
	return false, nil
}

const testSubject = "11111111-1111-4111-8111-111111111111"

func webClient() domain.OAuthClient {
	return domain.OAuthClient{
		ClientID:                "ring-dgov-mn",
		ClientName:              "ring.dgov.mn",
		TokenEndpointAuthMethod: domain.AuthMethodBasic,
		GrantTypes:              []string{domain.GrantAuthorizationCode, domain.GrantRefreshToken},
		ResponseTypes:           []string{"code"},
		Scopes:                  []string{"openid", "profile", "email", "svc:eid-proxy"},
		RedirectURIs:            []string{"https://ring.dgov.mn/sso/callback"},
		Enabled:                 true,
	}
}

func newService(c domain.OAuthClient) (*Service, *fakeFlow) {
	flow := newFakeFlow()
	return NewService(&fakeClients{c: c}, flow, "https://sso.dgov.mn"), flow
}

func validRequest() AuthorizeRequest {
	return AuthorizeRequest{
		ClientID:     "ring-dgov-mn",
		RedirectURI:  "https://ring.dgov.mn/sso/callback",
		ResponseType: "code",
		Scope:        "openid profile email",
		State:        "xyz",
	}
}

func authErr(t *testing.T, err error) *AuthorizeError {
	t.Helper()
	var e *AuthorizeError
	if !errors.As(err, &e) {
		t.Fatalf("expected an *AuthorizeError, got %T (%v)", err, err)
	}
	return e
}

// ── тестүүд ──────────────────────────────────────────────────────────────────

func TestAuthorizeHappyPath(t *testing.T) {
	s, flow := newService(webClient())
	ch, client, err := s.Authorize(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if ch == "" {
		t.Fatal("no challenge issued")
	}
	if client.ClientID != "ring-dgov-mn" {
		t.Fatalf("client = %q", client.ClientID)
	}
	stored := flow.challenges["login:"+ch]
	if stored.State != "xyz" || stored.RedirectURI != "https://ring.dgov.mn/sso/callback" {
		t.Fatalf("challenge did not capture the request: %+v", stored)
	}
	if len(stored.RequestedScopes) != 3 {
		t.Fatalf("scopes = %v", stored.RequestedScopes)
	}
}

// Буруу client эсвэл redirect_uri үед алдааг RP руу БУЦААЖ БОЛОХГҮЙ — тэр хаяг
// баталгаажаагүй тул open redirect болно.
func TestAuthorizeNeverRedirectsOnUnverifiedTarget(t *testing.T) {
	s, _ := newService(webClient())

	t.Run("unknown client", func(t *testing.T) {
		req := validRequest()
		req.ClientID = "attacker-app"
		_, _, err := s.Authorize(context.Background(), req)
		if e := authErr(t, err); e.CanRedirect() {
			t.Fatal("an unknown client must not cause a redirect to the supplied URI")
		}
	})

	t.Run("unregistered redirect_uri", func(t *testing.T) {
		req := validRequest()
		req.RedirectURI = "https://evil.mn/steal"
		_, _, err := s.Authorize(context.Background(), req)
		if e := authErr(t, err); e.CanRedirect() {
			t.Fatal("an unregistered redirect_uri must not be redirected to")
		}
	})

	t.Run("empty redirect_uri", func(t *testing.T) {
		req := validRequest()
		req.RedirectURI = ""
		_, _, err := s.Authorize(context.Background(), req)
		if e := authErr(t, err); e.CanRedirect() {
			t.Fatal("an empty redirect_uri must be rejected outright")
		}
	})
}

func TestAuthorizeRejectsDisabledClient(t *testing.T) {
	c := webClient()
	c.Enabled = false
	s, _ := newService(c)
	_, _, err := s.Authorize(context.Background(), validRequest())
	if e := authErr(t, err); e.Code != "unauthorized_client" {
		t.Fatalf("error = %q, want unauthorized_client", e.Code)
	}
}

func TestAuthorizeRejectsNonCodeResponseTypes(t *testing.T) {
	s, _ := newService(webClient())
	for _, rt := range []string{"token", "id_token", "code token", ""} {
		req := validRequest()
		req.ResponseType = rt
		_, _, err := s.Authorize(context.Background(), req)
		if e := authErr(t, err); e.Code != "unsupported_response_type" {
			t.Fatalf("response_type=%q gave %q, want unsupported_response_type", rt, e.Code)
		}
	}
}

// PKCE нь public client-д ЗААВАЛ, бөгөөд `plain` арга хэзээ ч зөвшөөрөгдөхгүй.
func TestAuthorizePKCERules(t *testing.T) {
	pub := webClient()
	pub.TokenEndpointAuthMethod = domain.AuthMethodNone

	t.Run("public client without PKCE is rejected", func(t *testing.T) {
		s, _ := newService(pub)
		_, _, err := s.Authorize(context.Background(), validRequest())
		if e := authErr(t, err); e.Code != "invalid_request" {
			t.Fatalf("error = %q", e.Code)
		}
	})

	t.Run("plain method is rejected", func(t *testing.T) {
		s, _ := newService(pub)
		req := validRequest()
		req.CodeChallenge = "abc"
		req.CodeChallengeMethod = "plain"
		_, _, err := s.Authorize(context.Background(), req)
		if e := authErr(t, err); e.Code != "invalid_request" {
			t.Fatalf("plain PKCE must be refused, got %q", e.Code)
		}
	})

	t.Run("S256 is accepted", func(t *testing.T) {
		s, _ := newService(pub)
		req := validRequest()
		req.CodeChallenge = "abc"
		req.CodeChallengeMethod = "S256"
		if _, _, err := s.Authorize(context.Background(), req); err != nil {
			t.Fatalf("Authorize with S256: %v", err)
		}
	})
}

// Client-д олгогдоогүй scope хүсэхэд чимээгүй хаягдана; огт үлдэхгүй бол алдаа.
func TestAuthorizeCannotEscalateScope(t *testing.T) {
	s, flow := newService(webClient())

	req := validRequest()
	req.Scope = "openid svc:eid-org-proxy admin"
	ch, _, err := s.Authorize(context.Background(), req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	got := flow.challenges["login:"+ch].RequestedScopes
	if len(got) != 1 || got[0] != "openid" {
		t.Fatalf("scopes = %v; disallowed scopes must be dropped", got)
	}

	req.Scope = "admin svc:eid-org-proxy"
	if _, _, err := s.Authorize(context.Background(), req); err == nil {
		t.Fatal("a request with no allowed scope at all should fail")
	} else if e := authErr(t, err); e.Code != "invalid_scope" {
		t.Fatalf("error = %q, want invalid_scope", e.Code)
	}
}

func TestAcceptLoginSkipsOnlyWhenConsentCoversEverything(t *testing.T) {
	s, flow := newService(webClient())
	ctx := context.Background()

	ch, _, err := s.Authorize(ctx, validRequest())
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}

	// Хэсэгчилсэн зөвшөөрөл — алгасаж БОЛОХГҮЙ.
	flow.consents[testSubject+"|ring-dgov-mn"] = []string{"openid"}
	_, skip, err := s.AcceptLogin(ctx, ch, testSubject)
	if err != nil {
		t.Fatalf("AcceptLogin: %v", err)
	}
	if skip {
		t.Fatal("consent must not be skipped when the remembered grant is narrower than the request")
	}

	// Бүрэн зөвшөөрөл — алгасана.
	ch2, _, _ := s.Authorize(ctx, validRequest())
	flow.consents[testSubject+"|ring-dgov-mn"] = []string{"openid", "profile", "email"}
	_, skip, err = s.AcceptLogin(ctx, ch2, testSubject)
	if err != nil {
		t.Fatalf("AcceptLogin: %v", err)
	}
	if !skip {
		t.Fatal("a remembered grant covering every requested scope should skip the consent UI")
	}
}

func TestChallengeIsSingleUse(t *testing.T) {
	s, _ := newService(webClient())
	ctx := context.Background()

	ch, _, _ := s.Authorize(ctx, validRequest())
	if _, _, err := s.AcceptLogin(ctx, ch, testSubject); err != nil {
		t.Fatalf("first AcceptLogin: %v", err)
	}
	if _, _, err := s.AcceptLogin(ctx, ch, testSubject); err == nil {
		t.Fatal("a login challenge must not be usable twice")
	}
}

func TestAcceptConsentIssuesCodeAndRedirects(t *testing.T) {
	s, flow := newService(webClient())
	ctx := context.Background()

	ch, _, _ := s.Authorize(ctx, validRequest())
	consentCh, _, _ := s.AcceptLogin(ctx, ch, testSubject)

	redirect, err := s.AcceptConsent(ctx, consentCh, testSubject, []string{"openid", "profile"})
	if err != nil {
		t.Fatalf("AcceptConsent: %v", err)
	}

	u, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("redirect is not a URL: %v", err)
	}
	if u.Host != "ring.dgov.mn" || u.Path != "/sso/callback" {
		t.Fatalf("redirect went to %q", redirect)
	}
	code := u.Query().Get("code")
	if code == "" {
		t.Fatal("no code in the redirect")
	}
	if u.Query().Get("state") != "xyz" {
		t.Fatal("state must be echoed back to the RP")
	}

	if len(flow.codes) != 1 {
		t.Fatalf("expected one stored code, got %d", len(flow.codes))
	}
	stored := flow.codes[0]

	// Түүхий code хадгалагдах ЁСГҮЙ — зөвхөн sha256.
	want := sha256.Sum256([]byte(code))
	if string(stored.CodeHash) != string(want[:]) {
		t.Fatal("the stored code is not the sha256 of the issued code")
	}
	if strings.Contains(string(stored.CodeHash), code) {
		t.Fatal("the raw code leaked into storage")
	}
	if len(stored.Scopes) != 2 {
		t.Fatalf("granted scopes = %v, want the two the user approved", stored.Scopes)
	}
	if stored.ExpiresAt.Sub(stored.AuthTime) > 5*time.Minute {
		t.Fatal("authorization codes must be short-lived")
	}
}

// Өөр иргэний нээлттэй challenge-ыг өөрийн session-ээр дуусгах боломжгүй.
func TestAcceptConsentRejectsSubjectMismatch(t *testing.T) {
	s, _ := newService(webClient())
	ctx := context.Background()

	ch, _, _ := s.Authorize(ctx, validRequest())
	consentCh, _, _ := s.AcceptLogin(ctx, ch, testSubject)

	const attacker = "22222222-2222-4222-8222-222222222222"
	if _, err := s.AcceptConsent(ctx, consentCh, attacker, nil); err == nil {
		t.Fatal("a consent challenge must only be completable by the user it was issued for")
	}
}

func TestAcceptConsentCannotGrantMoreThanRequested(t *testing.T) {
	s, flow := newService(webClient())
	ctx := context.Background()

	req := validRequest()
	req.Scope = "openid"
	ch, _, _ := s.Authorize(ctx, req)
	consentCh, _, _ := s.AcceptLogin(ctx, ch, testSubject)

	// UI нь хүсээгүй scope-ыг "олгож" оролдоно.
	if _, err := s.AcceptConsent(ctx, consentCh, testSubject, []string{"openid", "svc:eid-proxy"}); err != nil {
		t.Fatalf("AcceptConsent: %v", err)
	}
	got := flow.codes[0].Scopes
	if len(got) != 1 || got[0] != "openid" {
		t.Fatalf("granted = %v; the grant must not exceed what was requested", got)
	}
}

func TestRejectRedirectsWithAccessDenied(t *testing.T) {
	s, _ := newService(webClient())
	ctx := context.Background()

	ch, _, _ := s.Authorize(ctx, validRequest())
	redirect, err := s.Reject(ctx, domain.ChallengeLogin, ch, "user cancelled")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	u, _ := url.Parse(redirect)
	if u.Query().Get("error") != "access_denied" {
		t.Fatalf("error = %q, want access_denied", u.Query().Get("error"))
	}
	if u.Query().Get("state") != "xyz" {
		t.Fatal("state must survive a rejection too")
	}
}

func TestVerifyPKCE(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !VerifyPKCE(challenge, "S256", verifier) {
		t.Fatal("the matching verifier must pass")
	}
	if VerifyPKCE(challenge, "S256", verifier+"x") {
		t.Fatal("a wrong verifier must fail")
	}
	if VerifyPKCE(challenge, "S256", "") {
		t.Fatal("an empty verifier must fail when a challenge was set")
	}
	if VerifyPKCE(challenge, "plain", verifier) {
		t.Fatal("plain must never be accepted")
	}
	// Challenge тавиагүй урсгалд verifier ирвэл татгалзана (заль мэх).
	if VerifyPKCE("", "S256", verifier) {
		t.Fatal("a verifier for a flow that had no challenge must not pass")
	}
	if !VerifyPKCE("", "", "") {
		t.Fatal("a flow that used no PKCE at all should pass")
	}
}

func TestRedirectWithPreservesExistingQuery(t *testing.T) {
	got := redirectWith("https://rp.mn/cb?tenant=ub", map[string]string{"code": "abc", "state": "s"})
	u, _ := url.Parse(got)
	q := u.Query()
	if q.Get("tenant") != "ub" {
		t.Fatalf("the RP's own query parameter was dropped: %q", got)
	}
	if q.Get("code") != "abc" || q.Get("state") != "s" {
		t.Fatalf("missing protocol parameters: %q", got)
	}
}

// redirect_uri баталгаажсаны ДАРААХ алдаанууд RP руу буцаж, state-аа авч явна.
func TestAuthorizeErrorsAfterValidationCarryTheValidatedRedirect(t *testing.T) {
	s, _ := newService(webClient())

	req := validRequest()
	req.ResponseType = "token"
	_, _, err := s.Authorize(context.Background(), req)
	e := authErr(t, err)

	if !e.CanRedirect() {
		t.Fatal("once redirect_uri is known-good the error should go back to the RP")
	}
	if e.RedirectURI != "https://ring.dgov.mn/sso/callback" {
		t.Fatalf("RedirectURI = %q; must be the registered value", e.RedirectURI)
	}
	u, _ := url.Parse(e.RedirectURL())
	if u.Query().Get("error") != "unsupported_response_type" {
		t.Fatalf("error param = %q", u.Query().Get("error"))
	}
	if u.Query().Get("state") != "xyz" {
		t.Fatal("state must be echoed on the error redirect")
	}
}

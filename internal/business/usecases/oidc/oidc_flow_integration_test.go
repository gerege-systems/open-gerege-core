//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Бүрэн authorize → consent → token → userinfo → refresh урсгалыг ЖИНХЭНЭ
// Postgres дээр, ЖИНХЭНЭ RLS-ийн доор (non-superuser app_user) ажиллуулна.
//
// ЯАГААД ЭНЭ ТЕСТ БАЙХ ЁСТОЙ: unit тестүүд нь санах-ойн хуурамч store ашигладаг
// тул RLS-ийг огт хөнддөггүй. Ингэснээр протоколын логик бүхэлдээ ногоон байтал
// production-д (болон compose-д) хүсэлт бүр RLS-д хаагдаж, provider огт
// ажиллахгүй байх боломжтой байсан — яг тийм алдаа гарч байсан.
package oidc_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"template/internal/business/domain"
	oidcuc "template/internal/business/usecases/oidc"
	usersuc "template/internal/business/usecases/users"
	oauthpg "template/internal/datasources/repositories/postgres/oauth"
	userspg "template/internal/datasources/repositories/postgres/users"
	"template/internal/test/testenv"
	"template/pkg/secrethash"
)

const (
	testClientID = "ring-dgov-mn"
	testSecret   = "integration-test-client-secret"
	testRedirect = "https://ring.dgov.mn/sso/callback"
	testIssuer   = "https://sso.dgov.mn"
)

// ЖИНХЭНЭ users usecase + repository-г ашиглана, stub БИШ.
//
// Эхний хувилбар нь stub ашигласан бөгөөд яг тэр нь production-ы алдааг нуусан:
// token endpoint нэвтрээгүй дуудагддаг тул context-д RLS identity байхгүй, улмаас
// RLS-тэй `users` хүснэгтээс уншихад "user not found" болж token гаргах бүрд 500
// буцдаг байв. Stub нь DB-д огт хүрдэггүй тул тестүүд ногоон хэвээр байсан.
// setup нь RLS хүчинтэй (non-superuser) pool дээр service-ийг угсарна.
func setup(t *testing.T) (*oidcuc.Service, string) {
	t.Helper()
	admin := testenv.StartPostgres(t)
	app := testenv.AppUserPool(t, admin)

	// Production-ийн initdb нь migrate-ийн үүсгэсэн бүх хүснэгтэд DML эрхийг
	// ALTER DEFAULT PRIVILEGES-ээр олгодог; харнес нь зөвхөн users-т олгодог тул
	// oauth_* хүснэгтүүдэд эрхийг гараар нэмнэ (RLS нь эрхийн ДЭЭР ажилладаг).
	for _, tbl := range []string{
		"oauth_clients", "oauth_signing_keys", "oauth_auth_codes",
		"oauth_access_tokens", "oauth_refresh_tokens", "oauth_challenges", "oauth_consents",
	} {
		_, err := admin.Exec(context.Background(),
			`GRANT SELECT, INSERT, UPDATE, DELETE ON `+tbl+` TO app_user`)
		require.NoError(t, err, "grant on %s", tbl)
	}

	subject := seedUser(t, admin)
	seedClient(t, app)

	flow := oauthpg.NewFlowRepository(app)
	clients := oauthpg.NewClientRepository(app)
	keys, err := oidcuc.NewKeyManager(oauthpg.NewKeyRepository(app), "integration-test-encryption-key")
	require.NoError(t, err)
	require.NoError(t, keys.EnsureKey(context.Background()))

	users := usersuc.NewUsecase(userspg.NewUserRepository(app), nil, usersuc.Config{})

	svc := oidcuc.NewService(clients, flow, testIssuer).
		WithTokenIssuing(keys, users)
	return svc, subject
}

func seedUser(t *testing.T, admin *pgxpool.Pool) string {
	t.Helper()
	// FK-д зориулж жинхэнэ хэрэглэгч мөр (superuser-ээр, RLS тойрч) — org_rls_test-
	// ийн адил хэв маяг.
	const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	_, err := admin.Exec(context.Background(),
		`INSERT INTO users(id, username, email, first_name, last_name, active, role_id, created_at)
		 VALUES ($1, 'oidc_test', 'bat@example.mn', 'Бат', 'Дорж', true, 4, now())`, id)
	require.NoError(t, err)
	return id
}

func seedClient(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	hash, err := secrethash.Hash(testSecret)
	require.NoError(t, err)
	_, err = oauthpg.NewClientRepository(pool).Create(context.Background(), domain.OAuthClient{
		ClientID:                testClientID,
		ClientName:              "ring.dgov.mn",
		SecretHash:              hash,
		TokenEndpointAuthMethod: domain.AuthMethodBasic,
		AppType:                 "web",
		GrantTypes:              []string{domain.GrantAuthorizationCode, domain.GrantRefreshToken},
		ResponseTypes:           []string{"code"},
		Scopes:                  []string{"openid", "profile", "email", "offline_access"},
		RedirectURIs:            []string{testRedirect},
		PostLogoutRedirectURIs:  []string{"https://ring.dgov.mn/"},
		Enabled:                 true,
	})
	require.NoError(t, err)
}

// Бүтэн урсгал: authorize → login accept → consent accept → code exchange →
// userinfo → refresh. Алхам бүр RLS-ийн доор жинхэнэ DB-д бичиж/уншина.
func TestFullAuthorizationCodeFlowUnderRLS(t *testing.T) {
	svc, subject := setup(t)
	ctx := context.Background()

	// PKCE: verifier → S256 challenge.
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challengeParam := oidcuc.S256Challenge(verifier)

	loginChallenge, client, err := svc.Authorize(ctx, oidcuc.AuthorizeRequest{
		ClientID:            testClientID,
		RedirectURI:         testRedirect,
		ResponseType:        "code",
		Scope:               "openid profile email offline_access",
		State:               "state-abc",
		Nonce:               "nonce-xyz",
		CodeChallenge:       challengeParam,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err, "authorize must work under RLS")
	require.NotEmpty(t, loginChallenge)
	require.Equal(t, testClientID, client.ClientID)

	consentChallenge, skip, err := svc.AcceptLogin(ctx, loginChallenge, subject)
	require.NoError(t, err)
	require.False(t, skip, "first time through there is no remembered consent")

	redirect, err := svc.AcceptConsent(ctx, consentChallenge, subject, nil)
	require.NoError(t, err)

	u, err := url.Parse(redirect)
	require.NoError(t, err)
	code := u.Query().Get("code")
	require.NotEmpty(t, code, "redirect must carry a code: %s", redirect)
	require.Equal(t, "state-abc", u.Query().Get("state"))

	tok, err := svc.Token(ctx, oidcuc.TokenRequest{
		GrantType: domain.GrantAuthorizationCode, Code: code,
		RedirectURI: testRedirect, CodeVerifier: verifier,
		ClientID: testClientID, ClientSecret: testSecret, SecretFromBasic: true,
	})
	require.NoError(t, err, "code exchange must work under RLS")
	require.NotEmpty(t, tok.AccessToken)
	require.NotEmpty(t, tok.RefreshToken, "offline_access was granted")
	require.NotEmpty(t, tok.IDToken, "openid was granted")

	// id_token нь энэ issuer, энэ client, энэ subject-д зориулагдсан байх ёстой.
	require.Equal(t, 3, len(strings.Split(tok.IDToken, ".")), "id_token must be a JWS")

	info := svc.Introspect(ctx, testClientID, tok.AccessToken)
	require.True(t, info.Active)
	require.Equal(t, subject, info.Subject)

	claims, err := svc.Userinfo(ctx, tok.AccessToken)
	require.NoError(t, err)
	require.Equal(t, subject, claims["sub"])
	// Claims нь ЖИНХЭНЭ users хүснэгтээс RLS-ийн доор уншигдана.
	require.Equal(t, "Дорж Бат", claims["name"], "profile claims must come from the real users table")
	require.Equal(t, "bat@example.mn", claims["email"])

	// Refresh нь эргэлт хийж, хуучныг хүчингүй болгоно.
	refreshed, err := svc.Token(ctx, oidcuc.TokenRequest{
		GrantType: domain.GrantRefreshToken, RefreshToken: tok.RefreshToken,
		ClientID: testClientID, ClientSecret: testSecret, SecretFromBasic: true,
	})
	require.NoError(t, err, "refresh must work under RLS")
	require.NotEqual(t, tok.RefreshToken, refreshed.RefreshToken, "refresh token must rotate")

	// Хуучин refresh-ийг дахин ашиглах → бүлэг цуцлагдана.
	_, err = svc.Token(ctx, oidcuc.TokenRequest{
		GrantType: domain.GrantRefreshToken, RefreshToken: tok.RefreshToken,
		ClientID: testClientID, ClientSecret: testSecret, SecretFromBasic: true,
	})
	require.Error(t, err, "a consumed refresh token must be refused")

	require.False(t, svc.Introspect(ctx, testClientID, refreshed.AccessToken).Active,
		"detecting refresh reuse must revoke the whole family, including the newest access token")
}

// Хоёр дахь удаагийн нэвтрэлт: санагдсан зөвшөөрөл хүссэн бүх scope-ыг хамарвал
// consent UI алгасагдана.
func TestRememberedConsentSkipsUnderRLS(t *testing.T) {
	svc, subject := setup(t)
	ctx := context.Background()

	run := func() (string, bool) {
		loginChallenge, _, err := svc.Authorize(ctx, oidcuc.AuthorizeRequest{
			ClientID: testClientID, RedirectURI: testRedirect, ResponseType: "code",
			Scope: "openid profile",
		})
		require.NoError(t, err)
		consentChallenge, skip, err := svc.AcceptLogin(ctx, loginChallenge, subject)
		require.NoError(t, err)
		return consentChallenge, skip
	}

	consent, skip := run()
	require.False(t, skip)
	_, err := svc.AcceptConsent(ctx, consent, subject, nil)
	require.NoError(t, err)

	_, skip = run()
	require.True(t, skip, "the remembered grant covers the request, so consent should be skipped")
}

// RP-үүд logout дээр `client_id` биш `id_token_hint` илгээдэг (OIDC RP-Initiated
// Logout §3 нь түүнийг зөвлөдөг) — production дээр яг үүнээс болж logout унасан.
// Тест нь ЖИНХЭНЭ гаргасан id_token-оор шалгана.
func TestLogoutAcceptsIDTokenHintUnderRLS(t *testing.T) {
	svc, subject := setup(t)
	ctx := context.Background()

	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	loginChallenge, _, err := svc.Authorize(ctx, oidcuc.AuthorizeRequest{
		ClientID: testClientID, RedirectURI: testRedirect, ResponseType: "code",
		Scope: "openid profile", CodeChallenge: oidcuc.S256Challenge(verifier), CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	consentChallenge, _, err := svc.AcceptLogin(ctx, loginChallenge, subject)
	require.NoError(t, err)
	redirect, err := svc.AcceptConsent(ctx, consentChallenge, subject, nil)
	require.NoError(t, err)
	u, _ := url.Parse(redirect)

	tok, err := svc.Token(ctx, oidcuc.TokenRequest{
		GrantType: domain.GrantAuthorizationCode, Code: u.Query().Get("code"),
		RedirectURI: testRedirect, CodeVerifier: verifier,
		ClientID: testClientID, ClientSecret: testSecret, SecretFromBasic: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, tok.IDToken)

	t.Run("hint alone identifies the client", func(t *testing.T) {
		ch, err := svc.StartLogout(ctx, "", tok.IDToken, "https://ring.dgov.mn/", "s")
		require.NoError(t, err, "an id_token_hint must be accepted without client_id")
		back, err := svc.AcceptLogout(ctx, ch)
		require.NoError(t, err)
		require.Contains(t, back, "https://ring.dgov.mn/")
	})

	t.Run("unregistered post_logout_redirect_uri is refused", func(t *testing.T) {
		_, err := svc.StartLogout(ctx, "", tok.IDToken, "https://evil.mn/", "s")
		require.Error(t, err, "the hint must not authorise an unregistered return address")
	})

	t.Run("a forged hint is refused", func(t *testing.T) {
		forged := tok.IDToken[:len(tok.IDToken)-6] + "AAAAAA"
		_, err := svc.StartLogout(ctx, "", forged, "https://ring.dgov.mn/", "s")
		require.Error(t, err, "a hint whose signature does not verify must be rejected")
	})

	t.Run("logout without any hint still works", func(t *testing.T) {
		ch, err := svc.StartLogout(ctx, "", "", "", "")
		require.NoError(t, err)
		back, err := svc.AcceptLogout(ctx, ch)
		require.NoError(t, err)
		require.Equal(t, testIssuer+"/", back, "with no return address, fall back to the issuer home page")
	})
}

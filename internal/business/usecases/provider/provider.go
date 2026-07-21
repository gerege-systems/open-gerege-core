// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package provider нь sso.dgov.mn-ийг OIDC provider болгосон login/consent/
// logout цөм. `/oauth2/auth` нь browser-ыг энд (нэвтрэх/зөвшөөрөх хуудас руу)
// challenge-тэй чиглүүлдэг; энэ usecase нь challenge-ыг уншиж, иргэнийг
// платформын ОДОО БАЙГАА eID нэвтрэлтээр (session) баталгаажуулж, subject-ээр
// user ID-г тэмдэглэнэ.
//
// Өмнө нь challenge-уудыг Ory Hydra эзэмшдэг байсан. Одоо usecases/oidc
// эзэмшинэ; энэ багц нь HTTP/UI-д тохирсон нимгэн бүрхүүл хэвээр үлдсэн тул
// frontend (`/oauth/*`, `/api/provider/*`) огт өөрчлөгдөөгүй.
package provider

import (
	"context"
	"net/url"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	oidcuc "template/internal/business/usecases/oidc"
	usersuc "template/internal/business/usecases/users"
)

// UserLookup нь subject (user ID)-ээр иргэний record-ыг авах минимал хараат
// байдал (users usecase үүнийг хангана).
type UserLookup interface {
	GetByID(ctx context.Context, req usersuc.GetByIDRequest) (usersuc.GetByIDResponse, error)
}

// ClientLookup нь challenge дээрх client_id-аас апп-ийн мэдээллийг авна.
type ClientLookup interface {
	Get(ctx context.Context, clientID string) (domain.OAuthClient, error)
}

// LoginInfo нь login хуудсанд харуулах login хүсэлтийн товч.
type LoginInfo struct {
	Challenge      string
	ClientID       string
	ClientName     string
	RequestedScope []string
	Subject        string
	// Skip нь дахин eID шаардахгүй гэдгийг илэрхийлнэ. Бидний загварт нэвтрэлт
	// нь платформын session тул энэ нь үргэлж false — session байвал frontend
	// шууд accept руу шилждэг.
	Skip bool
}

// ConsentInfo нь consent хуудсанд харуулах зөвшөөрлийн хүсэлтийн товч.
type ConsentInfo struct {
	Challenge      string
	ClientID       string
	ClientName     string
	Subject        string
	RequestedScope []string
	// Skip нь consent UI-г алгасах эсэх (first-party апп эсвэл өмнө нь санагдсан
	// зөвшөөрөл хүссэн бүх scope-ыг хамарсан).
	Skip bool
}

// Usecase нь OIDC provider-ийн login/consent/logout зохицуулалт.
type Usecase interface {
	GetLogin(ctx context.Context, challenge string) (LoginInfo, error)
	AcceptLogin(ctx context.Context, userID, challenge string) (redirectTo string, err error)
	RejectLogin(ctx context.Context, challenge, reason string) (redirectTo string, err error)
	// LoginAppContext нь login_challenge-аас нэвтэрч буй RP апп-ийн (rp_app нэр,
	// rp_app_url домэйн)-г буцаана — eID push-д дамжуулна. Base SSO / first-party /
	// хоосон/буруу challenge үед хоосон (нэвтрэлтийг блоклохгүй, fail-open).
	LoginAppContext(ctx context.Context, challenge string) (rpApp, rpAppURL string)
	GetConsent(ctx context.Context, challenge string) (ConsentInfo, error)
	AcceptConsent(ctx context.Context, userID, challenge string, grantScope []string) (redirectTo string, err error)
	RejectConsent(ctx context.Context, challenge, reason string) (redirectTo string, err error)
	AcceptLogout(ctx context.Context, challenge string) (redirectTo string, err error)
}

type usecase struct {
	oidc       *oidcuc.Service
	clients    ClientLookup
	users      UserLookup
	firstParty map[string]struct{}
	issuer     string
}

// NewUsecase нь OIDC service, client lookup, user lookup болон first-party
// client_id жагсаалтаас provider usecase үүсгэнэ.
func NewUsecase(svc *oidcuc.Service, clients ClientLookup, users UserLookup, firstPartyClients []string, issuer string) Usecase {
	fp := make(map[string]struct{}, len(firstPartyClients))
	for _, c := range firstPartyClients {
		fp[c] = struct{}{}
	}
	return &usecase{oidc: svc, clients: clients, users: users, firstParty: fp, issuer: strings.TrimRight(issuer, "/")}
}

func (u *usecase) GetLogin(ctx context.Context, challenge string) (LoginInfo, error) {
	if strings.TrimSpace(challenge) == "" {
		return LoginInfo{}, apperror.BadRequest("login_challenge шаардлагатай")
	}
	c, err := u.oidc.LoginChallenge(ctx, challenge)
	if err != nil {
		return LoginInfo{}, err
	}
	name, _ := u.clientDisplay(ctx, c.ClientID)
	return LoginInfo{
		Challenge:      challenge,
		ClientID:       c.ClientID,
		ClientName:     name,
		RequestedScope: c.RequestedScopes,
	}, nil
}

func (u *usecase) LoginAppContext(ctx context.Context, challenge string) (rpApp, rpAppURL string) {
	if strings.TrimSpace(challenge) == "" {
		return "", ""
	}
	c, err := u.oidc.LoginChallenge(ctx, challenge)
	if err != nil {
		return "", "" // resolve чадсангүй — base гэж үзэж хоосон (fail-open)
	}
	// First-party client (base SSO / өөрийн web) → rp_app хоосон: "SSO өөрөө".
	if _, ok := u.firstParty[c.ClientID]; ok {
		return "", ""
	}
	name, origin := u.clientDisplay(ctx, c.ClientID)
	return name, origin
}

// clientDisplay нь апп-ийн харагдах нэр болон эхний redirect origin-ыг буцаана.
func (u *usecase) clientDisplay(ctx context.Context, clientID string) (name, origin string) {
	c, err := u.clients.Get(ctx, clientID)
	if err != nil {
		return clientID, ""
	}
	name = strings.TrimSpace(c.ClientName)
	if name == "" {
		name = c.ClientID
	}
	return name, redirectOrigin(c.RedirectURIs)
}

// redirectOrigin нь эхний хүчинтэй redirect_uri-ийн origin (scheme://host)-г буцаана.
func redirectOrigin(uris []string) string {
	for _, u := range uris {
		p, err := url.Parse(strings.TrimSpace(u))
		if err == nil && p.Scheme != "" && p.Host != "" {
			return p.Scheme + "://" + p.Host
		}
	}
	return ""
}

func (u *usecase) AcceptLogin(ctx context.Context, userID, challenge string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("login_challenge шаардлагатай")
	}
	if userID == "" {
		return "", apperror.Unauthorized("нэвтрээгүй байна")
	}
	// subject нь платформын тогтвортой, opaque per-citizen танигч (user UUID).
	consentChallenge, _, err := u.oidc.AcceptLogin(ctx, challenge, userID)
	if err != nil {
		return "", err
	}
	// Browser-ыг зөвшөөрлийн хуудас руу (өмнө нь Hydra-аар дамждаг байсан).
	return u.issuer + "/oauth/consent?consent_challenge=" + url.QueryEscape(consentChallenge), nil
}

func (u *usecase) RejectLogin(ctx context.Context, challenge, reason string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("login_challenge шаардлагатай")
	}
	if reason == "" {
		reason = "хэрэглэгч нэвтрэлтийг цуцлав"
	}
	return u.oidc.Reject(ctx, domain.ChallengeLogin, challenge, reason)
}

func (u *usecase) GetConsent(ctx context.Context, challenge string) (ConsentInfo, error) {
	if strings.TrimSpace(challenge) == "" {
		return ConsentInfo{}, apperror.BadRequest("consent_challenge шаардлагатай")
	}
	c, err := u.oidc.ConsentChallenge(ctx, challenge)
	if err != nil {
		return ConsentInfo{}, err
	}
	name, _ := u.clientDisplay(ctx, c.ClientID)
	_, firstParty := u.firstParty[c.ClientID]
	return ConsentInfo{
		Challenge:      challenge,
		ClientID:       c.ClientID,
		ClientName:     name,
		Subject:        c.Subject,
		RequestedScope: c.RequestedScopes,
		Skip:           firstParty || c.Skip,
	}, nil
}

func (u *usecase) AcceptConsent(ctx context.Context, userID, challenge string, grantScope []string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("consent_challenge шаардлагатай")
	}
	if userID == "" {
		return "", apperror.Forbidden("нэвтрээгүй байна")
	}
	// Иргэний бүртгэл байгааг ЭНД шалгана (fail-closed). Claims нь token
	// endpoint дээр, тухайн үеийн бодит өгөгдлөөр угсрагдана.
	if _, err := u.users.GetByID(ctx, usersuc.GetByIDRequest{ID: userID}); err != nil {
		return "", apperror.InternalCause(err)
	}
	return u.oidc.AcceptConsent(ctx, challenge, userID, grantScope)
}

func (u *usecase) RejectConsent(ctx context.Context, challenge, reason string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("consent_challenge шаардлагатай")
	}
	if reason == "" {
		reason = "хэрэглэгч зөвшөөрлийг цуцлав"
	}
	return u.oidc.Reject(ctx, domain.ChallengeConsent, challenge, reason)
}

func (u *usecase) AcceptLogout(ctx context.Context, challenge string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("logout_challenge шаардлагатай")
	}
	return u.oidc.AcceptLogout(ctx, challenge)
}

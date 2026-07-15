// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package provider нь dan.dgov.mn-ийг OIDC provider болгосон login/consent/
// logout цөм. Ory Hydra нь /oauth2/auth дээр browser-ыг энд (dan-ийн login/
// consent хуудас) чиглүүлж challenge өгдөг; энэ usecase нь challenge-ыг Hydra-
// аас авч, иргэнийг dan-ийн ЕОДОО БАЙГАА eID нэвтрэлтээр (session) баталгаажуулж,
// subject-аар dan-ийн user ID-г Hydra-д accept хийнэ. Consent дээр scope-оос
// хамааран иргэний claims-ыг (name/email/national_id...) id_token/access_token-д
// байрлуулна. sso-dgov-mn-ий internal/{auth,consent}-ийн provider логикийг dan-
// ийн identity model дээр дахин хэрэгжүүлэв.
package provider

import (
	"context"
	"fmt"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	usersuc "template/internal/business/usecases/users"
	"template/pkg/hydra"
)

// UserLookup нь subject (dan user ID)-ээр иргэний record-ыг авах минимал хараат
// байдал (users usecase үүнийг хангана).
type UserLookup interface {
	GetByID(ctx context.Context, req usersuc.GetByIDRequest) (usersuc.GetByIDResponse, error)
}

// LoginInfo нь login хуудсанд харуулах Hydra login request-ийн товч.
type LoginInfo struct {
	Challenge      string
	ClientID       string
	ClientName     string
	RequestedScope []string
	Subject        string
	// Skip нь Hydra аль хэдийн энэ subject-ийн session-тай (дахин eID шаардахгүй)
	// гэдгийг илэрхийлнэ.
	Skip bool
}

// ConsentInfo нь consent хуудсанд харуулах Hydra consent request-ийн товч.
type ConsentInfo struct {
	Challenge      string
	ClientID       string
	ClientName     string
	Subject        string
	RequestedScope []string
	// Skip нь consent UI-г алгасах эсэх (first-party апп эсвэл Hydra-ийн санасан
	// grant).
	Skip bool
}

// Usecase нь OIDC provider-ийн login/consent/logout зохицуулалт.
type Usecase interface {
	GetLogin(ctx context.Context, challenge string) (LoginInfo, error)
	AcceptLogin(ctx context.Context, userID, challenge string) (redirectTo string, err error)
	RejectLogin(ctx context.Context, challenge, reason string) (redirectTo string, err error)
	GetConsent(ctx context.Context, challenge string) (ConsentInfo, error)
	AcceptConsent(ctx context.Context, userID, challenge string, grantScope []string) (redirectTo string, err error)
	RejectConsent(ctx context.Context, challenge, reason string) (redirectTo string, err error)
	AcceptLogout(ctx context.Context, challenge string) (redirectTo string, err error)
}

type usecase struct {
	hydra      *hydra.Admin
	users      UserLookup
	firstParty map[string]struct{}
}

// NewUsecase нь Hydra admin client, user lookup, first-party client_id жагсаалтаас
// provider usecase үүсгэнэ.
func NewUsecase(h *hydra.Admin, users UserLookup, firstPartyClients []string) Usecase {
	fp := make(map[string]struct{}, len(firstPartyClients))
	for _, c := range firstPartyClients {
		fp[c] = struct{}{}
	}
	return &usecase{hydra: h, users: users, firstParty: fp}
}

const rememberFor = 3600 // session-ийг санах хугацаа (секунд)

func (u *usecase) GetLogin(ctx context.Context, challenge string) (LoginInfo, error) {
	if challenge == "" {
		return LoginInfo{}, apperror.BadRequest("login_challenge шаардлагатай")
	}
	req, err := u.hydra.GetLoginRequest(ctx, challenge)
	if err != nil {
		return LoginInfo{}, apperror.InternalCause(fmt.Errorf("hydra login request: %w", err))
	}
	return LoginInfo{
		Challenge:      challenge,
		ClientID:       req.Client.ClientID,
		ClientName:     req.Client.ClientName,
		RequestedScope: req.RequestedScope,
		Subject:        req.Subject,
		Skip:           req.Skip,
	}, nil
}

func (u *usecase) AcceptLogin(ctx context.Context, userID, challenge string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("login_challenge шаардлагатай")
	}
	if userID == "" {
		return "", apperror.Unauthorized("нэвтрээгүй байна")
	}
	// subject нь dan-ийн тогтвортой, opaque per-citizen танигч (user UUID). eID-
	// ээр баталгаажсан тул ACR/AMR-д "eid"-г тэмдэглэнэ.
	redirect, err := u.hydra.AcceptLogin(ctx, challenge, hydra.LoginAccept{
		Subject:     userID,
		Remember:    true,
		RememberFor: rememberFor,
		ACR:         "eid",
		AMR:         []string{"eid"},
	})
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("hydra accept login: %w", err))
	}
	return redirect, nil
}

func (u *usecase) RejectLogin(ctx context.Context, challenge, reason string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("login_challenge шаардлагатай")
	}
	if reason == "" {
		reason = "хэрэглэгч нэвтрэлтийг цуцлав"
	}
	redirect, err := u.hydra.RejectLogin(ctx, challenge, "access_denied", reason)
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("hydra reject login: %w", err))
	}
	return redirect, nil
}

func (u *usecase) GetConsent(ctx context.Context, challenge string) (ConsentInfo, error) {
	if challenge == "" {
		return ConsentInfo{}, apperror.BadRequest("consent_challenge шаардлагатай")
	}
	req, err := u.hydra.GetConsentRequest(ctx, challenge)
	if err != nil {
		return ConsentInfo{}, apperror.InternalCause(fmt.Errorf("hydra consent request: %w", err))
	}
	_, firstParty := u.firstParty[req.Client.ClientID]
	return ConsentInfo{
		Challenge:      challenge,
		ClientID:       req.Client.ClientID,
		ClientName:     req.Client.ClientName,
		Subject:        req.Subject,
		RequestedScope: req.RequestedScope,
		Skip:           firstParty || req.Skip,
	}, nil
}

func (u *usecase) AcceptConsent(ctx context.Context, userID, challenge string, grantScope []string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("consent_challenge шаардлагатай")
	}
	req, err := u.hydra.GetConsentRequest(ctx, challenge)
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("hydra consent request: %w", err))
	}
	// Нэвтэрсэн иргэн consent-ийн subject-тай ЗААВАЛ таарна — өөр иргэний нэрийн
	// өмнөөс consent өгөх боломжгүй.
	if userID == "" || req.Subject != userID {
		return "", apperror.Forbidden("consent subject нэвтэрсэн хэрэглэгчтэй таарахгүй")
	}
	// Хүссэнээс илүү scope олгохгүй: grantScope-ыг requested-ээр хязгаарлана.
	granted := intersect(req.RequestedScope, grantScope)
	if len(grantScope) == 0 {
		granted = req.RequestedScope
	}

	var idClaims, atClaims map[string]any
	if resp, uerr := u.users.GetByID(ctx, usersuc.GetByIDRequest{ID: userID}); uerr == nil {
		idClaims, atClaims = claimsForScopes(granted, resp.User)
	}

	_, firstParty := u.firstParty[req.Client.ClientID]
	redirect, err := u.hydra.AcceptConsent(ctx, challenge, hydra.ConsentAccept{
		GrantScope: granted,
		Session: hydra.ConsentSession{
			IDToken:     idClaims,
			AccessToken: atClaims,
		},
		Remember:    firstParty, // first-party апп-д дахин асуухгүй
		RememberFor: rememberFor,
	})
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("hydra accept consent: %w", err))
	}
	return redirect, nil
}

func (u *usecase) RejectConsent(ctx context.Context, challenge, reason string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("consent_challenge шаардлагатай")
	}
	if reason == "" {
		reason = "хэрэглэгч зөвшөөрлийг цуцлав"
	}
	redirect, err := u.hydra.RejectConsent(ctx, challenge, "access_denied", reason)
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("hydra reject consent: %w", err))
	}
	return redirect, nil
}

func (u *usecase) AcceptLogout(ctx context.Context, challenge string) (string, error) {
	if challenge == "" {
		return "", apperror.BadRequest("logout_challenge шаардлагатай")
	}
	redirect, err := u.hydra.AcceptLogout(ctx, challenge)
	if err != nil {
		return "", apperror.InternalCause(fmt.Errorf("hydra accept logout: %w", err))
	}
	return redirect, nil
}

// claimsForScopes нь олгосон scope-оос хамааран (id_token, access_token)-ий
// claims-ыг dan-ийн User record-оос гаргана. `sub`-ыг ЭНД тавихгүй — Hydra
// login subject-ээс тавина.
func claimsForScopes(scopes []string, u domain.User) (idToken, accessToken map[string]any) {
	idToken = map[string]any{}
	accessToken = map[string]any{}
	for _, s := range scopes {
		switch s {
		case "profile":
			setIfNonEmpty(idToken, "name", u.FullName())
			setIfNonEmpty(idToken, "given_name", u.FirstName)
			setIfNonEmpty(idToken, "family_name", u.LastName)
			setIfNonEmpty(idToken, "given_name_en", u.FirstNameEn)
			setIfNonEmpty(idToken, "family_name_en", u.LastNameEn)
		case "email":
			setIfNonEmpty(idToken, "email", u.Email)
			if u.Email != "" {
				idToken["email_verified"] = true
			}
		case "nationalid":
			setIfNonEmpty(idToken, "national_id", u.NationalID)
			setIfNonEmpty(idToken, "register_number", u.CivilID)
		}
	}
	return idToken, accessToken
}

func setIfNonEmpty(m map[string]any, k, v string) {
	if strings.TrimSpace(v) != "" {
		m[k] = v
	}
}

// intersect нь want доторх, allow-д мөн байгаа утгуудыг (дараалал хадгалж) буцаана.
func intersect(allow, want []string) []string {
	set := make(map[string]struct{}, len(allow))
	for _, a := range allow {
		set[a] = struct{}{}
	}
	out := make([]string, 0, len(want))
	for _, w := range want {
		if _, ok := set[w]; ok {
			out = append(out, w)
		}
	}
	return out
}

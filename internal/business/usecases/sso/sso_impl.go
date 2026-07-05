// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package sso

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/caches"
	"template/pkg/jwt"
	"template/pkg/oidc"
)

// stateTTL нь SSO authorize-callback хоорондын state-ийн амьдрах хугацаа.
const stateTTL = 10 * time.Minute

// statePrefix нь Redis дахь нэг удаагийн state (CSRF) түлхүүрийн угтвар.
const statePrefix = "sso:state:"

// idtPrefix нь logout ref → id_token хадгалуурын угтвар. logoutTTL нь session-ий
// амьдрах хугацаатай ойролцоо (гарах хүртэл logout ажиллана).
const idtPrefix = "sso:idt:"
const logoutTTL = 7 * 24 * time.Hour

type usecase struct {
	oidc           *oidc.Client
	store          UserStore
	jwt            jwt.JWTService
	redis          caches.RedisCache
	nativeClientID string
}

// NewUsecase нь SSO usecase угсарна. nativeClientID нь mobile (PKCE, public
// client) урсгалын Hydra client_id (жишээ template-gerege-mn-ios) — хоосон бол
// native code-exchange идэвхгүй.
func NewUsecase(oidcClient *oidc.Client, store UserStore, jwtSvc jwt.JWTService, redis caches.RedisCache, nativeClientID string) Usecase {
	return &usecase{oidc: oidcClient, store: store, jwt: jwtSvc, redis: redis, nativeClientID: nativeClientID}
}

func (u *usecase) Configured() bool { return u.oidc.Configured() }

func (u *usecase) Start(ctx context.Context) (StartResponse, error) {
	if !u.oidc.Configured() {
		return StartResponse{}, apperror.InternalCause(fmt.Errorf("sso client not configured"))
	}
	state, err := randomToken()
	if err != nil {
		return StartResponse{}, apperror.InternalCause(err)
	}
	nonce, err := randomToken()
	if err != nil {
		return StartResponse{}, apperror.InternalCause(err)
	}
	// State-ийг Redis-д нэг удаагийн (callback дээр GetDel) хэлбэрээр хадгална —
	// callback-ийн CSRF/replay хамгаалалт.
	if err := u.redis.Set(ctx, statePrefix+state, nonce); err != nil {
		return StartResponse{}, apperror.InternalCause(fmt.Errorf("store sso state: %w", err))
	}
	_ = u.redis.Expire(ctx, statePrefix+state, stateTTL)

	return StartResponse{AuthURL: u.oidc.AuthCodeURL(state, nonce)}, nil
}

func (u *usecase) Complete(ctx context.Context, state, code string) (CompleteResponse, error) {
	if !u.oidc.Configured() {
		return CompleteResponse{}, apperror.InternalCause(fmt.Errorf("sso client not configured"))
	}
	if strings.TrimSpace(state) == "" || strings.TrimSpace(code) == "" {
		return CompleteResponse{}, apperror.BadRequest("SSO callback дутуу параметртэй байна")
	}
	// State-ийг нэг удаа шалгаж устгана — байхгүй бол хугацаа дууссан/хуурамч.
	if consumed, err := u.redis.GetDel(ctx, statePrefix+state); err != nil || consumed == "" {
		return CompleteResponse{}, apperror.BadRequest("SSO нэвтрэлтийн хугацаа дууссан эсвэл хүчингүй байна. Дахин оролдоно уу.")
	}

	// Code → access token + id token (client_secret_basic), дараа нь shared tail.
	accessToken, idToken, err := u.oidc.Exchange(ctx, code)
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(err)
	}
	return u.finish(ctx, accessToken, idToken)
}

// CompleteNative нь mobile (PKCE, public client) урсгалын authorization code-ийг
// солино. Web-ийн state/CSRF шалгалт БАЙХГҮЙ — native дээр PKCE (code_verifier)
// нь replay/interception хамгаалалтыг хангана. code-ийг public client-ээр
// (client_id form-д, client_secret-гүй) солиод web-ийн адил finish tail-ийг
// (userinfo → upsert → JWT хос) дуудна.
func (u *usecase) CompleteNative(ctx context.Context, code, codeVerifier, redirectURI string) (CompleteResponse, error) {
	if strings.TrimSpace(u.nativeClientID) == "" {
		return CompleteResponse{}, apperror.InternalCause(fmt.Errorf("sso native client not configured"))
	}
	if strings.TrimSpace(code) == "" || strings.TrimSpace(codeVerifier) == "" {
		return CompleteResponse{}, apperror.BadRequest("SSO native нэвтрэлт дутуу параметртэй байна")
	}
	accessToken, idToken, err := u.oidc.ExchangePKCE(ctx, u.nativeClientID, code, codeVerifier, redirectURI)
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(err)
	}
	return u.finish(ctx, accessToken, idToken)
}

// finish нь access/id token авсны дараах нийтлэг tail — web (Complete) болон
// native (CompleteNative) хоёулаа хуваалцана: /userinfo → нэр/иргэний дугаар →
// upsert → JWT хос → refresh санах → id_token ref → CompleteResponse.
func (u *usecase) finish(ctx context.Context, accessToken, idToken string) (CompleteResponse, error) {
	info, err := u.oidc.UserInfo(ctx, accessToken)
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(err)
	}

	firstName := strings.TrimSpace(info.GivenName)
	lastName := strings.TrimSpace(info.FamilyName)
	// given/family хоосон ч name байвал бүтэн нэрийг LastName-д (fallback) тавина.
	if firstName == "" && lastName == "" && strings.TrimSpace(info.Name) != "" {
		lastName = strings.TrimSpace(info.Name)
	}

	// nationalid scope-оос иргэний дугаар (register_number = civil id) ирсэн бол
	// байгаа eID хэрэглэгчтэй civil_id-ээр тааруулна — ижил регистрээр eID болон
	// SSO-ээр нэвтрэхэд НЭГ данс болно (давхардал үүсэхгүй). national_id (регно)
	// нь eID-ийн адил жижиг үсгээр хадгалагдана.
	civilID := strings.TrimSpace(info.RegisterNumber)
	nationalID := strings.ToLower(strings.TrimSpace(info.NationalID))

	var stored domain.User
	if civilID != "" {
		user := &domain.User{
			Username:  "eid_" + civilID,
			FirstName: firstName,
			LastName:  lastName,
			RoleID:    domain.RoleUser, // зөвхөн ШИНЭ мөрд; байгаа хэрэглэгчийн эрхийг хөндөхгүй
		}
		stored, err = u.store.UpsertByCivilID(ctx, civilID, nationalID, info.Sub, user)
	} else {
		// Иргэний дугааргүй (nationalid scope байхгүй/буцаагаагүй) — pairwise
		// sub-ээр. Refresh нь email-ээр хайдаг тул синтетик email хадгална.
		slug := subSlug(info.Sub)
		user := &domain.User{
			Username:  "sso_" + slug,
			FirstName: firstName,
			LastName:  lastName,
			Email:     "sso_" + slug + "@sso.local",
			Active:    true,
			RoleID:    domain.RoleUser,
		}
		stored, err = u.store.UpsertBySSOSub(ctx, info.Sub, user)
	}
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(fmt.Errorf("upsert sso user: %w", err))
	}

	pair, err := u.jwt.GenerateTokenPair(stored.ID, stored.IsAdmin(), stored.RoleID, stored.Email)
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(fmt.Errorf("generate token: %w", err))
	}
	if err := u.rememberRefresh(ctx, pair); err != nil {
		return CompleteResponse{}, apperror.InternalCause(fmt.Errorf("persist refresh: %w", err))
	}

	// id_token-ыг богино ref-ээр Redis-д хадгална — гарах үед ref-ээр logout URL
	// (id_token_hint-тэй) байгуулна. Cookie-д зөвхөн ref (32 hex) л очно.
	var logoutRef string
	if idToken != "" {
		if ref, rErr := randomToken(); rErr == nil {
			if setErr := u.redis.Set(ctx, idtPrefix+ref, idToken); setErr == nil {
				_ = u.redis.Expire(ctx, idtPrefix+ref, logoutTTL)
				logoutRef = ref
			}
		}
	}

	return CompleteResponse{
		Token:        pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		LogoutRef:    logoutRef,
		User:         stored,
	}, nil
}

// LogoutURL нь logout ref-ээр Redis-ээс id_token-ыг GetDel-ээр авч, RP-initiated
// logout URL байгуулна. ref байхгүй/хугацаа дууссан бол хоосон (SSO-гүй/аль
// хэдийн гарсан) буцаана.
func (u *usecase) LogoutURL(ctx context.Context, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", nil
	}
	idToken, err := u.redis.GetDel(ctx, idtPrefix+ref)
	if err != nil || idToken == "" {
		return "", nil
	}
	return u.oidc.LogoutURLFor(idToken), nil
}

// rememberRefresh нь refresh jti-г Redis-д TTL-тэй хадгална — auth_refresh-ийн
// RefreshKey (prefix "refresh:") форматтай нийцүүлж, refresh endpoint-ийг SSO
// хэрэглэгчид ч ажиллуулна.
func (u *usecase) rememberRefresh(ctx context.Context, pair jwt.TokenPair) error {
	ttl := time.Until(pair.RefreshExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("refresh token already expired")
	}
	key := "refresh:" + pair.RefreshJTI
	if err := u.redis.Set(ctx, key, pair.RefreshJTI); err != nil {
		return err
	}
	return u.redis.Expire(ctx, key, ttl)
}

// subSlug нь pairwise sub-ээс тогтвортой, богино (20 hex) слаг гаргана —
// username (≤25) ба email (≤50)-д таарна, тусгай тэмдэггүй.
func subSlug(sub string) string {
	h := sha256.Sum256([]byte(sub))
	return hex.EncodeToString(h[:])[:20]
}

// randomToken нь 32 hex тэмдэгтийн (16 байт) crypto/rand токен үүсгэнэ.
func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

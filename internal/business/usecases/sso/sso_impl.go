// Government Template Platform V3.0
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
	"template/pkg/logger"
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

// TokenStorer нь нэвтрэлтийн дараа иргэний SSO OAuth токенуудыг хадгална (SSO
// eID proxy-д зориулж). Хоосон/nil бол токен хадгалахгүй (proxy идэвхгүй).
// *ssotoken.Service үүнийг хангадаг.
type TokenStorer interface {
	Store(ctx context.Context, userID string, tok oidc.Tokens) error
}

type usecase struct {
	oidc           *oidc.Client
	store          UserStore
	jwt            jwt.JWTService
	redis          caches.RedisCache
	nativeClientID string
	tokens         TokenStorer      // сонголттой — nil бол SSO токен хадгалахгүй
	access         AccessModeReader // сонголттой — nil бол public гэж үзнэ
}

// NewUsecase нь SSO usecase угсарна. nativeClientID нь mobile (PKCE, public
// client) урсгалын client_id (жишээ template-dgov-mn-ios) — хоосон бол native
// code-exchange идэвхгүй. tokenStorer нь SSO eID proxy-д зориулж токен хадгалах
// (nil бол хадгалахгүй). accessMode нь платформын хандалтын горим уншигч (nil бол
// public — хэн ч нэвтэрч болно).
func NewUsecase(oidcClient *oidc.Client, store UserStore, jwtSvc jwt.JWTService, redis caches.RedisCache, nativeClientID string, tokenStorer TokenStorer, accessMode AccessModeReader) Usecase {
	return &usecase{oidc: oidcClient, store: store, jwt: jwtSvc, redis: redis, nativeClientID: nativeClientID, tokens: tokenStorer, access: accessMode}
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

	// Code → access/id/refresh token (client_secret_basic), дараа нь shared tail.
	// refresh_token (offline_access) нь SSO eID proxy-д зориулж хадгалагдана.
	tokens, err := u.oidc.ExchangeFull(ctx, code)
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(err)
	}
	return u.finish(ctx, tokens)
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
	// Native (PKCE) урсгал нь refresh_token хадгалахгүй — eID proxy нь web BFF-ээр
	// дуудагдана. Тиймээс Tokens зөвхөн access/id-тэй.
	return u.finish(ctx, oidc.Tokens{AccessToken: accessToken, IDToken: idToken})
}

// finish нь токен авсны дараах нийтлэг tail — web (Complete) болон native
// (CompleteNative) хоёулаа хуваалцана: /userinfo → нэр/иргэний дугаар → upsert →
// SSO токен хадгалах (proxy) → JWT хос → refresh санах → id_token ref →
// CompleteResponse.
func (u *usecase) finish(ctx context.Context, tokens oidc.Tokens) (CompleteResponse, error) {
	accessToken, idToken := tokens.AccessToken, tokens.IDToken
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
	// Латин нэр — SSO-ий given_name_en/family_name_en claim-аас (profile scope).
	firstNameEn := strings.TrimSpace(info.GivenNameEn)
	lastNameEn := strings.TrimSpace(info.FamilyNameEn)

	// nationalid scope-оос иргэний дугаар (register_number = civil id) ирсэн бол
	// байгаа eID хэрэглэгчтэй civil_id-ээр тааруулна — ижил регистрээр eID болон
	// SSO-ээр нэвтрэхэд НЭГ данс болно (давхардал үүсэхгүй). national_id (регно)
	// нь eID-ийн адил жижиг үсгээр хадгалагдана.
	civilID := strings.TrimSpace(info.RegisterNumber)
	nationalID := strings.ToLower(strings.TrimSpace(info.NationalID))
	// provider (dan) дээр иргэн Google-ээр нэвтэрсэн/холбосон бол энэ апп дээр ч
	// "Google холбогдсон" гэж тусгана.
	googleSub := strings.TrimSpace(info.GoogleSub)
	googleEmail := strings.TrimSpace(info.GoogleEmail)
	googleName := strings.TrimSpace(info.GoogleName)
	googlePicture := strings.TrimSpace(info.GooglePicture)

	// Private платформын хандалтын шалгуур — eID-ээр баталгаажсаны ДАРАА, upsert-
	// ийн ӨМНӨ. Private горимд урьдчилан бүртгээгүй иргэнийг энд зогсооно (шинэ
	// данс үүсгэхгүй).
	if err := u.enforceAccessMode(ctx, civilID, nationalID); err != nil {
		return CompleteResponse{}, err
	}

	var stored domain.User
	if civilID != "" {
		user := &domain.User{
			Username:      "eid_" + civilID,
			FirstName:     firstName,
			LastName:      lastName,
			FirstNameEn:   firstNameEn,
			LastNameEn:    lastNameEn,
			GoogleSub:     googleSub,
			GoogleEmail:   googleEmail,
			GoogleName:    googleName,
			GooglePicture: googlePicture,
			RoleID:        domain.RoleUser, // зөвхөн ШИНЭ мөрд; байгаа хэрэглэгчийн эрхийг хөндөхгүй
		}
		stored, err = u.store.UpsertByCivilID(ctx, civilID, nationalID, info.Sub, user)
	} else {
		// Иргэний дугааргүй (nationalid scope байхгүй/буцаагаагүй) — pairwise
		// sub-ээр. Refresh нь email-ээр хайдаг тул синтетик email хадгална.
		slug := subSlug(info.Sub)
		user := &domain.User{
			Username:      "sso_" + slug,
			FirstName:     firstName,
			LastName:      lastName,
			FirstNameEn:   firstNameEn,
			LastNameEn:    lastNameEn,
			Email:         "sso_" + slug + "@sso.local",
			GoogleSub:     googleSub,
			GoogleEmail:   googleEmail,
			GoogleName:    googleName,
			GooglePicture: googlePicture,
			Active:        true,
			RoleID:        domain.RoleUser,
		}
		stored, err = u.store.UpsertBySSOSub(ctx, info.Sub, user)
	}
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(fmt.Errorf("upsert sso user: %w", err))
	}

	// SSO OAuth токенуудыг (refresh_token-той бол) хадгална — SSO eID proxy-г
	// иргэний нэрийн өмнөөс дуудахад ашиглана. Алдаа гарвал нэвтрэлтийг унагахгүй
	// (proxy боломжгүй болно, гэхдээ бусад eID урсгал шууд ажиллана).
	if u.tokens != nil {
		if sErr := u.tokens.Store(ctx, stored.ID, tokens); sErr != nil {
			logger.ErrorWithContext(ctx, "sso: failed to store SSO tokens (non-fatal)", logger.Fields{"error": sErr.Error()})
		}
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

// accessDeniedMsg нь private платформд бүртгэлгүй иргэнд буцаах мессеж.
const accessDeniedMsg = "Энэ платформ хаалттай (private). Танд нэвтрэх эрх олгогдоогүй байна — системийн админд хандана уу."

// enforceAccessMode нь private платформ дээр зөвхөн админаас урьдчилан бүртгэсэн
// (national_id/civil_id-ээр тохирох) иргэнийг л оруулна. Public горимд (эсвэл
// reader тохируулаагүй бол) юу ч блоклохгүй. Горим унших/шалгах DB алдаа гарвал
// internal error буцааж, нэвтрэлтийг тэр удаад зогсооно (fail-open биш —
// баталгаагүй байдалд эрхгүй иргэнийг оруулахгүй).
func (u *usecase) enforceAccessMode(ctx context.Context, civilID, nationalID string) error {
	if u.access == nil {
		return nil // public (default)
	}
	mode, err := u.access.GetAccessMode(ctx)
	if err != nil {
		return apperror.InternalCause(fmt.Errorf("read access mode: %w", err))
	}
	if mode != domain.AccessModePrivate {
		return nil // public — хэн ч нэвтэрч болно
	}
	// Private: иргэнийг тодорхойлох дугаар байхгүй бол оруулах аргагүй.
	if civilID == "" && nationalID == "" {
		return apperror.Forbidden(accessDeniedMsg)
	}
	ok, err := u.store.AuthorizedByCivilOrNational(ctx, civilID, nationalID)
	if err != nil {
		return apperror.InternalCause(err)
	}
	if !ok {
		return apperror.Forbidden(accessDeniedMsg)
	}
	return nil
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

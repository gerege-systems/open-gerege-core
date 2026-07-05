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
	oidc  *oidc.Client
	store UserStore
	jwt   jwt.JWTService
	redis caches.RedisCache
}

// NewUsecase нь SSO usecase угсарна.
func NewUsecase(oidcClient *oidc.Client, store UserStore, jwtSvc jwt.JWTService, redis caches.RedisCache) Usecase {
	return &usecase{oidc: oidcClient, store: store, jwt: jwtSvc, redis: redis}
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

	// Code → access token + id token (client_secret_basic), дараа нь /userinfo.
	accessToken, idToken, err := u.oidc.Exchange(ctx, code)
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(err)
	}
	info, err := u.oidc.UserInfo(ctx, accessToken)
	if err != nil {
		return CompleteResponse{}, apperror.InternalCause(err)
	}

	// sub-ээс тогтвортой, аюулгүй урттай username/email гаргана (sub нь урт
	// pairwise hash). Refresh нь email-ээр (auth_refresh) хайдаг тул синтетик
	// email-ийг хадгална.
	slug := subSlug(info.Sub)
	user := &domain.User{
		Username:    "sso_" + slug,
		FirstName:   strings.TrimSpace(info.GivenName),
		LastName:    strings.TrimSpace(info.FamilyName),
		FirstNameEn: "",
		LastNameEn:  "",
		Email:       "sso_" + slug + "@sso.local",
		Active:      true,
		RoleID:      domain.RoleUser,
	}
	// given/family хоосон ч name байвал бүтэн нэрийг LastName-д (fallback) тавина.
	if user.FirstName == "" && user.LastName == "" && strings.TrimSpace(info.Name) != "" {
		user.LastName = strings.TrimSpace(info.Name)
	}

	stored, err := u.store.UpsertBySSOSub(ctx, info.Sub, user)
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

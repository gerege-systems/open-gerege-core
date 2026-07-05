// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/google"
	"template/pkg/jwt"
	"template/pkg/logger"
)

// googleAccountOf нь Google OAuth профайлыг холбоход хадгалах domain хэлбэрт буулгана.
func googleAccountOf(gu google.User) domain.GoogleAccount {
	return domain.GoogleAccount{
		Sub: gu.Sub, Email: gu.Email, EmailVerified: gu.EmailVerified,
		Name: gu.Name, Picture: gu.Picture,
	}
}

// googleLinkTTL нь Google→eID холбохыг хүлээх токены амьдрах хугацаа.
const googleLinkTTL = 15 * time.Minute

// mintSession нь хэрэглэгчид access+refresh токен хос үүсгэж, refresh-ийг
// Redis-д тэмдэглэнэ (Login/EIDPoll/Google хуваалцдаг).
func (uc *usecase) mintSession(ctx context.Context, user domain.User) (jwt.TokenPair, error) {
	pair, err := uc.jwtService.GenerateTokenPair(user.ID, user.IsAdmin(), user.RoleID, user.Email)
	if err != nil {
		return jwt.TokenPair{}, err
	}
	if err := uc.rememberRefresh(ctx, pair); err != nil {
		return jwt.TokenPair{}, err
	}
	return pair, nil
}

// GoogleLogin нь Google authorization code-ийг token руу солиж, тухайн Google
// account холбогдсон эсэхийг шалгана:
//   - Холбогдсон → шууд access+refresh токен олгож нэвтрүүлнэ (Linked=true).
//   - Холбоогүй (эхний удаа) → богино хугацааны LinkToken үүсгэж буцаана; клиент
//     дараа нь eID нэвтрэлт хийж (EIDPoll-д LinkToken дамжуулж) бодит хүнтэй
//     холбоно (Linked=false).
func (uc *usecase) GoogleLogin(ctx context.Context, code, redirectURI string) (resp GoogleLoginResponse, err error) {
	const (
		usecaseName = "auth"
		funcName    = "GoogleLogin"
		fileName    = "auth_google.go"
	)

	if uc.google == nil || !uc.google.Configured() {
		return GoogleLoginResponse{}, apperror.InternalCause(fmt.Errorf("google login not configured"))
	}

	gu, exErr := uc.google.Exchange(ctx, code, redirectURI)
	if exErr != nil {
		logger.ErrorWithContext(ctx, "GoogleLogin failed: token exchange", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": exErr.Error(),
		})
		return GoogleLoginResponse{}, apperror.BadRequest("Google нэвтрэлт амжилтгүй боллоо")
	}

	// Аль хэдийн холбогдсон Google account уу?
	user, lookErr := uc.users.GetByGoogleSub(ctx, gu.Sub)
	if lookErr == nil {
		// Профайлыг (нэр/зураг/и-мэйл) хамгийн сүүлийн Google утгаар шинэчилнэ
		// (best-effort — нэвтрэлтийг тасалдуулахгүй).
		if refreshErr := uc.users.LinkGoogleAccount(ctx, user.ID, googleAccountOf(*gu)); refreshErr != nil {
			logger.ErrorWithContext(ctx, "google profile refresh failed (non-fatal)", logger.Fields{
				"usecase": usecaseName, "method": funcName, "file": fileName, "error": refreshErr.Error(),
			})
		}
		pair, mintErr := uc.mintSession(ctx, user)
		if mintErr != nil {
			return GoogleLoginResponse{}, apperror.InternalCause(fmt.Errorf("mint session: %w", mintErr))
		}
		return GoogleLoginResponse{Linked: true, Login: LoginResponse{
			User: user, AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken,
		}}, nil
	}

	var domErr *apperror.DomainError
	if !errors.As(lookErr, &domErr) || domErr.Type != apperror.ErrTypeNotFound {
		return GoogleLoginResponse{}, lookErr // жинхэнэ алдаа (DB г.м.)
	}

	// Эхний удаа — eID-ээр баталгаажуулах LinkToken үүсгэнэ. Google профайлыг
	// бүтнээр нь (JSON) Redis-д хадгална — eID COMPLETE болоход хэрэглэгчид
	// холбохдоо email/нэр/зураг зэргийг бүгдийг хадгалахад ашиглана.
	token, tErr := randomLinkToken()
	if tErr != nil {
		return GoogleLoginResponse{}, apperror.InternalCause(fmt.Errorf("link token: %w", tErr))
	}
	payload, mErr := json.Marshal(gu)
	if mErr != nil {
		return GoogleLoginResponse{}, apperror.InternalCause(fmt.Errorf("marshal google profile: %w", mErr))
	}
	key := GoogleLinkKey(token)
	if setErr := uc.redisCache.Set(ctx, key, string(payload)); setErr != nil {
		return GoogleLoginResponse{}, apperror.InternalCause(fmt.Errorf("store link token: %w", setErr))
	}
	_ = uc.redisCache.Expire(ctx, key, googleLinkTTL)

	return GoogleLoginResponse{Linked: false, LinkToken: token, Email: gu.Email}, nil
}

// UnlinkGoogleFromUser нь хэрэглэгчийн Google холболтыг арилгана.
func (uc *usecase) UnlinkGoogleFromUser(ctx context.Context, userID string) error {
	return uc.users.UnlinkGoogle(ctx, userID)
}

// linkGoogleIfPending нь EIDPoll COMPLETE болоход дуудагдана: GoogleLinkToken
// байвал тухайн Google account-ийг (Redis-ээс sub-г GetDel-ээр авч) энэ eID
// хэрэглэгчид холбоно. Холболтын алдаа non-fatal — eID нэвтрэлт үргэлж амжилттай
// (лог-д тэмдэглэнэ; жишээ нь Google account өөр хүнд аль хэдийн холбогдсон бол).
func (uc *usecase) linkGoogleIfPending(ctx context.Context, userID, linkToken string) {
	if linkToken == "" {
		return
	}
	raw, err := uc.redisCache.GetDel(ctx, GoogleLinkKey(linkToken))
	if err != nil || raw == "" {
		logger.ErrorWithContext(ctx, "google link token invalid/expired (non-fatal)", logger.Fields{
			"usecase": "auth", "method": "linkGoogleIfPending", "has_error": err != nil,
		})
		return
	}
	var gu google.User
	if uErr := json.Unmarshal([]byte(raw), &gu); uErr != nil || gu.Sub == "" {
		logger.ErrorWithContext(ctx, "google link payload invalid (non-fatal)", logger.Fields{
			"usecase": "auth", "method": "linkGoogleIfPending", "has_error": uErr != nil,
		})
		return
	}
	if linkErr := uc.users.LinkGoogleAccount(ctx, userID, googleAccountOf(gu)); linkErr != nil {
		logger.ErrorWithContext(ctx, "google link failed (non-fatal)", logger.Fields{
			"usecase": "auth", "method": "linkGoogleIfPending", "error": linkErr.Error(), "user_id": userID,
		})
	}
}

// randomLinkToken нь 32 hex тэмдэгтийн (16 байт) crypto/rand токен үүсгэнэ.
func randomLinkToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

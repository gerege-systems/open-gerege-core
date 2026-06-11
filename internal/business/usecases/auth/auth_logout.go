// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/pkg/logger"
)

// Logout нь refresh токены jti-г Redis-ээс устгаснаар токеныг хүчингүй болгоно.
// AccessToken өгөгдсөн бол түүний jti-г токены үлдсэн амьдрах хугацаагаар
// deny-list-д нэмдэг тул access токен ч мөн шууд хүчингүй болно (auth
// middleware хүсэлт бүрд шалгадаг). Access deny нь best-effort — задлагдахгүй
// токен logout-ийг унагадаггүй (refresh revoke нь гол ажиллагаа).
func (uc *usecase) Logout(ctx context.Context, req LogoutRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "Logout"
		fileName    = "auth_logout.go"
	)
	startTime := time.Now()
	refreshToken := req.RefreshToken

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"has_refresh_token": refreshToken != "",
		},
	})

	defer func() {
		duration := time.Since(startTime)
		fields := logger.Fields{
			"usecase":  usecaseName,
			"method":   funcName,
			"file":     fileName,
			"duration": duration.Milliseconds(),
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	claims, parseErr := uc.jwtService.ParseRefreshToken(refreshToken)
	if parseErr != nil {
		err = apperror.Unauthorized("invalid refresh token")
		logger.ErrorWithContext(ctx, "Logout failed: invalid token", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "parse_refresh_token",
			"error":   parseErr.Error(),
		})
		return err
	}
	if delErr := uc.redisCache.Del(ctx, RefreshKey(claims.ID)); delErr != nil {
		err = apperror.InternalCause(fmt.Errorf("revoke refresh: %w", delErr))
		logger.ErrorWithContext(ctx, "Logout failed: redis del error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "redis_del",
			"error":   delErr.Error(),
			"jti":     claims.ID,
		})
		return err
	}

	uc.denyAccessToken(ctx, req.AccessToken)
	return nil
}

// denyAccessToken нь access токены jti-г үлдсэн амьдрах хугацаагаар нь
// deny-list-д нэмнэ. Best-effort: токен хоосон / задлагдахгүй / аль хэдийн
// дууссан бол чимээгүй алгасна (logout-ийн үр дүнд нөлөөлөхгүй).
func (uc *usecase) denyAccessToken(ctx context.Context, accessToken string) {
	if accessToken == "" {
		return
	}
	claims, parseErr := uc.jwtService.ParseToken(accessToken)
	if parseErr != nil || claims.ID == "" || claims.ExpiresAt == nil {
		return
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return
	}
	key := AccessDenyKey(claims.ID)
	if setErr := uc.redisCache.Set(ctx, key, "1"); setErr != nil {
		logger.ErrorWithContext(ctx, "Logout: failed to deny access token (non-fatal)", logger.Fields{
			"step":  "redis_set_access_deny",
			"error": setErr.Error(),
		})
		return
	}
	if expErr := uc.redisCache.Expire(ctx, key, ttl); expErr != nil {
		logger.ErrorWithContext(ctx, "Logout: failed to set deny TTL (non-fatal)", logger.Fields{
			"step":  "redis_expire_access_deny",
			"error": expErr.Error(),
		})
	}
}

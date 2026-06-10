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
// Access токенууд жам ёсны дуусах хугацаа хүртэл хүчинтэй хэвээр үлддэг —
// клиентүүд logout хийхдээ тэдгээрийг хаях ёстой. (Бүрэн access токены хар
// жагсаалт нь хүсэлт тутамд Redis-руу нэг алхам нэмэх тул энэ boilerplate-ийн
// хүрээнээс зориудаар гадуур орхисон.)
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
	return nil
}

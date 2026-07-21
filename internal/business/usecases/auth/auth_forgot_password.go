// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
	"template/pkg/logger"
)

// ForgotPassword нь GeregeCloud Verify API-аар нэг удаагийн OTP кодыг
// email-ээр илгээж, буцаасан request_id-г TTL-тэйгээр Redis-д хадгална;
// ResetPassword дараа нь кодыг тэр request_id-аар шалгана. Email-ийн
// тооллогыг (enumeration) таслахын тулд email байгаа эсэхээс үл хамааран
// ижил хариу (nil) буцаана — тодорхойгүй email-д код илгээхгүй (зардал +
// спам).
func (uc *usecase) ForgotPassword(ctx context.Context, req ForgotPasswordRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "ForgotPassword"
		fileName    = "auth_forgot_password.go"
	)
	startTime := time.Now()
	email := domain.NormalizeEmail(req.Email)

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
		"request": logger.Fields{"email": email},
	})
	defer func() {
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		})
	}()

	if uc.cfg.ForgotMaxAttempts > 0 {
		key := ForgotAttemptsKey(email)
		attempts, incrErr := uc.incrWithExpiry(ctx, key, uc.cfg.ForgotLockoutTTL, "forgot_attempts")
		if incrErr != nil {
			logger.ErrorWithContext(ctx, "ForgotPassword: failed to track attempts (non-fatal)", logger.Fields{
				"usecase": usecaseName, "method": funcName, "file": fileName,
				"step": "redis_incr_attempts", "error": incrErr.Error(), "email": email,
			})
		}
		if attempts > int64(uc.cfg.ForgotMaxAttempts) {
			logger.ErrorWithContext(ctx, "ForgotPassword failed: rate limit exceeded", logger.Fields{
				"usecase": usecaseName, "method": funcName, "file": fileName,
				"step": "check_rate_limit", "email": email, "attempts": attempts,
			})
			return apperror.Forbidden("too many password reset requests, please try again later")
		}
	}

	_, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: email})
	if lookupErr != nil {
		var domErr *apperror.DomainError
		if errors.As(lookupErr, &domErr) && domErr.Type == apperror.ErrTypeNotFound {
			// Enumeration: тодорхойгүй email-д код илгээхгүй, ижил nil буцаана.
			return nil
		}
		logger.ErrorWithContext(ctx, "Forgot password failed: user lookup error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "get_user_by_email", "error": lookupErr.Error(), "email": email,
		})
		return lookupErr
	}

	// Verify API-аар reset OTP илгээж request_id-г email-ээр хадгална.
	requestID, sendErr := uc.verifier.Send(ctx, email, "")
	if sendErr != nil {
		logger.ErrorWithContext(ctx, "Forgot password failed: verify send error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "verify_send", "error": sendErr.Error(), "email": email,
		})
		return apperror.InternalCause(fmt.Errorf("send reset otp: %w", sendErr))
	}
	resetKey := ResetRequestKey(email)
	if setErr := uc.redisCache.Set(ctx, resetKey, requestID); setErr != nil {
		logger.ErrorWithContext(ctx, "Forgot password failed: persist request_id error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_set_reset_request", "error": setErr.Error(), "email": email,
		})
		return apperror.InternalCause(fmt.Errorf("persist reset request: %w", setErr))
	}
	if expireErr := uc.redisCache.Expire(ctx, resetKey, uc.cfg.PasswordResetTTL); expireErr != nil {
		logger.ErrorWithContext(ctx, "Forgot password: failed to set TTL on reset request (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_expire_reset_request", "error": expireErr.Error(), "email": email,
		})
	}
	return nil
}

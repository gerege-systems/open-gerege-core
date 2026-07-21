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
	"template/pkg/observability"
	"template/pkg/verify"
)

// VerifyOTP нь Redis дахь request_id-г уншиж, GeregeCloud Verify API-ийн /check-д
// кодыг шалгуулж, амжилттай бол бүртгэлийг идэвхжүүлнэ. Verify үйлчилгээ өөрөө
// brute-force-ыг хязгаарладаг ч, дотооддоо email тус бүрийн оролдлогын
// тоолуурыг (Config.OTPMaxAttempts) давхар барина.
func (uc *usecase) VerifyOTP(ctx context.Context, req VerifyOTPRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "VerifyOTP"
		fileName    = "auth_verify_otp.go"
	)
	startTime := time.Now()
	email := domain.NormalizeEmail(req.Email)
	otpCode := req.OTPCode

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
		"request": logger.Fields{"email": email, "has_otp_code": otpCode != ""},
	})
	defer func() {
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		})
	}()

	lookupResp, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: email})
	if lookupErr != nil {
		logger.ErrorWithContext(ctx, "Verify OTP failed: user lookup error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "get_user_by_email", "error": lookupErr.Error(), "email": email,
		})
		return apperror.NotFound("email not found")
	}
	user := lookupResp.User

	if user.Active {
		logger.ErrorWithContext(ctx, "Verify OTP failed: account already activated", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "check_active", "user_id": user.ID,
		})
		return apperror.BadRequest("account already activated")
	}

	attemptsKey := OTPAttemptsKey(email)
	attempts, incrErr := uc.incrWithExpiry(ctx, attemptsKey, uc.cfg.OTPTTL, "otp_attempts")
	if incrErr != nil {
		logger.ErrorWithContext(ctx, "Verify OTP: failed to track attempts (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_incr_attempts", "error": incrErr.Error(), "email": email,
		})
	}
	if attempts > int64(uc.cfg.OTPMaxAttempts) {
		logger.ErrorWithContext(ctx, "Verify OTP failed: lockout (max attempts exceeded)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "check_lockout", "email": email, "attempts": attempts,
		})
		return apperror.Forbidden("too many invalid otp attempts, please request a new code")
	}

	otpKey := UserOTPKey(email)
	requestID, getErr := uc.redisCache.Get(ctx, otpKey)
	if getErr != nil {
		observability.ObserveCacheOp("redis", "get", "miss")
		logger.ErrorWithContext(ctx, "Verify OTP failed: request_id expired or not found", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_get_request_id", "error": getErr.Error(), "email": email,
		})
		return apperror.BadRequest("otp code expired or not found")
	}
	observability.ObserveCacheOp("redis", "get", "hit")

	// Verify API-д кодыг шалгуулна. Буруу/хугацаа дууссан код → BadRequest
	// (request_id-г үлдээж дахин оролдох боломжтой); бусад нь дотоод алдаа.
	if checkErr := uc.verifier.Check(ctx, requestID, otpCode); checkErr != nil {
		if errors.Is(checkErr, verify.ErrNotApproved) {
			logger.ErrorWithContext(ctx, "Verify OTP failed: invalid code", logger.Fields{
				"usecase": usecaseName, "method": funcName, "file": fileName,
				"step": "verify_check", "email": email,
			})
			return apperror.BadRequest("invalid otp code")
		}
		logger.ErrorWithContext(ctx, "Verify OTP failed: verify check error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "verify_check", "error": checkErr.Error(), "email": email,
		})
		return apperror.InternalCause(fmt.Errorf("verify check: %w", checkErr))
	}

	if activateErr := uc.users.Activate(ctx, users.ActivateRequest{UserID: user.ID}); activateErr != nil {
		logger.ErrorWithContext(ctx, "Verify OTP failed: activate error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "users_activate", "error": activateErr.Error(), "user_id": user.ID,
		})
		return activateErr
	}

	if delErr := uc.redisCache.Del(ctx, otpKey); delErr != nil {
		logger.ErrorWithContext(ctx, "Verify OTP: failed to delete request_id (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_del_request_id", "error": delErr.Error(), "email": email,
		})
	}
	_ = uc.redisCache.Del(ctx, attemptsKey)
	return nil
}

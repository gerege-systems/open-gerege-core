// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
	"template/pkg/logger"
	"template/pkg/observability"
)

// SendOTP нь GeregeCloud Verify API-аар OTP-ийг email/SMS-ээр илгээж, буцаасан
// request_id-г TTL-тэйгээр Redis-д хадгална. Кодыг өөрөө үүсгэж/хадгалдаггүй —
// Verify үйлчилгээ кодын үүсгэлт, bcrypt-хадгалалт, илгээлт, brute-force-ыг
// хариуцдаг; template нь зөвхөн request_id-ийн амьдрах хугацааг хянана.
func (uc *usecase) SendOTP(ctx context.Context, req SendOTPRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "SendOTP"
		fileName    = "auth_send_otp.go"
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

	lookupResp, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: email})
	if lookupErr != nil {
		logger.ErrorWithContext(ctx, "Send OTP failed: user lookup error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "get_user_by_email", "error": lookupErr.Error(), "email": email,
		})
		return apperror.NotFound("email not found")
	}
	user := lookupResp.User

	if user.Active {
		logger.ErrorWithContext(ctx, "Send OTP failed: account already activated", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "check_active", "user_id": user.ID,
		})
		return apperror.BadRequest("account already activated")
	}

	// Verify API руу илгээх (channel = client-ийн өгөгдмөл, ихэвчлэн "email").
	requestID, sendErr := uc.verifier.Send(ctx, email, "")
	if sendErr != nil {
		logger.ErrorWithContext(ctx, "Send OTP failed: verify send error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "verify_send", "error": sendErr.Error(), "email": email,
		})
		return apperror.InternalCause(fmt.Errorf("send otp: %w", sendErr))
	}

	// request_id-г Redis-д хадгална (VerifyOTP үүгээр /check дуудна). Хадгалж
	// чадахгүй бол хэрэглэгчид баталгаажуулах боломжгүй болохыг тодорхой алдаагаар
	// мэдэгдэнэ.
	otpKey := UserOTPKey(email)
	if cacheErr := uc.redisCache.Set(ctx, otpKey, requestID); cacheErr != nil {
		observability.ObserveCacheOp("redis", "set", "error")
		logger.ErrorWithContext(ctx, "Send OTP failed: persist request_id error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_set_request_id", "error": cacheErr.Error(), "email": email,
		})
		return apperror.InternalCause(fmt.Errorf("persist otp request: %w", cacheErr))
	}
	observability.ObserveCacheOp("redis", "set", "ok")
	if expireErr := uc.redisCache.Expire(ctx, otpKey, uc.cfg.OTPTTL); expireErr != nil {
		logger.ErrorWithContext(ctx, "Send OTP: failed to set TTL on request_id (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_expire_request_id", "error": expireErr.Error(), "email": email,
		})
	}
	observability.ObserveOTPSend("sent")

	// Шинэ код илгээсэн тул оролдлогын тоологчийг тэглэнэ.
	_ = uc.redisCache.Del(ctx, OTPAttemptsKey(email))
	return nil
}

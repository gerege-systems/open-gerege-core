// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
	"template/pkg/helpers"
	"template/pkg/logger"
	"template/pkg/observability"
)

// SendOTP нь 6 оронтой код үүсгэж, TTL-тэйгээр Redis-д хадгалж, async mailer-ээр
// дамжуулан email-ийг дараалалд оруулна. HTTP хариу нь бодит SMTP хүргэлт дээр
// биш, дараалалд оруулсан үед буцдаг.
func (uc *usecase) SendOTP(ctx context.Context, req SendOTPRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "SendOTP"
		fileName    = "auth.send_otp.go"
	)
	startTime := time.Now()
	email := domain.NormalizeEmail(req.Email)

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"email": email,
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

	lookupResp, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: email})
	if lookupErr != nil {
		err = apperror.NotFound("email not found")
		logger.ErrorWithContext(ctx, "Send OTP failed: user lookup error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "get_user_by_email",
			"error":   lookupErr.Error(),
			"email":   email,
		})
		return err
	}
	user := lookupResp.User

	if user.Active {
		err = apperror.BadRequest("account already activated")
		logger.ErrorWithContext(ctx, "Send OTP failed: account already activated", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "check_active",
			"error":   err.Error(),
			"user_id": user.ID,
		})
		return err
	}

	code, otpErr := helpers.GenerateOTPCode(6)
	if otpErr != nil {
		err = apperror.InternalCause(fmt.Errorf("generate otp: %w", otpErr))
		logger.ErrorWithContext(ctx, "Send OTP failed: code generation error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "generate_otp_code",
			"error":   otpErr.Error(),
			"email":   email,
		})
		return err
	}

	// Кодыг ЭХЛЭЭД Redis-д (тохиргоотой OTPTTL-тэйгээр) хадгална, дараа нь
	// имэйлийг дараалалд оруулна. Урьд нь имэйлийг эхэлж дараалалд оруулдаг
	// байсан тул Redis Set амжилтгүй болоход (non-fatal, логлогддог)
	// хэрэглэгчид баталгаажуулах боломжгүй код хүрдэг байв. Код хадгалагдаж
	// чадахгүй бол имэйл огт явуулахгүй (баталгаажих боломжгүй кодыг
	// илгээхгүйн тулд).
	otpKey := UserOTPKey(email)
	if cacheErr := uc.redisCache.Set(ctx, otpKey, code); cacheErr != nil {
		observability.ObserveCacheOp("redis", "set", "error")
		err = apperror.InternalCause(fmt.Errorf("persist otp: %w", cacheErr))
		logger.ErrorWithContext(ctx, "Send OTP failed: persist OTP code error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "redis_set_otp",
			"error":   cacheErr.Error(),
			"email":   email,
		})
		return err
	}
	observability.ObserveCacheOp("redis", "set", "ok")
	// Set() нь кэшийн нийтлэг өгөгдмөл TTL-г хэрэглэдэг; OTP-ийн амьдрах
	// хугацааг оролдлогын тоологчтой (OTPAttemptsKey) тааруулахын тулд
	// тохируулсан OTPTTL-г тодорхой override хийнэ.
	if expireErr := uc.redisCache.Expire(ctx, otpKey, uc.cfg.OTPTTL); expireErr != nil {
		logger.ErrorWithContext(ctx, "Send OTP: failed to set TTL on OTP code (non-fatal)", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "redis_expire_otp",
			"error":   expireErr.Error(),
			"email":   email,
		})
	}

	if mailErr := uc.mailer.SendOTP(ctx, code, email); mailErr != nil {
		observability.ObserveMailerOp("queue_full")
		// Имэйл дараалалд орж чадаагүй тул хадгалсан кодыг устга — ингэснээр
		// хэзээ ч хүргэгдээгүй кодын дугаар Redis-д өнчирч үлдэхгүй.
		_ = uc.redisCache.Del(ctx, otpKey)
		err = apperror.InternalCause(fmt.Errorf("send otp: %w", mailErr))
		logger.ErrorWithContext(ctx, "Send OTP failed: mailer enqueue error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "mailer_send_otp",
			"error":   mailErr.Error(),
			"email":   email,
		})
		return err
	}

	_ = uc.redisCache.Del(ctx, OTPAttemptsKey(email))

	return nil
}

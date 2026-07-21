// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/pkg/logger"
	"template/pkg/observability"
	"template/pkg/verify"
)

// EmailSend нь шидтэний 3 дахь алхам: УРИЛГЫН и-мэйл рүү OTP илгээнэ.
//
// Хаяг нь pending session-оос (урилгын мөрөөс) ирдэг тул клиент өөр и-мэйл рүү
// код илгээж чадахгүй. Verify үйлчилгээ кодын үүсгэлт/хадгалалт/илгээлтийг
// хариуцах ба template зөвхөн request_id-г TTL-тэй хадгална (auth.SendOTP-ийн
// адил).
func (uc *usecase) EmailSend(ctx context.Context, req TokenRequest) (StepResponse, error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "EmailSend"
		fileName    = "onboarding_email.go"
	)

	sess, err := uc.loadPending(ctx, req.OnboardToken)
	if err != nil {
		return StepResponse{}, err
	}
	if err := requireStep(sess, StepEmail); err != nil {
		return StepResponse{}, err
	}

	requestID, sendErr := uc.verifier.Send(ctx, sess.Email, "email")
	if sendErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding: OTP илгээх амжилтгүй", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "verify_send", "error": sendErr.Error(),
		})
		return StepResponse{}, apperror.InternalCause(fmt.Errorf("send otp: %w", sendErr))
	}

	otpKey := OnboardOTPKey(req.OnboardToken)
	if cacheErr := uc.redisCache.Set(ctx, otpKey, requestID); cacheErr != nil {
		observability.ObserveCacheOp("redis", "set", "error")
		return StepResponse{}, apperror.InternalCause(fmt.Errorf("persist otp request: %w", cacheErr))
	}
	observability.ObserveCacheOp("redis", "set", "ok")
	if expErr := uc.redisCache.Expire(ctx, otpKey, uc.cfg.OTPTTL); expErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding: OTP TTL тогтоох амжилтгүй (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": expErr.Error(),
		})
	}
	observability.ObserveOTPSend("sent")

	// Шинэ код илгээсэн тул оролдлогын тоологчийг тэглэнэ.
	_ = uc.redisCache.Del(ctx, OnboardOTPAttemptsKey(req.OnboardToken))
	return StepResponse{Step: StepEmail}, nil
}

// EmailVerify нь OTP кодыг Verify API-аар шалгаж, амжилттай бол алхмыг "totp"
// болгоно. Verify өөрөө brute-force-ыг хязгаарладаг ч дотооддоо токен тус
// бүрийн оролдлогын тоологчийг (OTPMaxAttempts) давхар барина.
func (uc *usecase) EmailVerify(ctx context.Context, req EmailVerifyRequest) (StepResponse, error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "EmailVerify"
		fileName    = "onboarding_email.go"
	)

	sess, err := uc.loadPending(ctx, req.OnboardToken)
	if err != nil {
		return StepResponse{}, err
	}
	if err := requireStep(sess, StepEmail); err != nil {
		return StepResponse{}, err
	}

	attemptsKey := OnboardOTPAttemptsKey(req.OnboardToken)
	attempts, incrErr := uc.redisCache.Incr(ctx, attemptsKey)
	if incrErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding: оролдлого тоолох амжилтгүй (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": incrErr.Error(),
		})
	} else if attempts == 1 {
		_ = uc.redisCache.Expire(ctx, attemptsKey, uc.cfg.OTPTTL)
	}
	if uc.cfg.OTPMaxAttempts > 0 && attempts > int64(uc.cfg.OTPMaxAttempts) {
		logger.WarnWithContext(ctx, "superadmin onboarding: OTP оролдлого хэтэрлээ", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "attempts": attempts,
		})
		return StepResponse{}, apperror.Forbidden("Хэт олон буруу оролдлого — шинэ код авна уу")
	}

	requestID, getErr := uc.redisCache.Get(ctx, OnboardOTPKey(req.OnboardToken))
	if getErr != nil || requestID == "" {
		observability.ObserveCacheOp("redis", "get", "miss")
		return StepResponse{}, apperror.BadRequest("Кодын хугацаа дууссан эсвэл олдсонгүй — шинэ код авна уу")
	}
	observability.ObserveCacheOp("redis", "get", "hit")

	if checkErr := uc.verifier.Check(ctx, requestID, req.Code); checkErr != nil {
		if errors.Is(checkErr, verify.ErrNotApproved) {
			return StepResponse{}, apperror.BadRequest("Баталгаажуулах код буруу байна")
		}
		logger.ErrorWithContext(ctx, "superadmin onboarding: verify check алдаа", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": checkErr.Error(),
		})
		return StepResponse{}, apperror.InternalCause(fmt.Errorf("verify check: %w", checkErr))
	}

	sess.EmailVerified = true
	sess.Step = StepTOTP
	if err := uc.savePending(ctx, req.OnboardToken, sess); err != nil {
		return StepResponse{}, err
	}
	_ = uc.redisCache.Del(ctx, OnboardOTPKey(req.OnboardToken))
	_ = uc.redisCache.Del(ctx, attemptsKey)

	logger.InfoWithContext(ctx, "superadmin onboarding: и-мэйл баталгаажлаа", logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName, "step": StepTOTP,
	})
	return StepResponse{Step: StepTOTP}, nil
}

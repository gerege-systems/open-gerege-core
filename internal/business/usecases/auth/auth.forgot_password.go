// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
	"template/pkg/logger"
)

// resetTokenBytes нь тунгалаг бус (opaque) reset токены энтропи юм. 32
// байт (~256 бит) нь 30 минутын цонхон дахь brute force-ийг хэрэгжих
// боломжгүй болгодог.
const resetTokenBytes = 32

// ForgotPassword нь нэг удаагийн reset токен олгож, TTL-тэйгээр Redis-д
// хадгалж, хэрэглэгчид email-ээр илгээдэг. Email-ийн тооллогыг (enumeration)
// таслахын тулд email байгаа эсэхээс үл хамааран хариу нь ижил байдаг.
func (uc *usecase) ForgotPassword(ctx context.Context, req ForgotPasswordRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "ForgotPassword"
		fileName    = "auth.forgot_password.go"
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

	if uc.cfg.ForgotMaxAttempts > 0 {
		key := ForgotAttemptsKey(email)
		attempts, incrErr := uc.incrWithExpiry(ctx, key, uc.cfg.ForgotLockoutTTL, "forgot_attempts")
		if incrErr != nil {
			logger.ErrorWithContext(ctx, "ForgotPassword: failed to track attempts (non-fatal)", logger.Fields{
				"usecase": usecaseName,
				"method":  funcName,
				"file":    fileName,
				"step":    "redis_incr_attempts",
				"error":   incrErr.Error(),
				"email":   email,
			})
		}
		if attempts > int64(uc.cfg.ForgotMaxAttempts) {
			err = apperror.Forbidden("too many password reset requests, please try again later")
			logger.ErrorWithContext(ctx, "ForgotPassword failed: rate limit exceeded", logger.Fields{
				"usecase":  usecaseName,
				"method":   funcName,
				"file":     fileName,
				"step":     "check_rate_limit",
				"error":    err.Error(),
				"email":    email,
				"attempts": attempts,
			})
			return err
		}
	}

	// Тодорхойгүй-email-ийн зам нь тодорхой-email-ийн замтай ижил крипто
	// ажил хийхийн тулд токеныг болзолгүйгээр үүсгэнэ. Энэ нь ижил 200-OK
	// хариуг нөхөж, цаг хугацаанд суурилсан email тооллогыг таслана.
	token, tokenErr := generateResetToken()
	if tokenErr != nil {
		err = apperror.InternalCause(fmt.Errorf("generate reset token: %w", tokenErr))
		logger.ErrorWithContext(ctx, "Forgot password failed: token generation error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "generate_reset_token",
			"error":   tokenErr.Error(),
			"email":   email,
		})
		return err
	}

	lookupResp, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: email})
	if lookupErr != nil {
		var domErr *apperror.DomainError
		if errors.As(lookupErr, &domErr) && domErr.Type == apperror.ErrTypeNotFound {
			// Хамгаалалт (hedge): тодорхойгүй email-ууд тодорхой
			// email-уудтай ойролцоо ижил цаг зарцуулахын тулд адил
			// хэлбэрийн Redis бичилт хийнэ.
			decoyKey := PasswordResetKey(token)
			_ = uc.redisCache.Set(ctx, decoyKey, "decoy")
			_ = uc.redisCache.Expire(ctx, decoyKey, uc.cfg.PasswordResetTTL)
			_ = uc.redisCache.Del(ctx, decoyKey)
			return nil
		}
		err = lookupErr
		logger.ErrorWithContext(ctx, "Forgot password failed: user lookup error", logger.Fields{
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

	// Алдагдсан өмнөх холбоос шинэтэй нь өрсөлдөхгүйн тулд энэ хэрэглэгчид
	// одоо ч амьд байгаа аливаа токеныг хүчингүй болго. "Би reset-ийг хоёр
	// удаа хүссэн" гэдэгт хүлээгдэх сэтгэцийн загвар нь нэг идэвхтэй токен юм.
	if prior, getErr := uc.redisCache.Get(ctx, UserResetIndexKey(user.ID)); getErr == nil && prior != "" {
		_ = uc.redisCache.Del(ctx, PasswordResetKey(prior))
	}

	if setErr := uc.redisCache.Set(ctx, PasswordResetKey(token), user.ID); setErr != nil {
		err = apperror.InternalCause(fmt.Errorf("persist reset token: %w", setErr))
		logger.ErrorWithContext(ctx, "Forgot password failed: persist token error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "redis_set_reset_token",
			"error":   setErr.Error(),
			"user_id": user.ID,
		})
		return err
	}
	if expireErr := uc.redisCache.Expire(ctx, PasswordResetKey(token), uc.cfg.PasswordResetTTL); expireErr != nil {
		logger.ErrorWithContext(ctx, "Forgot password: failed to set TTL on reset token (non-fatal)", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "redis_expire_reset_token",
			"error":   expireErr.Error(),
		})
	}
	if setIdxErr := uc.redisCache.Set(ctx, UserResetIndexKey(user.ID), token); setIdxErr != nil {
		logger.ErrorWithContext(ctx, "Forgot password: failed to update user reset index (non-fatal)", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "redis_set_user_index",
			"error":   setIdxErr.Error(),
			"user_id": user.ID,
		})
	} else {
		_ = uc.redisCache.Expire(ctx, UserResetIndexKey(user.ID), uc.cfg.PasswordResetTTL)
	}

	if mailErr := uc.mailer.SendPasswordReset(ctx, token, email); mailErr != nil {
		err = apperror.InternalCause(fmt.Errorf("send reset email: %w", mailErr))
		logger.ErrorWithContext(ctx, "Forgot password failed: mailer error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "mailer_send_password_reset",
			"error":   mailErr.Error(),
			"email":   email,
		})
		return err
	}
	return nil
}

func generateResetToken() (string, error) {
	buf := make([]byte, resetTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

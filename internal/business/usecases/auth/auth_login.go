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
)

// Login нь credential-ийг шалгаж, шинэ access+refresh токен хосыг буцаана.
// Хэрэглэгчийн тооллогыг (enumeration) далдлахын тулд буруу нууц үг болон
// тодорхойгүй email ижил хэмжээний бодит цаг (wall time) зарцуулдаг.
//
// Brute-force түгжих: email тус бүрийн Redis тоолуур login оролдлого бүр дээр
// нэмэгдэж, амжилттай болоход цэвэрлэгддэг. Config.LoginLockoutTTL дотор
// Config.LoginMaxAttempts-аас давмагц email түгжигдэнэ — per-IP rate limit-ээр
// илрүүлэх боломжгүй удаан тархсан brute-force-ийг таслан зогсооно.
func (uc *usecase) Login(ctx context.Context, req LoginRequest) (resp LoginResponse, err error) {
	const (
		usecaseName = "auth"
		funcName    = "Login"
		fileName    = "auth_login.go"
	)
	startTime := time.Now()
	email := domain.NormalizeEmail(req.Email)
	password := req.Password

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
		if err == nil {
			fields["response"] = logger.Fields{"user_id": resp.User.ID}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	// Brute-force-ийн хамгаалалт. Тодорхойгүй email хүртэл түгжилт рүү
	// тоологдохын тулд эхлээд нэмэгдүүлнэ — эс бөгөөс халдагч email-ийн хүчинтэй
	// эсэхийг үнэгүй шалгаж чадна. Тоолуур LoginLockoutTTL-ийг хуваалцдаг тул
	// цонх дуусахад reset болдог.
	attemptsKey := LoginAttemptsKey(email)
	if uc.cfg.LoginMaxAttempts > 0 {
		attempts, incrErr := uc.incrWithExpiry(ctx, attemptsKey, uc.cfg.LoginLockoutTTL, "login_attempts")
		if incrErr != nil {
			logger.ErrorWithContext(ctx, "Login: failed to track attempts (non-fatal)", logger.Fields{
				"usecase": usecaseName,
				"method":  funcName,
				"file":    fileName,
				"step":    "redis_incr_attempts",
				"error":   incrErr.Error(),
				"email":   email,
			})
		}
		if attempts > int64(uc.cfg.LoginMaxAttempts) {
			err = apperror.Forbidden("too many failed login attempts, please try again later")
			logger.ErrorWithContext(ctx, "Login failed: lockout (max attempts exceeded)", logger.Fields{
				"usecase":  usecaseName,
				"method":   funcName,
				"file":     fileName,
				"step":     "check_lockout",
				"error":    err.Error(),
				"email":    email,
				"attempts": attempts,
			})
			return LoginResponse{}, err
		}
	}

	lookupResp, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: email})
	if lookupErr != nil {
		// Энэ зам нь бодит нууц үгийн шалгалттай ойролцоо ижил бодит цаг
		// (wall-clock) зарцуулахын тулд хуурамч (dummy) bcrypt харьцуулалт хийнэ.
		_ = helpers.ValidateHash(password, dummyBcryptHash)
		err = apperror.Unauthorized("invalid email or password")
		logger.ErrorWithContext(ctx, "Login failed: user lookup error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "get_user_by_email",
			"error":   lookupErr.Error(),
			"email":   email,
		})
		return LoginResponse{}, err
	}
	user := lookupResp.User

	if !user.Active {
		err = apperror.Forbidden("account is not activated")
		logger.ErrorWithContext(ctx, "Login failed: account not activated", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "check_active",
			"error":   err.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	if !user.VerifyPassword(password) {
		err = apperror.Unauthorized("invalid email or password")
		logger.ErrorWithContext(ctx, "Login failed: invalid password", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "verify_password",
			"error":   err.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	pair, mintErr := uc.jwtService.GenerateTokenPair(user.ID, user.IsAdmin(), user.RoleID, user.Email)
	if mintErr != nil {
		err = apperror.InternalCause(fmt.Errorf("generate token: %w", mintErr))
		logger.ErrorWithContext(ctx, "Login failed: token generation error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "generate_token_pair",
			"error":   mintErr.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	if persistErr := uc.rememberRefresh(ctx, pair); persistErr != nil {
		err = apperror.InternalCause(fmt.Errorf("persist refresh: %w", persistErr))
		logger.ErrorWithContext(ctx, "Login failed: persist refresh error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "persist_refresh",
			"error":   persistErr.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	// Амжилт — дараагийн жинхэнэ session хуучирсан тоогоор эхлэхгүйн тулд
	// амжилтгүй болсны тоолуурыг цэвэрлэнэ.
	_ = uc.redisCache.Del(ctx, attemptsKey)

	resp = LoginResponse{
		User:         user,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}
	return resp, nil
}

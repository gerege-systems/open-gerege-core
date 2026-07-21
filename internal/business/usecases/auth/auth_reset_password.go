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
	"template/pkg/verify"
)

// ResetPassword нь ForgotPassword-ийн илгээсэн OTP кодыг GeregeCloud Verify
// /check-ээр баталгаажуулж, хэрэглэгчийн нууц үгийг солино. Амжилттай болоход
// request_id-г устгаж дахин тоглуулахаас (replay) сэргийлнэ.
func (uc *usecase) ResetPassword(ctx context.Context, req ResetPasswordRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "ResetPassword"
		fileName    = "auth_reset_password.go"
	)
	startTime := time.Now()
	email := domain.NormalizeEmail(req.Email)
	code := req.Code
	newPassword := req.NewPassword

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
		"request": logger.Fields{"email": email, "has_code": code != "", "has_new_password": newPassword != ""},
	})
	defer func() {
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		})
	}()

	if newPassword == "" {
		return apperror.BadRequest("new password is required")
	}
	if code == "" {
		return apperror.BadRequest("reset code is required")
	}

	lookupResp, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: email})
	if lookupErr != nil {
		logger.ErrorWithContext(ctx, "Reset password failed: user lookup error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "get_user_by_email", "error": lookupErr.Error(), "email": email,
		})
		return apperror.Unauthorized("reset code is invalid or expired")
	}
	user := lookupResp.User

	requestID, getErr := uc.redisCache.Get(ctx, ResetRequestKey(email))
	if getErr != nil || requestID == "" {
		logger.ErrorWithContext(ctx, "Reset password failed: reset request expired or not found", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "redis_get_reset_request", "email": email,
		})
		return apperror.Unauthorized("reset code is invalid or expired")
	}

	if checkErr := uc.verifier.Check(ctx, requestID, code); checkErr != nil {
		if errors.Is(checkErr, verify.ErrNotApproved) {
			logger.ErrorWithContext(ctx, "Reset password failed: invalid code", logger.Fields{
				"usecase": usecaseName, "method": funcName, "file": fileName,
				"step": "verify_check", "email": email,
			})
			return apperror.Unauthorized("reset code is invalid or expired")
		}
		logger.ErrorWithContext(ctx, "Reset password failed: verify check error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "verify_check", "error": checkErr.Error(), "email": email,
		})
		return apperror.InternalCause(fmt.Errorf("verify check: %w", checkErr))
	}

	if hashErr := user.ChangePassword(newPassword, uc.cfg.BcryptCost); hashErr != nil {
		if errors.Is(hashErr, domain.ErrEmptyPassword) {
			return apperror.BadRequest(hashErr.Error())
		}
		logger.ErrorWithContext(ctx, "Reset password failed: hash error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "domain_change_password", "error": hashErr.Error(), "user_id": user.ID,
		})
		return apperror.InternalCause(fmt.Errorf("hash reset password: %w", hashErr))
	}
	if updateErr := uc.users.UpdatePassword(ctx, users.UpdatePasswordRequest{User: &user}); updateErr != nil {
		logger.ErrorWithContext(ctx, "Reset password failed: update error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "users_update_password", "error": updateErr.Error(), "user_id": user.ID,
		})
		return updateErr
	}
	_ = uc.redisCache.Del(ctx, ResetRequestKey(email))
	if user.PasswordChangedAt != nil {
		uc.recordTokenCutoff(ctx, user.ID, *user.PasswordChangedAt)
	}
	return nil
}

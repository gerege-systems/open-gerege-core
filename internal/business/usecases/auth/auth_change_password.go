// Gerege Template Version 27.0
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

// ChangePassword нь баталгаажсан хэрэглэгчийн нууц үгийг одоогийнхыг нь
// шалгасны дараа солино. Шинэ PasswordChangedAt timestamp нь хүчингүй болгох
// тасалбар цэг болж ажилладаг — түүнээс өмнө олгогдсон refresh токенуудыг
// /refresh дээр татгалздаг.
func (uc *usecase) ChangePassword(ctx context.Context, req ChangePasswordRequest) (err error) {
	const (
		usecaseName = "auth"
		funcName    = "ChangePassword"
		fileName    = "auth_change_password.go"
	)
	startTime := time.Now()
	userID := req.UserID
	currentPassword := req.CurrentPassword
	newPassword := req.NewPassword

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"user_id":              userID,
			"has_current_password": currentPassword != "",
			"has_new_password":     newPassword != "",
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

	if newPassword == "" {
		err = apperror.BadRequest("new password is required")
		logger.ErrorWithContext(ctx, "Change password failed: empty new password", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "validate_new_password",
			"error":   err.Error(),
			"user_id": userID,
		})
		return err
	}
	lookupResp, lookupErr := uc.users.GetByID(ctx, users.GetByIDRequest{ID: userID})
	if lookupErr != nil {
		err = lookupErr
		logger.ErrorWithContext(ctx, "Change password failed: user lookup error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "get_user_by_id",
			"error":   lookupErr.Error(),
			"user_id": userID,
		})
		return err
	}
	user := lookupResp.User
	if !user.VerifyPassword(currentPassword) {
		err = apperror.Unauthorized("current password is incorrect")
		logger.ErrorWithContext(ctx, "Change password failed: invalid current password", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "verify_current_password",
			"error":   err.Error(),
			"user_id": userID,
		})
		return err
	}
	if hashErr := user.ChangePassword(newPassword, uc.cfg.BcryptCost); hashErr != nil {
		if errors.Is(hashErr, domain.ErrEmptyPassword) {
			err = apperror.BadRequest(hashErr.Error())
		} else {
			err = apperror.InternalCause(fmt.Errorf("hash new password: %w", hashErr))
		}
		logger.ErrorWithContext(ctx, "Change password failed: hash error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "domain_change_password",
			"error":   hashErr.Error(),
			"user_id": userID,
		})
		return err
	}
	if updateErr := uc.users.UpdatePassword(ctx, users.UpdatePasswordRequest{User: &user}); updateErr != nil {
		err = updateErr
		logger.ErrorWithContext(ctx, "Change password failed: update error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "users_update_password",
			"error":   updateErr.Error(),
			"user_id": userID,
		})
		return err
	}
	if user.PasswordChangedAt != nil {
		uc.recordTokenCutoff(ctx, userID, *user.PasswordChangedAt)
	}
	return nil
}

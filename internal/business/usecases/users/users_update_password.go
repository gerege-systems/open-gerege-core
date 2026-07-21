// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/pkg/logger"
)

// UpdatePassword нь аль хэдийн hash хийсэн нууц үг + хүчингүй болгох timestamp-ийг
// өгөгдсөн хэрэглэгч дээр хадгална. Үүнийг дуудахаас өмнө дуудагч Password +
// PasswordChangedAt + UpdatedAt-ийг бөглөхийн тулд эхлээд
// domain.User.ChangePassword-ийг дуудна гэж тооцдог.
func (uc *usecase) UpdatePassword(ctx context.Context, req UpdatePasswordRequest) (err error) {
	const (
		usecaseName = "users"
		funcName    = "UpdatePassword"
		fileName    = "users_update_password.go"
	)
	startTime := time.Now()

	var userID string
	if req.User != nil {
		userID = req.User.ID
	}
	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"user_id": userID,
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

	if req.User == nil || req.User.ID == "" {
		err = apperror.BadRequest("user id required")
		logger.ErrorWithContext(ctx, "Update password failed: missing user id", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "validate_input",
			"error":   err.Error(),
		})
		return err
	}
	if repoErr := uc.repo.UpdatePassword(ctx, req.User); repoErr != nil {
		err = mapRepoError(repoErr, fmt.Sprintf("update password for %s", req.User.ID))
		logger.ErrorWithContext(ctx, "Update password failed: repository error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "repo_update_password",
			"error":   repoErr.Error(),
			"user_id": req.User.ID,
		})
		return err
	}
	// Дараагийн Login нь хуучирсан (хуучин hash-тай) кэшлэгдсэн хэрэглэгчийг
	// уншихгүйн тулд email-ээр түлхүүрлэгдсэн ristretto бичлэгийг хүчингүй болгоно.
	email := req.User.Email
	if email == "" {
		if existing, getErr := uc.repo.GetByID(ctx, req.User.ID); getErr == nil {
			email = existing.Email
		}
	}
	if email != "" {
		uc.ristrettoCache.Del(fmt.Sprintf("user/%s", email))
	}
	return nil
}

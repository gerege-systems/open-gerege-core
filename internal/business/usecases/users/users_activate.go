// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/logger"
)

// Activate нь хэрэглэгчийн active флагийг хувиргана — цорын ганц зүй ёсны
// дуудагч нь auth context-ийн VerifyOTP урсгал юм. `active`-ийг хувиргах нь
// юу өдөөснөөс үл хамааран хэрэглэгчийн бичлэг дээрх үйлдэл тул энэ нь Auth-д
// биш, User bounded context-д байрладаг.
func (uc *usecase) Activate(ctx context.Context, req ActivateRequest) (err error) {
	const (
		usecaseName = "users"
		funcName    = "Activate"
		fileName    = "users_activate.go"
	)
	startTime := time.Now()

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"user_id": req.UserID,
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

	u := &domain.User{ID: req.UserID}
	u.Activate()
	if changeErr := uc.repo.ChangeActiveUser(ctx, u); changeErr != nil {
		err = apperror.InternalCause(fmt.Errorf("activate user: %w", changeErr))
		logger.ErrorWithContext(ctx, "Activate user failed: repository error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "repo_change_active_user",
			"error":   changeErr.Error(),
			"user_id": req.UserID,
		})
		return err
	}
	// Дараагийн Login нь OTP урсгалын үед GetByEmail-ийн бөглөсөн хуучирсан
	// (Active=false) бичлэгийг уншихгүйн тулд ristretto кэшийг хүчингүй болгоно.
	if existing, getErr := uc.repo.GetByID(ctx, req.UserID); getErr == nil && existing.Email != "" {
		uc.ristrettoCache.Del(fmt.Sprintf("user/%s", existing.Email))
	}
	return nil
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"fmt"
	"time"

	"template/pkg/logger"
)

// GetByID нь өгөгдсөн primary key-тэй хэрэглэгчийг буцаана. ID-аар хайх нь
// email-ээр түлхүүрлэгдсэн кэшээр дамждаггүй — тэдгээр нь шууд DB унших нь
// зүгээр байхуйц ховор бөгөөд ID-аар кэшлэх нь хэмжигдэх hit rate-гүйгээр
// төлөвийг давхардуулах болно.
func (uc *usecase) GetByID(ctx context.Context, req GetByIDRequest) (resp GetByIDResponse, err error) {
	const (
		usecaseName = "users"
		funcName    = "GetByID"
		fileName    = "users_get_by_id.go"
	)
	startTime := time.Now()

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"user_id": req.ID,
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
			fields["response"] = logger.Fields{"user_id": resp.User.ID, "email": resp.User.Email}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	user, repoErr := uc.repo.GetByID(ctx, req.ID)
	if repoErr != nil {
		err = mapRepoError(repoErr, "get user by id")
		logger.ErrorWithContext(ctx, "Get user by id failed: repository error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "repo_get_by_id",
			"error":   repoErr.Error(),
			"user_id": req.ID,
		})
		return GetByIDResponse{}, err
	}
	resp = GetByIDResponse{User: user}
	return resp, nil
}

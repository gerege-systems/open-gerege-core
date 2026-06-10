// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"fmt"
	"time"

	"template/internal/business/usecases/users"
	"template/pkg/logger"
)

// Register нь шинэ, идэвхгүй хэрэглэгчийн бүртгэл үүсгэнэ. Login амжилттай
// болохоос өмнө хэрэглэгч SendOTP + VerifyOTP-ийг үргэлжлүүлэн хийх ёстой.
//
// Өнөөдөр энэ нь цэвэр төлөөлөл (delegation); ирээдүйн урьдчилсан шалгалтууд
// (IP-аар rate limit, blocklist, captcha, урилгын кодын хаалт) нь User
// context-ийн мэдэлгүйгээр энд бууж болохын тулд auth хилийн ард байрладаг.
func (uc *usecase) Register(ctx context.Context, req RegisterRequest) (resp RegisterResponse, err error) {
	const (
		usecaseName = "auth"
		funcName    = "Register"
		fileName    = "auth_register.go"
	)
	startTime := time.Now()
	in := req.User

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"username": in.Username,
			"email":    in.Email,
			"role_id":  in.RoleID,
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

	storeResp, storeErr := uc.users.Store(ctx, users.StoreRequest{User: in})
	if storeErr != nil {
		err = storeErr
		logger.ErrorWithContext(ctx, "Register failed: store error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "users_store",
			"error":   err.Error(),
			"email":   in.Email,
		})
		return RegisterResponse{}, err
	}
	resp = RegisterResponse{User: storeResp.User}
	return resp, nil
}

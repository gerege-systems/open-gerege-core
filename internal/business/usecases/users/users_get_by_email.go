// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"fmt"
	"time"

	"template/internal/business/domain"
	"template/pkg/logger"
	"template/pkg/observability"
)

// GetByEmail нь өгөгдсөн email-тэй хэрэглэгчийг буцаана. Эхлээд санах ой дахь
// (Ristretto) кэшийг шалгана; алдалт (miss) дээр зэрэгцээ goroutine-ууд Postgres
// руу олон зэрэг хүсэлт (thundering herd) очихоос сэргийлж singleflight-аар
// дамжуулан нэг DB алхамыг хуваалцана.
func (uc *usecase) GetByEmail(ctx context.Context, req GetByEmailRequest) (resp GetByEmailResponse, err error) {
	const (
		usecaseName = "users"
		funcName    = "GetByEmail"
		fileName    = "users_get_by_email.go"
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
		if err == nil {
			fields["response"] = logger.Fields{"user_id": resp.User.ID}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	cacheKey := fmt.Sprintf("user/%s", email)
	if val := uc.ristrettoCache.Get(cacheKey); val != nil {
		if cached, ok := val.(domain.User); ok {
			observability.ObserveCacheOp("ristretto", "get", "hit")
			resp = GetByEmailResponse{User: cached}
			return resp, nil
		}
		observability.ObserveCacheOp("ristretto", "get", "error")
		logger.WarnWithContext(ctx, "Get user by email: cache type assertion failed", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "ristretto_cache_get",
			"email":   email,
		})
	} else {
		observability.ObserveCacheOp("ristretto", "get", "miss")
	}

	v, sfErr, _ := uc.userByEmailGroup.Do(email, func() (any, error) {
		user, repoErr := uc.repo.GetByEmail(ctx, &domain.User{Email: email})
		if repoErr != nil {
			return domain.User{}, repoErr
		}
		uc.ristrettoCache.Set(cacheKey, user)
		observability.ObserveCacheOp("ristretto", "set", "ok")
		return user, nil
	})
	if sfErr != nil {
		// Дэд бүтцийн алдаанууд 404 мэт харагдахгүйн тулд төрөлжсөн алдаануудыг дамжуулна.
		err = mapRepoError(sfErr, "get user by email")
		logger.ErrorWithContext(ctx, "Get user by email failed: repository error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "repo_get_by_email",
			"error":   sfErr.Error(),
			"email":   email,
		})
		return GetByEmailResponse{}, err
	}
	resp = GetByEmailResponse{User: v.(domain.User)}
	return resp, nil
}

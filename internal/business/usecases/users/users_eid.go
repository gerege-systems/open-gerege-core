// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"fmt"
	"time"

	"template/pkg/logger"
)

// GetByNationalID нь eID-ийн national_id-ээр хэрэглэгчийг буцаана. GetByID-тэй
// адил кэшийг алгасна (national_id-ээр индекслэсэн кэш байхгүй).
func (uc *usecase) GetByNationalID(ctx context.Context, req GetByNationalIDRequest) (resp GetByNationalIDResponse, err error) {
	const (
		usecaseName = "users"
		funcName    = "GetByNationalID"
		fileName    = "users_eid.go"
	)
	startTime := time.Now()

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
		"request": logger.Fields{"has_national_id": req.NationalID != ""},
	})
	defer func() {
		fields := logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		}
		if err == nil {
			fields["response"] = logger.Fields{"user_id": resp.User.ID}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	user, repoErr := uc.repo.GetByNationalID(ctx, req.NationalID)
	if repoErr != nil {
		err = mapRepoError(repoErr, "get user by national_id")
		logger.ErrorWithContext(ctx, "Get user by national_id failed: repository error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "repo_get_by_national_id", "error": repoErr.Error(),
		})
		return GetByNationalIDResponse{}, err
	}
	resp = GetByNationalIDResponse{User: user}
	return resp, nil
}

// UpsertFromEID нь eID identity-аас (domain.NewEIDUser-ээр бүтээгдсэн) хэрэглэгчийг
// repository-ийн ON CONFLICT upsert-ээр үүсгэх/шинэчилж, хадгалагдсан мөрийг
// буцаана. national_id давхцвал хуучин хэрэглэгчийг (ижил ID-тэй) шинэчилнэ.
func (uc *usecase) UpsertFromEID(ctx context.Context, req UpsertFromEIDRequest) (resp UpsertFromEIDResponse, err error) {
	const (
		usecaseName = "users"
		funcName    = "UpsertFromEID"
		fileName    = "users_eid.go"
	)
	startTime := time.Now()
	in := req.User

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
		"request": logger.Fields{"username": in.Username, "role_id": in.RoleID},
	})
	defer func() {
		fields := logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		}
		if err == nil {
			fields["response"] = logger.Fields{"user_id": resp.User.ID}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	stored, repoErr := uc.repo.UpsertFromEID(ctx, in)
	if repoErr != nil {
		err = mapRepoError(repoErr, "upsert eid user")
		logger.ErrorWithContext(ctx, "Upsert eID user failed: repository error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "repo_upsert_from_eid", "error": repoErr.Error(),
		})
		return UpsertFromEIDResponse{}, err
	}
	resp = UpsertFromEIDResponse{User: stored}
	return resp, nil
}

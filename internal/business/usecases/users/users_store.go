// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/logger"
)

// Store нь шинэ domain.User (email-ийг нормчилж, нууц үгийг hash хийж,
// CreatedAt-ийг тэмдэглэдэг — бүгд нэг газар) үүсгэж, оруулна.
// Repo-гийн INSERT … RETURNING нь хадгалсан мөрийг нэг алхамд (round-trip)
// өгдөг тул дуудагч дараагийн уншилт хийлгүйгээр өгөгдлийн сангийн үүсгэсэн
// ID-г авдаг.
//
// req.User-ийг бүртгэлийн талбаруудын DTO гэж үздэгийг анхаар; бид үүнийг
// өөрчлөхгүй бөгөөд түүний hash/CreatedAt-д итгэдэггүй — domain.NewUser нь
// хүчинтэй User үүсгэдэг цорын ганц зам юм.
func (uc *usecase) Store(ctx context.Context, req StoreRequest) (resp StoreResponse, err error) {
	const (
		usecaseName = "users"
		funcName    = "Store"
		fileName    = "users_store.go"
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

	user, buildErr := domain.NewUser(in.Username, in.Email, in.Password, in.RoleID, uc.cfg.BcryptCost)
	if buildErr != nil {
		// Domain-ийн баталгаажуулалтын алдаанууд (хоосон талбарууд) нь
		// хэрэглэгчид харагдах төрлийнх — тэдгээрийг BadRequest болгож гаргана.
		// Бусад зүйл (жишээ нь bcrypt-ийн алдаа) нь дотоод гэмтэл юм.
		if errors.Is(buildErr, domain.ErrEmptyUsername) ||
			errors.Is(buildErr, domain.ErrEmptyEmail) ||
			errors.Is(buildErr, domain.ErrInvalidEmail) ||
			errors.Is(buildErr, domain.ErrEmptyPassword) {
			err = apperror.BadRequest(buildErr.Error())
		} else {
			err = apperror.InternalCause(fmt.Errorf("build user: %w", buildErr))
		}
		logger.ErrorWithContext(ctx, "Store user failed: build error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "domain_new_user",
			"error":   buildErr.Error(),
			"email":   in.Email,
		})
		return StoreResponse{}, err
	}

	// NewUser нь identity/нууц үгийг л барьдаг тул овог/нэрийг (мн+en) энд хуулна.
	user.FirstName = in.FirstName
	user.LastName = in.LastName
	user.FirstNameEn = in.FirstNameEn
	user.LastNameEn = in.LastNameEn

	stored, repoErr := uc.repo.Store(ctx, user)
	if repoErr != nil {
		err = mapRepoError(repoErr, "store user")
		logger.ErrorWithContext(ctx, "Store user failed: repository error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "repo_store",
			"error":   repoErr.Error(),
			"email":   user.Email,
		})
		return StoreResponse{}, err
	}
	resp = StoreResponse{User: stored}
	return resp, nil
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/usecases/users"
	"template/pkg/logger"
)

// Refresh нь өгөгдсөн refresh токеныг баталгаажуулж, шинэ access+refresh хос
// үүсгэж, хуучин jti-г Redis-д хүчингүй болгоно. Аль хэдийн ашигласан refresh
// токеныг дахин тоглуулах (replay) нь амжилтгүй болдог, учир нь хуучин jti-г
// эхэнд нь GetDel-ээр атомаар уншиж-устгадаг. Энэ нь TOCTOU-гийн цоорхойг
// хаана: ижил токентой зэрэгцээ хоёр хүсэлт ирвэл зөвхөн нэг нь jti-г амжид
// хэрэглэж чадах тул нэг л шинэ session үүснэ.
func (uc *usecase) Refresh(ctx context.Context, req RefreshRequest) (resp LoginResponse, err error) {
	const (
		usecaseName = "auth"
		funcName    = "Refresh"
		fileName    = "auth_refresh.go"
	)
	startTime := time.Now()
	refreshToken := req.RefreshToken

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName,
		"method":  funcName,
		"file":    fileName,
		"request": logger.Fields{
			"has_refresh_token": refreshToken != "",
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

	claims, parseErr := uc.jwtService.ParseRefreshToken(refreshToken)
	if parseErr != nil {
		err = apperror.Unauthorized("invalid refresh token")
		logger.ErrorWithContext(ctx, "Refresh failed: invalid token", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "parse_refresh_token",
			"error":   parseErr.Error(),
		})
		return LoginResponse{}, err
	}

	// jti нь сервер талд одоо ч амьд эсэхийг шалгаад тэр дороо хэрэглэнэ
	// (single-use). GetDel нь атомаар уншиж-устгадаг тул зэрэгцээ хоёр
	// хүсэлт ижил токеныг хэрэглэж чадахгүй — зөвхөн нэг нь хоосон бус утга
	// авна, нөгөө нь redis.Nil/хоосон утгатай тулж татгалзана. Logout /
	// өмнөх эргэлт мөн энэ jti-г устгасан байх ёстой.
	if consumed, getDelErr := uc.redisCache.GetDel(ctx, RefreshKey(claims.ID)); getDelErr != nil || consumed == "" {
		err = apperror.Unauthorized("refresh token has been revoked")
		errMsg := "token already used or not found"
		if getDelErr != nil {
			errMsg = getDelErr.Error()
		}
		logger.ErrorWithContext(ctx, "Refresh failed: token revoked", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "consume_jti",
			"error":   errMsg,
			"jti":     claims.ID,
		})
		return LoginResponse{}, err
	}

	// Хүчингүй болгосон / идэвхгүйжүүлсэн бүртгэлүүд refresh нь амьд байсан ч
	// шинэ access токен авахаа болихын тулд identity-г шинээр хайна.
	lookupResp, lookupErr := uc.users.GetByEmail(ctx, users.GetByEmailRequest{Email: claims.Email})
	if lookupErr != nil {
		err = apperror.Unauthorized("user no longer exists")
		logger.ErrorWithContext(ctx, "Refresh failed: user lookup error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "get_user_by_email",
			"error":   lookupErr.Error(),
			"email":   claims.Email,
		})
		return LoginResponse{}, err
	}
	user := lookupResp.User
	if !user.Active {
		err = apperror.Forbidden("account is not activated")
		logger.ErrorWithContext(ctx, "Refresh failed: account not activated", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "check_active",
			"error":   err.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	// Хамгийн сүүлийн нууц үг солихоос өмнө (эсвэл яг тэр секундэд) олгогдсон
	// токенуудыг татгалз — нууц үг эргүүлэх нь өмнө байсан session-уудыг хаах
	// ёстой. JWT IssuedAt нь секунд хүртэл бутархайгүй болгогддог тул нууц үг
	// солихтой нэг секундэд олгогдсон токеныг алгасахгүйн тулд "After биш"
	// (issued <= cutoff) семантик ашиглана.
	if cutoff := user.TokensRevokedBefore(); !cutoff.IsZero() &&
		claims.IssuedAt != nil && !claims.IssuedAt.After(cutoff) {
		err = apperror.Unauthorized("refresh token has been revoked")
		logger.ErrorWithContext(ctx, "Refresh failed: token issued before password rotation", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "check_revocation_cutoff",
			"error":   err.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	pair, mintErr := uc.jwtService.GenerateTokenPair(user.ID, user.IsAdmin(), user.RoleID, user.Email)
	if mintErr != nil {
		err = apperror.InternalCause(fmt.Errorf("generate token: %w", mintErr))
		logger.ErrorWithContext(ctx, "Refresh failed: token generation error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "generate_token_pair",
			"error":   mintErr.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	// Эргүүлэх: хуучин jti-г аль хэдийн дээр GetDel-ээр устгасан тул энд
	// зөвхөн шинэ хосыг бүртгэнэ.
	if persistErr := uc.rememberRefresh(ctx, pair); persistErr != nil {
		err = apperror.InternalCause(fmt.Errorf("persist refresh: %w", persistErr))
		logger.ErrorWithContext(ctx, "Refresh failed: persist refresh error", logger.Fields{
			"usecase": usecaseName,
			"method":  funcName,
			"file":    fileName,
			"step":    "persist_refresh",
			"error":   persistErr.Error(),
			"user_id": user.ID,
		})
		return LoginResponse{}, err
	}

	resp = LoginResponse{
		User:         user,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}
	return resp, nil
}

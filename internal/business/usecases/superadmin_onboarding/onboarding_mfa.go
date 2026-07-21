// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/datasources/rls"
	"template/pkg/logger"
	"template/pkg/recovery"
	"template/pkg/totp"
)

// SuperadminMFA нь MFA-тай super admin нэвтрэлтийн 2 дахь (сүүлийн) шат.
//
// auth.GoogleLogin / auth.EIDPoll нь ийм хэрэглэгчид session ОЛГОЛГҮЙ зөвхөн
// богино хугацааны mfa_token буцаадаг; энэ method тэр токеныг TOTP код ЭСВЭЛ
// нөөц кодоор баталгаажуулж session олгоно. Ингэснээр super admin-ий олгогдсон
// АЛИВАА session нь угаасаа MFA-баталгаажсан байна (JWT claim өөрчлөх
// шаардлагагүй).
//
// Аюулгүй байдал: mfa_token нь GetDel-гүйгээр уншигдана (буруу код оруулахад
// дахин оролдох боломжтой байх), гэхдээ токен тус бүрийн буруу оролдлогыг
// тоолж MFAMaxAttempts-д хүрмэгц токеныг устгана (TOTP нь 6 орон тул
// brute-force-оос хамгаалах шаардлагатай). Амжилттай болмогц токен устана
// (нэг удаагийн).
func (uc *usecase) SuperadminMFA(ctx context.Context, req MFARequest) (MFAResponse, error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "SuperadminMFA"
		fileName    = "onboarding_mfa.go"
	)

	if req.MFAToken == "" || req.Code == "" {
		return MFAResponse{}, apperror.BadRequest("mfa_token болон code шаардлагатай")
	}

	// Redis алдаа/байхгүй токен → fail-closed (нэвтрүүлэхгүй).
	mfaKey := auth.SuperadminMFAKey(req.MFAToken)
	userID, getErr := uc.redisCache.Get(ctx, mfaKey)
	if getErr != nil || userID == "" {
		logger.WarnWithContext(ctx, "superadmin MFA: токен хүчингүй/хугацаа дууссан", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "has_error": getErr != nil,
		})
		return MFAResponse{}, apperror.Forbidden("Нэвтрэлтийн session хүчингүй эсвэл хугацаа нь дууссан байна. Дахин нэвтэрнэ үү.")
	}

	// Оролдлогын тоологч — токен тус бүрд. Хэтэрвэл токеныг устгаж, дахин
	// нэвтрэхийг шаардана.
	attemptsKey := MFAAttemptsKey(req.MFAToken)
	attempts, incrErr := uc.redisCache.Incr(ctx, attemptsKey)
	if incrErr != nil {
		logger.ErrorWithContext(ctx, "superadmin MFA: оролдлого тоолох амжилтгүй (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": incrErr.Error(),
		})
	} else if attempts == 1 {
		_ = uc.redisCache.Expire(ctx, attemptsKey, superadminMFAAttemptsTTL)
	}
	if attempts > int64(uc.cfg.MFAMaxAttempts) {
		_ = uc.redisCache.Del(ctx, mfaKey)
		_ = uc.redisCache.Del(ctx, attemptsKey)
		logger.WarnWithContext(ctx, "superadmin MFA: оролдлого хэтэрлээ — токен цуцлагдлаа", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"attempts": attempts, "user_id": userID,
		})
		return MFAResponse{}, apperror.Forbidden("Хэт олон буруу оролдлого — дахин нэвтэрнэ үү")
	}

	// Хэрэглэгчийг service RLS дор уншина (хараахан нэвтрээгүй).
	sctx := rls.WithService(ctx)
	user, userErr := uc.users.GetByID(sctx, userID)
	if userErr != nil {
		return MFAResponse{}, userErr
	}
	// Super admin-ы MFA бүртгэл (TOTP secret) нь superadmin_accounts satellite-д —
	// эндээс уншина. Токен олгогдсоноос хойш эрх/MFA өөрчлөгдсөн байж болзошгүй тул
	// super admin эсэх + MFA идэвхтэй эсэхийг дахин шалгана (account алга бол fail-closed).
	account, acctErr := uc.superadminAccts.Get(sctx, userID)
	if !user.IsSuperAdmin() || acctErr != nil || !account.MFAEnabled {
		logger.WarnWithContext(ctx, "superadmin MFA: хэрэглэгч super admin биш эсвэл MFA идэвхгүй", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "user_id": user.ID,
			"has_acct_error": acctErr != nil,
		})
		_ = uc.redisCache.Del(ctx, mfaKey)
		return MFAResponse{}, apperror.Forbidden("Энэ нэвтрэлт боломжгүй байна")
	}

	usedRecovery, verifyErr := uc.verifyMFACode(sctx, user, account.TOTPSecret, req.Code)
	if verifyErr != nil {
		return MFAResponse{}, verifyErr
	}

	// Амжилттай — токен нэг удаагийн тул устгана.
	_ = uc.redisCache.Del(ctx, mfaKey)
	_ = uc.redisCache.Del(ctx, attemptsKey)

	pair, mintErr := uc.mintSession(ctx, user)
	if mintErr != nil {
		return MFAResponse{}, mintErr
	}

	left := 0
	if usedRecovery {
		// Нөөц кодоор нэвтэрсэн бол үлдсэн тоог мэдэгдэнэ (UI сануулга).
		if remaining, listErr := uc.recovery.ListActive(sctx, user.ID); listErr == nil {
			left = len(remaining)
		}
		logger.WarnWithContext(ctx, "superadmin MFA: нөөц кодоор нэвтэрлээ", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"user_id": user.ID, "codes_left": left,
		})
	}

	return MFAResponse{
		User:              user,
		AccessToken:       pair.AccessToken,
		RefreshToken:      pair.RefreshToken,
		RecoveryCodesLeft: left,
		UsedRecoveryCode:  usedRecovery,
	}, nil
}

// verifyMFACode нь кодыг эхлээд TOTP-ээр (хадгалсан шифрлэгдсэн secret-ийг
// тайлж), тэр нь таарахгүй бол нөөц кодоор (SHA-256 тулгалт, нэг удаагийн)
// шалгана. usedRecovery нь нөөц код хэрэглэгдсэн эсэхийг илэрхийлнэ.
func (uc *usecase) verifyMFACode(ctx context.Context, user domain.User, totpSecretEnc, code string) (usedRecovery bool, err error) {
	if totpSecretEnc != "" {
		secret, decErr := uc.cipher.Decrypt(totpSecretEnc)
		if decErr != nil {
			// Шифр тайлагдахгүй бол түлхүүр солигдсон/өгөгдөл эвдэрсэн —
			// нөөц кодоор нэвтрэх боломж үлдэнэ тул энд зогсохгүй.
			logger.ErrorWithContext(ctx, "superadmin MFA: TOTP secret тайлах амжилтгүй", logger.Fields{
				"usecase": "superadmin_onboarding", "method": "verifyMFACode",
				"file": "onboarding_mfa.go", "error": decErr.Error(), "user_id": user.ID,
			})
		} else if totp.Validate(code, secret) {
			return false, nil
		}
	}

	// TOTP таарсангүй — нөөц код эсэхийг шалгана (нэг удаагийн, атомаар).
	consumeErr := uc.recovery.Consume(ctx, user.ID, recovery.Hash(code))
	if consumeErr == nil {
		return true, nil
	}
	var domErr *apperror.DomainError
	if errors.As(consumeErr, &domErr) && domErr.Type == apperror.ErrTypeNotFound {
		// Аль нь ч биш — цөөн үгтэй, тоочих (enumeration) боломжгүй мессеж.
		return false, apperror.BadRequest("Баталгаажуулах код буруу байна")
	}
	return false, apperror.InternalCause(fmt.Errorf("consume recovery code: %w", consumeErr))
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/rls"
	"template/pkg/logger"
	"template/pkg/recovery"
	"template/pkg/totp"
)

// TOTPInit нь шидтэний 4 дэх алхам: шинэ TOTP secret үүсгэж pending session-д
// хадгалаад, authenticator app-д уншуулах otpauth:// URI-г буцаана (QR-г
// frontend зурна). Дахин дуудвал ШИНЭ secret үүснэ — хэрэглэгч QR-аа
// алдсан/буруу уншуулсан бол дахин эхлэх боломжтой.
//
// АНХААР: энд үүссэн secret нь ХАРААХАН идэвхжээгүй — зөвхөн TOTPVerify
// амжилттай болоход л (хэрэглэгч app-аа зөв тохируулсны баталгаа) шифрлэгдэж
// DB-д бичигдэнэ.
func (uc *usecase) TOTPInit(ctx context.Context, req TokenRequest) (TOTPInitResponse, error) {
	sess, err := uc.loadPending(ctx, req.OnboardToken)
	if err != nil {
		return TOTPInitResponse{}, err
	}
	if err := requireStep(sess, StepTOTP); err != nil {
		return TOTPInitResponse{}, err
	}

	secret, url, genErr := totp.Generate(uc.cfg.Issuer, sess.Email)
	if genErr != nil {
		return TOTPInitResponse{}, apperror.InternalCause(fmt.Errorf("generate totp secret: %w", genErr))
	}
	sess.PendingTOTPSecret = secret
	if err := uc.savePending(ctx, req.OnboardToken, sess); err != nil {
		return TOTPInitResponse{}, err
	}
	return TOTPInitResponse{Secret: secret, OtpauthURL: url, Step: StepTOTP}, nil
}

// TOTPVerify нь authenticator app-ийн кодыг secret-тэй тулгаж, амжилттай бол
// бүртгэлийг ТӨГСГӨНӨ (finalize):
//
//  1. super admin хэрэглэгчийг service RLS дор upsert (Google + eID + и-мэйл,
//     email_verified, mfa_enabled, ШИФРЛЭГДСЭН totp_secret, RoleSuperAdmin);
//  2. нөөц кодуудыг үүсгэж SHA-256 hash-аар хадгална;
//  3. урилгыг accepted болгож (дахин ашиглагдахгүй);
//  4. pending session-ийг устгаж, session (токен хос) олгоно.
//
// Энгийн текст нөөц кодууд ЗӨВХӨН энэ хариунд, ЗӨВХӨН НЭГ УДАА буцна — дахин
// авах ямар ч зам байхгүй (DB-д зөвхөн hash).
func (uc *usecase) TOTPVerify(ctx context.Context, req TOTPVerifyRequest) (FinalizeResponse, error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "TOTPVerify"
		fileName    = "onboarding_totp.go"
	)

	sess, err := uc.loadPending(ctx, req.OnboardToken)
	if err != nil {
		return FinalizeResponse{}, err
	}
	if err := requireStep(sess, StepTOTP); err != nil {
		return FinalizeResponse{}, err
	}
	if sess.PendingTOTPSecret == "" {
		return FinalizeResponse{}, apperror.BadRequest("Эхлээд TOTP тохируулгыг эхлүүлнэ үү")
	}
	// Урьдчилсан алхмуудын баталгаа — pending session эвдэрсэн ч алхам алгасахгүй.
	if !sess.EmailVerified || sess.CivilID == "" || sess.GoogleSub == "" {
		return FinalizeResponse{}, apperror.BadRequest("Бүртгэлийн алхмууд дутуу байна. Дахин эхлүүлнэ үү.")
	}
	if !totp.Validate(req.Code, sess.PendingTOTPSecret) {
		logger.WarnWithContext(ctx, "superadmin onboarding: TOTP код буруу", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
		})
		return FinalizeResponse{}, apperror.BadRequest("Баталгаажуулах код буруу байна")
	}

	// TOTP secret нь storage-д ил текстээр ХЭЗЭЭ Ч бичигдэхгүй (AES-256-GCM).
	encSecret, encErr := uc.cipher.Encrypt(sess.PendingTOTPSecret)
	if encErr != nil {
		return FinalizeResponse{}, apperror.InternalCause(fmt.Errorf("encrypt totp secret: %w", encErr))
	}

	// Super admin-ы users мөр — Google/email-ээр түлхүүрлэнэ, civil_id-г users-д
	// ТАВИХГҮЙ (нэг хүн eID-ээр admin, Google-оор super admin байж чадахын тулд).
	// eID баталгаа (civil_id/national_id) болон MFA нь superadmin_accounts-д очно.
	// username нь "sa_<civil_id>" — админы "eid_<civil_id>" мөрөөс ялгаатай, давхардахгүй.
	newUser := &domain.User{
		Username:            "sa_" + sess.CivilID,
		FirstName:           sess.FirstName,
		LastName:            sess.LastName,
		FirstNameEn:         sess.FirstNameEn,
		LastNameEn:          sess.LastNameEn,
		Email:               sess.Email,
		Active:              true,
		RoleID:              domain.RoleSuperAdmin,
		KYCLevel:            sess.KYCLevel,
		GoogleSub:           sess.GoogleSub,
		GoogleEmail:         sess.Email,
		GoogleEmailVerified: sess.GoogleEmailVerified,
		GoogleName:          sess.Name,
		GooglePicture:       sess.Picture,
	}
	account := &domain.SuperadminAccount{
		CivilID:       sess.CivilID,
		NationalID:    sess.NationalID,
		EmailVerified: true,
		MFAEnabled:    true,
		TOTPSecret:    encSecret,
	}

	// Хэрэглэгч хараахан нэвтрээгүй тул бичилт нь service RLS дор явна.
	sctx := rls.WithService(ctx)
	user, upsertErr := uc.users.UpsertSuperAdmin(sctx, newUser, account)
	if upsertErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding: upsert амжилтгүй", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": upsertErr.Error(),
		})
		return FinalizeResponse{}, upsertErr
	}

	// Нөөц кодууд — энгийн текстийг нэг удаа буцааж, зөвхөн hash-ийг хадгална.
	codes, codesErr := recovery.Generate(uc.cfg.RecoveryCodeCount)
	if codesErr != nil {
		return FinalizeResponse{}, apperror.InternalCause(fmt.Errorf("generate recovery codes: %w", codesErr))
	}
	if err := uc.recovery.Replace(sctx, user.ID, recovery.HashAll(codes)); err != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding: нөөц код хадгалах амжилтгүй", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"error": err.Error(), "user_id": user.ID,
		})
		return FinalizeResponse{}, err
	}

	// Урилгыг ашигласан гэж тэмдэглэнэ. Хэрэглэгч аль хэдийн үүссэн тул энэ
	// алдаа нь нэвтрэлтийг эвдэхгүй (best-effort) — зөвхөн лог.
	if err := uc.invites.MarkAccepted(ctx, sess.Email); err != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding: урилгыг accepted болгож чадсангүй (non-fatal)", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"error": err.Error(), "user_id": user.ID,
		})
	}

	// Шидтэн дууссан — pending session (ил текст TOTP secret агуулсан) устгана.
	_ = uc.redisCache.Del(ctx, OnboardKey(req.OnboardToken))

	pair, mintErr := uc.mintSession(ctx, user)
	if mintErr != nil {
		return FinalizeResponse{}, mintErr
	}

	logger.InfoWithContext(ctx, "superadmin onboarding төгслөө — шинэ super admin үүслээ", logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName, "user_id": user.ID,
	})
	return FinalizeResponse{
		User:          user,
		AccessToken:   pair.AccessToken,
		RefreshToken:  pair.RefreshToken,
		RecoveryCodes: codes,
		Step:          StepDone,
	}, nil
}

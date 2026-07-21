// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/logger"
)

// Google нь шидтэний 1 дэх алхам: OAuth code-ийг Google профайл руу солиж,
// и-мэйлийг superadmin_invites-ийн эсрэг шалгана.
//
// Хаалт (gate): урилга байхгүй ЭСВЭЛ аль хэдийн ашиглагдсан бол Forbidden —
// энэ нь super admin болох ЦОРЫН ГАНЦ хаалга (өөр бүх алхам үүнээс үүссэн
// pending session шаарддаг). Google-ийн и-мэйл баталгаажаагүй бол мөн
// татгалзана (баталгаажаагүй и-мэйлээр урилгын allow-list-ыг тойрч болохгүй).
func (uc *usecase) Google(ctx context.Context, req GoogleRequest) (resp GoogleResponse, err error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "Google"
		fileName    = "onboarding_google.go"
	)

	if uc.google == nil || !uc.google.Configured() {
		return GoogleResponse{}, apperror.InternalCause(fmt.Errorf("google login not configured"))
	}

	gu, exErr := uc.google.Exchange(ctx, req.Code, req.RedirectURI)
	if exErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding failed: token exchange", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": exErr.Error(),
		})
		return GoogleResponse{}, apperror.BadRequest("Google нэвтрэлт амжилтгүй боллоо")
	}

	email := domain.NormalizeInviteEmail(gu.Email)
	if email == "" {
		return GoogleResponse{}, apperror.BadRequest("Google бүртгэлээс и-мэйл авч чадсангүй")
	}
	// Баталгаажаагүй Google и-мэйлээр урилгын allow-list-ыг тойрох боломжгүй.
	if !gu.EmailVerified {
		logger.WarnWithContext(ctx, "superadmin onboarding: Google и-мэйл баталгаажаагүй", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
		})
		return GoogleResponse{}, apperror.Forbidden("Google бүртгэлийн и-мэйл баталгаажаагүй байна")
	}

	// Урилгын шалгалт — энэ бол бүртгэлийн ханын хаалга.
	invite, invErr := uc.invites.GetByEmail(ctx, email)
	if invErr != nil {
		var domErr *apperror.DomainError
		if errors.As(invErr, &domErr) && domErr.Type == apperror.ErrTypeNotFound {
			logger.WarnWithContext(ctx, "superadmin onboarding: урилгагүй и-мэйл", logger.Fields{
				"usecase": usecaseName, "method": funcName, "file": fileName,
			})
			return GoogleResponse{}, apperror.Forbidden("Энэ и-мэйл super admin болох урилга аваагүй байна")
		}
		return GoogleResponse{}, invErr
	}
	if invite.Accepted() {
		logger.WarnWithContext(ctx, "superadmin onboarding: урилга аль хэдийн ашиглагдсан", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
		})
		return GoogleResponse{}, apperror.Forbidden("Энэ урилга аль хэдийн ашиглагдсан байна")
	}

	token, tErr := newOnboardToken()
	if tErr != nil {
		return GoogleResponse{}, apperror.InternalCause(fmt.Errorf("onboard token: %w", tErr))
	}
	// АНХААР: и-мэйлийг Google-ийн буцаасан утгаас биш, УРИЛГЫН мөрөөс авна —
	// цаашдын бүх алхам (OTP илгээх, хэрэглэгч үүсгэх) урьсан и-мэйл дээр л
	// ажиллана.
	sess := pendingSession{
		GoogleSub:           gu.Sub,
		Email:               invite.Email,
		Name:                gu.Name,
		Picture:             gu.Picture,
		GoogleEmailVerified: gu.EmailVerified,
		Step:                StepEID,
	}
	if err := uc.savePending(ctx, token, sess); err != nil {
		return GoogleResponse{}, err
	}

	logger.InfoWithContext(ctx, "superadmin onboarding эхэллээ (Google баталгаажлаа)", logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName, "step": StepEID,
	})
	return GoogleResponse{OnboardToken: token, Email: invite.Email, Step: StepEID}, nil
}

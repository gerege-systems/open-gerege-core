// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"fmt"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
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

	return uc.beginFromIdentity(ctx, verifiedIdentity{
		Sub:           gu.Sub,
		Email:         email,
		Name:          gu.Name,
		Picture:       gu.Picture,
		EmailVerified: gu.EmailVerified,
	})
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"fmt"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
)

// SSO нь шидтэний 1 дэх алхмын SSO хувилбар: төвийн SSO (sso.gerege.mn)-оос
// ирсэн code-ийг сольж, баталгаажсан и-мэйлээр урилгын хаалгыг дамжина.
//
// ЯАГААД GOOGLE-ИЙН ОРОНД: Google-ийн шууд урсгал платформ бүрээс өөрийн
// redirect URI-г илгээдэг тул шинэ платформ нэмэх бүрд Google Console-д
// мөр нэмэхийг шаарддаг. SSO нь тэр бүртгэлийг НЭГ газар (SSO хост) төвлөрүүлж,
// платформууд түүний RP болно — платформ бүрийн SSO redirect URI аль хэдийн
// бүртгэгдсэн байдаг.
//
// Урилгын хаалга, и-мэйл баталгаажилтын шаардлага, дараагийн бүх алхам
// Google замтай ЯГ ИЖИЛ (beginFromIdentity хуваалцана).
func (uc *usecase) SSO(ctx context.Context, req SSORequest) (GoogleResponse, error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "SSO"
		fileName    = "onboarding_sso.go"
	)

	if uc.ssoClient == nil || !uc.ssoClient.Configured() {
		return GoogleResponse{}, apperror.InternalCause(fmt.Errorf("sso login not configured"))
	}

	accessToken, _, exErr := uc.ssoClient.Exchange(ctx, req.Code)
	if exErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding failed: sso token exchange", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": exErr.Error(),
		})
		return GoogleResponse{}, apperror.BadRequest("SSO нэвтрэлт амжилтгүй боллоо")
	}

	// Claims-ыг ID token-оос задлахын оронд userinfo-гоос авна: SSO нь
	// и-мэйлийг заавал ID token-д тавьдаггүй (scope-оос хамаарна), userinfo
	// нь access token-оор баталгаажсан эх сурвалж.
	ui, uiErr := uc.ssoClient.UserInfo(ctx, accessToken)
	if uiErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding failed: sso userinfo", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": uiErr.Error(),
		})
		return GoogleResponse{}, apperror.BadRequest("SSO профайл уншиж чадсангүй")
	}

	return uc.beginFromIdentity(ctx, verifiedIdentity{
		Sub:           ui.Sub,
		Email:         domain.NormalizeInviteEmail(ui.Email),
		Name:          ui.Name,
		Picture:       ui.GooglePicture,
		EmailVerified: ui.EmailVerified,
	})
}

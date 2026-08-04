// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"errors"
	"fmt"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
)

// verifiedIdentity нь шидтэний 1 дэх алхмын ҮР ДҮН: аль IdP-ээс ирснээс
// үл хамааран баталгаажсан хэн болох нь.
type verifiedIdentity struct {
	Sub           string
	Email         string
	Name          string
	Picture       string
	EmailVerified bool
}

// beginFromIdentity нь бүртгэлийн ХАНЫН ХААЛГА — Google, SSO аль ч замаар
// ирсэн байсан ижил дүрэм үйлчилнэ:
//
//   - и-мэйл баталгаажаагүй бол татгалзана (баталгаажаагүй и-мэйлээр
//     урилгын allow-list-ыг тойрч болохгүй);
//   - урилга байхгүй ЭСВЭЛ ашиглагдсан бол Forbidden.
//
// Хоёр IdP-д хуваалцсан шалтгаан: энэ бол super admin болох цорын ганц
// хаалга. Хоёр газар хуулбарлавал нэгийг нь чангатгаад нөгөөг нь мартах
// эрсдэл үүснэ.
func (uc *usecase) beginFromIdentity(ctx context.Context, id verifiedIdentity) (GoogleResponse, error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "beginFromIdentity"
		fileName    = "onboarding_identity.go"
	)

	if id.Email == "" {
		return GoogleResponse{}, apperror.BadRequest("Бүртгэлээс и-мэйл авч чадсангүй")
	}
	if !id.EmailVerified {
		logger.WarnWithContext(ctx, "superadmin onboarding: и-мэйл баталгаажаагүй", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
		})
		return GoogleResponse{}, apperror.Forbidden("Бүртгэлийн и-мэйл баталгаажаагүй байна")
	}

	invite, invErr := uc.invites.GetByEmail(ctx, id.Email)
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
	// АНХААР: и-мэйлийг IdP-ийн буцаасан утгаас биш, УРИЛГЫН мөрөөс авна —
	// цаашдын бүх алхам урьсан и-мэйл дээр л ажиллана.
	sess := pendingSession{
		GoogleSub:           id.Sub,
		Email:               invite.Email,
		Name:                id.Name,
		Picture:             id.Picture,
		GoogleEmailVerified: id.EmailVerified,
		Step:                StepEID,
	}
	if err := uc.savePending(ctx, token, sess); err != nil {
		return GoogleResponse{}, err
	}

	logger.InfoWithContext(ctx, "superadmin onboarding эхэллээ", logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName, "step": StepEID,
	})
	return GoogleResponse{OnboardToken: token, Email: invite.Email, Step: StepEID}, nil
}

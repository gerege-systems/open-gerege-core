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
			// ПЛАТФОРМЫН АНХНЫ АЖИЛЛУУЛАЛТ: super admin огт байхгүй бол
			// урилга өгөх хүн ч байхгүй — тахиа/өндөгний нөхцөл. Ийм үед
			// бүртгэлээ гүйцээсэн ЭХНИЙ хүн super admin болно.
			//
			// Энэ нь хаалгыг сулруулж БАЙГАА тул зөвхөн ҮНЭХЭЭР хэн ч
			// байхгүй үед нээгдэнэ; нэг super admin үүсмэгц дахин урилга
			// заавал болно. Үлдсэн бүх алхам (eID, и-мэйл OTP, TOTP) хэвээр
			// — өөрөөр хэлбэл иргэний eID баталгаажуулалтгүйгээр орох
			// боломжгүй.
			exists, exErr := uc.superadminAccts.AnySuperAdminExists(ctx)
			if exErr != nil {
				return GoogleResponse{}, exErr
			}
			if !exists {
				logger.WarnWithContext(ctx, "superadmin onboarding: BOOTSTRAP — super admin алга тул урилгагүйгээр зөвшөөрөв", logger.Fields{
					"usecase": usecaseName, "method": funcName, "file": fileName,
					"email": id.Email,
				})
				return uc.startPending(ctx, id, id.Email)
			}
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

	// АНХААР: и-мэйлийг IdP-ийн буцаасан утгаас биш, УРИЛГЫН мөрөөс авна —
	// цаашдын бүх алхам урьсан и-мэйл дээр л ажиллана.
	return uc.startPending(ctx, id, invite.Email)
}

// startPending нь баталгаажсан хэн болохоос шидтэний pending session үүсгэнэ.
// email нь урилгын мөрөөс (энгийн зам) эсвэл IdP-ээс (bootstrap зам) ирнэ.
func (uc *usecase) startPending(ctx context.Context, id verifiedIdentity, email string) (GoogleResponse, error) {
	token, tErr := newOnboardToken()
	if tErr != nil {
		return GoogleResponse{}, apperror.InternalCause(fmt.Errorf("onboard token: %w", tErr))
	}
	sess := pendingSession{
		GoogleSub:           id.Sub,
		Email:               email,
		Name:                id.Name,
		Picture:             id.Picture,
		GoogleEmailVerified: id.EmailVerified,
		Step:                StepEID,
	}
	if err := uc.savePending(ctx, token, sess); err != nil {
		return GoogleResponse{}, err
	}
	logger.InfoWithContext(ctx, "superadmin onboarding эхэллээ", logger.Fields{
		"usecase": "superadmin_onboarding", "method": "startPending",
		"file": "onboarding_identity.go", "step": StepEID,
	})
	return GoogleResponse{OnboardToken: token, Email: email, Step: StepEID}, nil
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"template/internal/apperror"
	"template/pkg/eid"
	"template/pkg/logger"
)

// eidPollTimeoutMs нь IdP-ийн session long-poll-ийн хүлээх дээд хугацаа (мс) —
// auth-ийн урсгалтай ижил (eid client-ийн HTTP timeout 30с-ээс богино).
const eidPollTimeoutMs = 25000

// EIDStart нь шидтэний 2 дахь алхмыг QR/deep-link-ээр эхлүүлнэ. callbackURL
// хоосон бол CROSS-DEVICE (desktop QR); хоосон биш бол SAME-DEVICE (mobile).
func (uc *usecase) EIDStart(ctx context.Context, req EIDStartRequest) (EIDStartResponse, error) {
	sess, err := uc.loadPending(ctx, req.OnboardToken)
	if err != nil {
		return EIDStartResponse{}, err
	}
	if err := requireStep(sess, StepEID); err != nil {
		return EIDStartResponse{}, err
	}

	nonce, nErr := randomNonce()
	if nErr != nil {
		return EIDStartResponse{}, apperror.InternalCause(fmt.Errorf("generate nonce: %w", nErr))
	}
	start, initErr := uc.eid.QRInitiate(ctx, uc.cfg.EIDDisplayText, req.CallbackURL, nonce)
	if initErr != nil {
		return EIDStartResponse{}, mapInitiateErr(initErr, "eID session эхлүүлэх боломжгүй байна")
	}
	return EIDStartResponse{
		SessionID:        start.SessionID,
		DeviceLinkURL:    start.DeviceLinkURL,
		VerificationCode: start.VerificationCode,
		ExpiresAt:        start.ExpiresAt,
	}, nil
}

// EIDStartByNationalID нь шидтэний eID алхмыг иргэний РД-аар (бүртгэлтэй
// төхөөрөмж рүү push) эхлүүлнэ — device_link шаардлагагүй.
func (uc *usecase) EIDStartByNationalID(ctx context.Context, req EIDStartByNationalIDRequest) (EIDStartResponse, error) {
	sess, err := uc.loadPending(ctx, req.OnboardToken)
	if err != nil {
		return EIDStartResponse{}, err
	}
	if err := requireStep(sess, StepEID); err != nil {
		return EIDStartResponse{}, err
	}

	nationalID := strings.TrimSpace(req.NationalID)
	if nationalID == "" {
		return EIDStartResponse{}, apperror.BadRequest("national_id is required")
	}
	start, initErr := uc.eid.Initiate(ctx, nationalID, uc.cfg.EIDDisplayText, req.CallbackURL)
	if initErr != nil {
		return EIDStartResponse{}, mapInitiateErr(initErr, "Регистрийн дугаар олдсонгүй эсвэл буруу байна")
	}
	return EIDStartResponse{
		SessionID:        start.SessionID,
		VerificationCode: start.VerificationCode,
		ExpiresAt:        start.ExpiresAt,
	}, nil
}

// EIDPoll нь eID session-ийн төлвийг long-poll-оор асууна. COMPLETE болоход
// identity-г pending session-д БАРИНА.
//
// АНХААР: энэ алхамд хэрэглэгч ҮҮСГЭХГҮЙ, session ОЛГОХГҮЙ (auth.EIDPoll-оос
// үндсэн ялгаа нь энэ) — eID нь зөвхөн "энэ и-мэйлийг урьсан хүн бодитоор хэн
// бэ" гэдгийг тогтоох баталгаажуулалтын алхам. Хэрэглэгч зөвхөн шидтэн бүрэн
// төгсөхөд (TOTP баталгаажсаны дараа) үүснэ.
func (uc *usecase) EIDPoll(ctx context.Context, req EIDPollRequest) (EIDPollResponse, error) {
	const (
		usecaseName = "superadmin_onboarding"
		funcName    = "EIDPoll"
		fileName    = "onboarding_eid.go"
	)

	sess, err := uc.loadPending(ctx, req.OnboardToken)
	if err != nil {
		return EIDPollResponse{}, err
	}
	if err := requireStep(sess, StepEID); err != nil {
		return EIDPollResponse{}, err
	}
	if req.SessionID == "" {
		return EIDPollResponse{}, apperror.BadRequest("session_id is required")
	}

	res, pollErr := uc.eid.Session(ctx, req.SessionID, eidPollTimeoutMs)
	if pollErr != nil {
		logger.ErrorWithContext(ctx, "superadmin onboarding EIDPoll failed: session error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName, "error": pollErr.Error(),
		})
		return EIDPollResponse{}, apperror.InternalCause(fmt.Errorf("eid session: %w", pollErr))
	}
	// Terminal биш (RUNNING) болон terminal-fail (EXPIRED/REFUSED) — зөвхөн төлөв.
	if res.State != eid.StateComplete {
		return EIDPollResponse{State: res.State, Step: StepEID}, nil
	}

	// Public RP-д IdP нь national_id-г илчлэхгүй тул civil_id нь давтагдашгүй
	// түлхүүр; хоосон бол identity дутуу гэж үзэж татгалзана (РД/civil_id-г
	// лог-д бичихгүй — хувийн мэдээлэл).
	var id *eid.Identity
	if res.Identity != nil {
		id = res.Identity
	}
	if id == nil || strings.TrimSpace(id.CivilID) == "" {
		logger.ErrorWithContext(ctx, "superadmin onboarding EIDPoll failed: complete without identity", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"has_identity": res.Identity != nil,
		})
		return EIDPollResponse{}, apperror.InternalCause(fmt.Errorf("eid complete without identity"))
	}

	// Identity-г барьж, дараагийн алхам руу (и-мэйл OTP) шилжинэ.
	sess.CivilID = strings.ToLower(strings.TrimSpace(id.CivilID))
	sess.NationalID = strings.ToLower(strings.TrimSpace(id.NationalID))
	sess.FirstName = strings.TrimSpace(id.GivenName)
	sess.LastName = strings.TrimSpace(id.Surname)
	sess.FirstNameEn = strings.TrimSpace(id.GivenNameEn)
	sess.LastNameEn = strings.TrimSpace(id.SurnameEn)
	sess.KYCLevel = strings.TrimSpace(id.KYCLevel)
	sess.Step = StepEmail
	if err := uc.savePending(ctx, req.OnboardToken, sess); err != nil {
		return EIDPollResponse{}, err
	}

	logger.InfoWithContext(ctx, "superadmin onboarding: eID баталгаажлаа", logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName, "step": StepEmail,
	})
	return EIDPollResponse{State: eid.StateComplete, Step: StepEmail}, nil
}

// randomNonce нь IdP-ийн replay-аас хамгаалах 32 hex тэмдэгтийн nonce үүсгэнэ.
func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// mapInitiateErr нь eID initiate-ийн алдааг HTTP статус руу буулгана: IdP-ийн
// 4xx бол цэвэр BadRequest, бусад (сүлжээ/5xx) бол дотоод алдаа.
func mapInitiateErr(initErr error, clientMsg string) error {
	if errors.Is(initErr, eid.ErrInitiateRejected) {
		return apperror.BadRequest(clientMsg)
	}
	return apperror.InternalCause(fmt.Errorf("eid initiate: %w", initErr))
}

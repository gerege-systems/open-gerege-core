// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package superadminonboard нь /v1/auth/superadmin/* endpoint-уудыг үйлчилнэ:
// урилгаар хаалттай super admin бүртгэлийн шидтэн (Google → eID → и-мэйл OTP →
// TOTP) болон MFA-тай super admin нэвтрэлтийн 2 дахь шат.
//
// Бүгд НЭВТРЭЭГҮЙ (нэвтрэхээс өмнөх) гадаргуу тул route түвшинд rate limiter,
// чанга body хязгаар болон service RLS context-оор хамгаалагдсан. Хаалт нь
// урилгын allow-list (Google алхам) + шидтэний onboard_token дээр тогтоно.
package superadminonboard

import (
	"net/http"

	onboarding "template/internal/business/usecases/superadmin_onboarding"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

// Handler нь super admin бүртгэл + MFA-ийн endpoint-уудыг үйлчилнэ.
type Handler struct {
	usecase onboarding.Usecase
}

func NewHandler(usecase onboarding.Usecase) Handler {
	return Handler{usecase: usecase}
}

// decode нь body-г задалж баталгаажуулна (давхардлыг багасгах туслах).
func decode[T any](w http.ResponseWriter, r *http.Request, dst *T) error {
	if err := v1.DecodeBody(r, dst); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(*dst); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return nil
}

// Google godoc
// @Summary      Super admin бүртгэл — Google алхам
// @Description  Google OAuth code-ийг солиж, и-мэйлийг super admin урилгын allow-list-ийн эсрэг шалгана. Урилгагүй / аль хэдийн ашигласан урилга бол 403. Амжилттай бол шидтэний onboard_token үүсгэж, дараагийн алхмыг (eid) буцаана.
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardGoogleRequest  true  "OAuth code + redirect_uri"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardStartResponse}  "Onboarding started"
// @Failure      400  {object}  v1.BaseResponse  "Invalid code"
// @Failure      403  {object}  v1.BaseResponse  "Email is not invited or invite already used"
// @Router       /v1/auth/superadmin/onboard/google [post]
func (h Handler) Google(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardGoogleRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.Google(r.Context(), onboarding.GoogleRequest{
		Code: req.Code, RedirectURI: req.RedirectURI,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "superadmin onboarding started", responses.FromOnboardGoogle(res))
}

// EIDStart godoc
// @Summary      Super admin бүртгэл — eID эхлүүлэх (QR / deep-link)
// @Description  Бүртгэлийн eID баталгаажуулах алхмыг QR/deep-link-ээр эхлүүлнэ. callbackUrl хоосон бол CROSS-DEVICE (desktop QR); хоосон биш бол SAME-DEVICE (mobile browser).
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardEIDStartRequest  true  "Onboard token"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardEIDStartResponse}  "eID session started"
// @Failure      400  {object}  v1.BaseResponse  "Invalid body or wrong step"
// @Failure      403  {object}  v1.BaseResponse  "Onboard session invalid or expired"
// @Router       /v1/auth/superadmin/onboard/eid/start [post]
func (h Handler) EIDStart(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardEIDStartRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.EIDStart(r.Context(), onboarding.EIDStartRequest{
		OnboardToken: req.OnboardToken, CallbackURL: req.CallbackUrl,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session started", responses.FromOnboardEIDStart(res))
}

// EIDStartByNationalID godoc
// @Summary      Super admin бүртгэл — eID эхлүүлэх (РД-аар push)
// @Description  Бүртгэлийн eID алхмыг иргэний РД-аар эхлүүлж, бүртгэлтэй төхөөрөмж рүү баталгаажуулах push илгээнэ (device_link шаардлагагүй).
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardEIDStartIDRequest  true  "Onboard token + иргэний РД"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardEIDStartResponse}  "eID session started"
// @Failure      400  {object}  v1.BaseResponse  "Invalid national_id or wrong step"
// @Failure      403  {object}  v1.BaseResponse  "Onboard session invalid or expired"
// @Router       /v1/auth/superadmin/onboard/eid/start-id [post]
func (h Handler) EIDStartByNationalID(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardEIDStartIDRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.EIDStartByNationalID(r.Context(), onboarding.EIDStartByNationalIDRequest{
		OnboardToken: req.OnboardToken, NationalID: req.NationalID, CallbackURL: req.CallbackUrl,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session started", responses.FromOnboardEIDStart(res))
}

// EIDPoll godoc
// @Summary      Super admin бүртгэл — eID төлөв асуух (long-poll)
// @Description  eID session-ийн төлвийг long-poll-оор (≤25с) асууна: RUNNING/COMPLETE/EXPIRED/REFUSED. COMPLETE үед identity нь шидтэнд БАРИГДАЖ, алхам "email" болно. АНХААР: энэ алхамд токен ОЛГОГДОХГҮЙ.
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardEIDPollRequest  true  "Onboard token + eID session id"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardEIDPollResponse}  "Session state"
// @Failure      400  {object}  v1.BaseResponse  "Invalid body or wrong step"
// @Failure      403  {object}  v1.BaseResponse  "Onboard session invalid or expired"
// @Router       /v1/auth/superadmin/onboard/eid/poll [post]
func (h Handler) EIDPoll(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardEIDPollRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.EIDPoll(r.Context(), onboarding.EIDPollRequest{
		OnboardToken: req.OnboardToken, SessionID: req.SessionID,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session state", responses.FromOnboardEIDPoll(res))
}

// EmailSend godoc
// @Summary      Super admin бүртгэл — и-мэйл OTP илгээх
// @Description  Урилгын и-мэйл рүү 6 оронтой баталгаажуулах код илгээнэ (хаяг нь шидтэний session-оос авагдана — клиент өөр хаяг заах боломжгүй).
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardTokenRequest  true  "Onboard token"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardStepResponse}  "OTP sent"
// @Failure      400  {object}  v1.BaseResponse  "Wrong step"
// @Failure      403  {object}  v1.BaseResponse  "Onboard session invalid or expired"
// @Router       /v1/auth/superadmin/onboard/email/send [post]
func (h Handler) EmailSend(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardTokenRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.EmailSend(r.Context(), onboarding.TokenRequest{OnboardToken: req.OnboardToken})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "otp sent", responses.FromOnboardStep(res))
}

// EmailVerify godoc
// @Summary      Super admin бүртгэл — и-мэйл OTP баталгаажуулах
// @Description  И-мэйл рүү илгээсэн кодыг шалгана. Амжилттай бол алхам "totp" болно. Хэт олон буруу оролдлого → 403.
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardCodeRequest  true  "Onboard token + код"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardStepResponse}  "Email verified"
// @Failure      400  {object}  v1.BaseResponse  "Invalid or expired code"
// @Failure      403  {object}  v1.BaseResponse  "Onboard session invalid / too many attempts"
// @Router       /v1/auth/superadmin/onboard/email/verify [post]
func (h Handler) EmailVerify(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardCodeRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.EmailVerify(r.Context(), onboarding.EmailVerifyRequest{
		OnboardToken: req.OnboardToken, Code: req.Code,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "email verified", responses.FromOnboardStep(res))
}

// TOTPInit godoc
// @Summary      Super admin бүртгэл — TOTP (2FA) тохируулга эхлүүлэх
// @Description  Шинэ TOTP secret үүсгэж, authenticator app-д уншуулах otpauth:// URI-г буцаана (QR-г frontend зурна; secret нь гараар оруулах хувилбарт). Дахин дуудвал ШИНЭ secret үүснэ.
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardTokenRequest  true  "Onboard token"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardTOTPResponse}  "TOTP secret + otpauth URI"
// @Failure      400  {object}  v1.BaseResponse  "Wrong step"
// @Failure      403  {object}  v1.BaseResponse  "Onboard session invalid or expired"
// @Router       /v1/auth/superadmin/onboard/totp/init [post]
func (h Handler) TOTPInit(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardTokenRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.TOTPInit(r.Context(), onboarding.TokenRequest{OnboardToken: req.OnboardToken})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "totp setup started", responses.FromOnboardTOTPInit(res))
}

// TOTPVerify godoc
// @Summary      Super admin бүртгэл — TOTP баталгаажуулж ТӨГСГӨХ
// @Description  Authenticator app-ийн кодыг шалгаж, бүртгэлийг төгсгөнө: super admin хэрэглэгч үүсгэж/ахиулж, нөөц кодуудыг хадгалж, урилгыг ашигласан болгож, session (token + refresh_token) олгоно. recovery_codes нь ЗӨВХӨН ЭНЭ хариунд, ЗӨВХӨН НЭГ УДАА буцна — дахин авах боломжгүй.
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminOnboardCodeRequest  true  "Onboard token + TOTP код"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminOnboardDoneResponse}  "Super admin created + logged in (recovery codes shown once)"
// @Failure      400  {object}  v1.BaseResponse  "Invalid code or incomplete steps"
// @Failure      403  {object}  v1.BaseResponse  "Onboard session invalid or expired"
// @Failure      409  {object}  v1.BaseResponse  "Email or Google account already linked to another user"
// @Router       /v1/auth/superadmin/onboard/totp/verify [post]
func (h Handler) TOTPVerify(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminOnboardCodeRequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.TOTPVerify(r.Context(), onboarding.TOTPVerifyRequest{
		OnboardToken: req.OnboardToken, Code: req.Code,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "superadmin onboarding completed", responses.FromOnboardDone(res))
}

// MFA godoc
// @Summary      Super admin нэвтрэлт — 2FA (TOTP / нөөц код)
// @Description  Google эсвэл eID нэвтрэлтээс авсан mfa_token-ийг TOTP код ЭСВЭЛ нөөц кодоор баталгаажуулж session олгоно. Нөөц код нэг удаагийн (хэрэглэгдмэгц идэвхгүй болно). Хэт олон буруу оролдлого → токен цуцлагдана.
// @Tags         superadmin-onboard
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SuperadminMFARequest  true  "mfa_token + TOTP эсвэл нөөц код"
// @Success      200  {object}  v1.BaseResponse{data=responses.SuperadminMFAResponse}  "Logged in"
// @Failure      400  {object}  v1.BaseResponse  "Invalid code"
// @Failure      403  {object}  v1.BaseResponse  "MFA token invalid/expired or too many attempts"
// @Router       /v1/auth/superadmin/mfa [post]
func (h Handler) MFA(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminMFARequest
	if err := decode(w, r, &req); err != nil {
		return err
	}
	res, err := h.usecase.SuperadminMFA(r.Context(), onboarding.MFARequest{
		MFAToken: req.MFAToken, Code: req.Code,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "superadmin mfa verified", responses.FromSuperadminMFA(res))
}

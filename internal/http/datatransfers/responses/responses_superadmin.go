// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/domain"
	onboarding "template/internal/business/usecases/superadmin_onboarding"
)

// SuperadminInviteResponse нь super admin болох урилгын нэг мөр.
type SuperadminInviteResponse struct {
	Email      string     `json:"email"`
	InvitedBy  string     `json:"invited_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	// Pending нь урилга хараахан ашиглагдаагүй (бүртгэл хийгдээгүй) эсэх —
	// UI-д "хүлээгдэж буй / ашигласан" гэж ялгахад.
	Pending bool `json:"pending"`
}

// FromSuperadminInvite нь урилгыг DTO рүү буулгана.
func FromSuperadminInvite(i domain.SuperadminInvite) SuperadminInviteResponse {
	return SuperadminInviteResponse{
		Email:      i.Email,
		InvitedBy:  i.InvitedBy,
		CreatedAt:  i.CreatedAt,
		AcceptedAt: i.AcceptedAt,
		Pending:    !i.Accepted(),
	}
}

// ToSuperadminInviteList нь урилгын жагсаалтыг DTO жагсаалт руу буулгана
// (хоосон үед JSON null биш [] буцаана).
func ToSuperadminInviteList(list []domain.SuperadminInvite) []SuperadminInviteResponse {
	out := make([]SuperadminInviteResponse, 0, len(list))
	for _, i := range list {
		out = append(out, FromSuperadminInvite(i))
	}
	return out
}

// ── Бүртгэлийн шидтэн (onboarding) ──

// SuperadminOnboardStartResponse нь /onboard/google-ийн хариу — onboard_token-ийг
// дараагийн бүх алхамд дамжуулна.
type SuperadminOnboardStartResponse struct {
	OnboardToken string `json:"onboard_token"`
	Email        string `json:"email"`
	Step         string `json:"step"`
}

// FromOnboardGoogle нь usecase-ийн үр дүнг DTO рүү буулгана.
func FromOnboardGoogle(r onboarding.GoogleResponse) SuperadminOnboardStartResponse {
	return SuperadminOnboardStartResponse{OnboardToken: r.OnboardToken, Email: r.Email, Step: r.Step}
}

// SuperadminOnboardEIDStartResponse нь /onboard/eid/start(-id)-ийн хариу.
type SuperadminOnboardEIDStartResponse struct {
	SessionID        string `json:"session_id"`
	DeviceLinkURL    string `json:"device_link_url,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
}

// FromOnboardEIDStart нь eID эхлүүлэлтийн үр дүнг DTO рүү буулгана.
func FromOnboardEIDStart(r onboarding.EIDStartResponse) SuperadminOnboardEIDStartResponse {
	return SuperadminOnboardEIDStartResponse{
		SessionID:        r.SessionID,
		DeviceLinkURL:    r.DeviceLinkURL,
		VerificationCode: r.VerificationCode,
		ExpiresAt:        r.ExpiresAt,
	}
}

// SuperadminOnboardEIDPollResponse нь /onboard/eid/poll-ийн хариу — токен
// БУЦАХГҮЙ (eID нь зөвхөн баталгаажуулалтын алхам).
type SuperadminOnboardEIDPollResponse struct {
	State string `json:"state"`
	Step  string `json:"step"`
}

// FromOnboardEIDPoll нь poll-ийн үр дүнг DTO рүү буулгана.
func FromOnboardEIDPoll(r onboarding.EIDPollResponse) SuperadminOnboardEIDPollResponse {
	return SuperadminOnboardEIDPollResponse{State: r.State, Step: r.Step}
}

// SuperadminOnboardStepResponse нь шидтэний дараагийн алхмыг мэдэгдэнэ.
type SuperadminOnboardStepResponse struct {
	Step string `json:"step"`
}

// FromOnboardStep нь алхмын үр дүнг DTO рүү буулгана.
func FromOnboardStep(r onboarding.StepResponse) SuperadminOnboardStepResponse {
	return SuperadminOnboardStepResponse{Step: r.Step}
}

// SuperadminOnboardTOTPResponse нь /onboard/totp/init-ийн хариу — otpauth_url-аас
// QR зурж, эсвэл secret-ийг гараар оруулж authenticator app тохируулна.
type SuperadminOnboardTOTPResponse struct {
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauth_url"`
	Step       string `json:"step"`
}

// FromOnboardTOTPInit нь TOTP тохируулгын үр дүнг DTO рүү буулгана.
func FromOnboardTOTPInit(r onboarding.TOTPInitResponse) SuperadminOnboardTOTPResponse {
	return SuperadminOnboardTOTPResponse{Secret: r.Secret, OtpauthURL: r.OtpauthURL, Step: r.Step}
}

// SuperadminOnboardDoneResponse нь /onboard/totp/verify-ийн хариу — бүртгэл
// төгсөж, хэрэглэгч нэвтэрсэн. recovery_codes нь ЗӨВХӨН ЭНД, ЗӨВХӨН НЭГ УДАА
// буцна (DB-д зөвхөн hash хадгалагдана) — клиент хэрэглэгчид заавал харуулж
// хадгалуулна.
type SuperadminOnboardDoneResponse struct {
	Step          string   `json:"step"`
	RecoveryCodes []string `json:"recovery_codes"`
	UserResponse
}

// FromOnboardDone нь finalize-ийн үр дүнг DTO рүү буулгана (токенууд нь
// /login-той ИЖИЛ хэлбэрээр token / refresh_token талбарт).
func FromOnboardDone(r onboarding.FinalizeResponse) SuperadminOnboardDoneResponse {
	out := SuperadminOnboardDoneResponse{Step: r.Step, RecoveryCodes: r.RecoveryCodes}
	out.UserResponse = FromV1Domain(r.User)
	out.Token = r.AccessToken
	out.RefreshToken = r.RefreshToken
	return out
}

// SuperadminMFAResponse нь /auth/superadmin/mfa-ийн хариу — MFA давсан тул
// session олгогдсон (/login-той ижил хэлбэр).
type SuperadminMFAResponse struct {
	// UsedRecoveryCode нь нөөц кодоор нэвтэрсэн эсэх; тийм бол
	// recovery_codes_left нь үлдсэн кодын тоо (UI сануулга харуулна).
	UsedRecoveryCode  bool `json:"used_recovery_code"`
	RecoveryCodesLeft int  `json:"recovery_codes_left"`
	UserResponse
}

// FromSuperadminMFA нь MFA нэвтрэлтийн үр дүнг DTO рүү буулгана.
func FromSuperadminMFA(r onboarding.MFAResponse) SuperadminMFAResponse {
	out := SuperadminMFAResponse{
		UsedRecoveryCode:  r.UsedRecoveryCode,
		RecoveryCodesLeft: r.RecoveryCodesLeft,
	}
	out.UserResponse = FromV1Domain(r.User)
	out.Token = r.AccessToken
	out.RefreshToken = r.RefreshToken
	return out
}

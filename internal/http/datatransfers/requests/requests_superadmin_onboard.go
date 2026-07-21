// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// SuperadminInviteRequest нь POST /superadmin/invites-ийн body — и-мэйлийг
// super admin болох allow-list-д нэмнэ.
type SuperadminInviteRequest struct {
	Email string `json:"email" validate:"required,email,max=100"`
}

// SuperadminOnboardGoogleRequest нь POST /auth/superadmin/onboard/google-ийн
// body — Google OAuth callback-ийн code + redirect_uri (BFF дамжуулна).
type SuperadminOnboardGoogleRequest struct {
	Code        string `json:"code" validate:"required"`
	RedirectURI string `json:"redirect_uri" validate:"required"`
}

// SuperadminOnboardEIDStartRequest нь POST /auth/superadmin/onboard/eid/start-ийн
// body. callbackUrl (сонголт): SAME-DEVICE (утасны browser) үед
// <origin>/auth/eid/callback; хоосон бол CROSS-DEVICE (desktop QR).
type SuperadminOnboardEIDStartRequest struct {
	OnboardToken string `json:"onboard_token" validate:"required"`
	CallbackUrl  string `json:"callbackUrl,omitempty"`
}

// SuperadminOnboardEIDStartIDRequest нь POST /auth/superadmin/onboard/eid/start-id-ийн
// body — иргэний РД-аар нэвтрэлт эхлүүлж, төхөөрөмж рүү push хийлгэнэ.
type SuperadminOnboardEIDStartIDRequest struct {
	OnboardToken string `json:"onboard_token" validate:"required"`
	NationalID   string `json:"national_id" validate:"required"`
	CallbackUrl  string `json:"callbackUrl,omitempty"`
}

// SuperadminOnboardEIDPollRequest нь POST /auth/superadmin/onboard/eid/poll-ийн body.
type SuperadminOnboardEIDPollRequest struct {
	OnboardToken string `json:"onboard_token" validate:"required"`
	SessionID    string `json:"session_id" validate:"required"`
}

// SuperadminOnboardTokenRequest нь зөвхөн шидтэний токен шаардах алхмуудын body
// (/email/send, /totp/init).
type SuperadminOnboardTokenRequest struct {
	OnboardToken string `json:"onboard_token" validate:"required"`
}

// SuperadminOnboardCodeRequest нь код шаардах алхмуудын body (/email/verify,
// /totp/verify).
type SuperadminOnboardCodeRequest struct {
	OnboardToken string `json:"onboard_token" validate:"required"`
	Code         string `json:"code" validate:"required,max=32"`
}

// SuperadminMFARequest нь POST /auth/superadmin/mfa-ийн body — Google/eID
// нэвтрэлтээс авсан mfa_token + TOTP код ЭСВЭЛ нөөц код.
type SuperadminMFARequest struct {
	MFAToken string `json:"mfa_token" validate:"required"`
	Code     string `json:"code" validate:"required,max=32"`
}

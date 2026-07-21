// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

import (
	"template/internal/business/domain"
)

// RegisterRequest нь POST /auth/register-ийн body юм.
type RegisterRequest struct {
	LastName    string `json:"last_name" validate:"required,min=1,max=50"`  // овог (монгол)
	FirstName   string `json:"first_name" validate:"required,min=1,max=50"` // нэр (монгол)
	LastNameEn  string `json:"last_name_en" validate:"omitempty,max=50"`    // овог (англи)
	FirstNameEn string `json:"first_name_en" validate:"omitempty,max=50"`   // нэр (англи)
	Username    string `json:"username" validate:"required,min=3,max=25"`
	Email       string `json:"email" validate:"required,email,max=50"`
	Password    string `json:"password" validate:"required,min=12,max=72,strongpassword"`
}

func (r RegisterRequest) ToV1Domain() *domain.User {
	return &domain.User{
		Username:    r.Username,
		FirstName:   r.FirstName,
		LastName:    r.LastName,
		FirstNameEn: r.FirstNameEn,
		LastNameEn:  r.LastNameEn,
		Email:       r.Email,
		Password:    r.Password,
		RoleID:      2, // бүртгүүлсэн хүн бүр энгийн хэрэглэгч байна
	}
}

// SendOTPRequest нь POST /auth/send-otp-ийн body юм.
type SendOTPRequest struct {
	Email string `json:"email" validate:"required,email,max=50"`
}

// VerifyOTPRequest нь POST /auth/verify-otp-ийн body юм.
type VerifyOTPRequest struct {
	Email string `json:"email" validate:"required,email,max=50"`
	Code  string `json:"code" validate:"required,numeric"`
}

// LoginRequest нь POST /auth/login-ийн body юм.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=50"`
	Password string `json:"password" validate:"required,min=1,max=72"`
}

func (r *LoginRequest) ToV1Domain() *domain.User {
	return &domain.User{
		Email:    r.Email,
		Password: r.Password,
	}
}

// RefreshRequest нь POST /auth/refresh-ийн body юм.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest нь POST /auth/logout-ийн body юм. access_token нь сонголттой —
// өгвөл түүний jti deny-list-д орж access токен шууд хүчингүй болно.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	AccessToken  string `json:"access_token" validate:"omitempty"`
}

// ChangePasswordRequest нь PUT /auth/password/change-ийн body юм.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,min=1,max=72"`
	NewPassword     string `json:"new_password" validate:"required,min=12,max=72,strongpassword"`
}

// ForgotPasswordRequest нь POST /auth/password/forgot-ийн body юм.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email,max=50"`
}

// EIDPollRequest нь POST /auth/eid/poll-ийн body юм — /eid/start-аас авсан
// session_id-г IdP-д long-poll-оор асуухад дамжуулна.
type EIDPollRequest struct {
	SessionID string `json:"session_id" validate:"required"`
	// GoogleLinkToken нь Google-ээр эхний удаа нэвтэрсэн хэрэглэгч eID-ээр
	// баталгаажуулж байгаа үед л ирнэ (сонголттой).
	GoogleLinkToken string `json:"google_link_token,omitempty"`
}

// GoogleLoginRequest нь POST /auth/google-ийн body — Google OAuth callback-ийн
// code + redirect_uri (BFF дамжуулна).
type GoogleLoginRequest struct {
	Code        string `json:"code" validate:"required"`
	RedirectURI string `json:"redirect_uri" validate:"required"`
}

// EIDStartByNationalIDRequest нь POST /auth/eid/start-id-ийн body юм — иргэний
// РД-аар нэвтрэлт эхлүүлж, бүртгэлтэй төхөөрөмж рүү push хийлгэнэ.
type EIDStartByNationalIDRequest struct {
	NationalID string `json:"national_id" validate:"required"`
	// CallbackUrl (сонголт): SAME-DEVICE (утасны browser) үед <origin>/auth/eid/callback; хоосон
	// бол CROSS-DEVICE (desktop). Backend force-normalize хийнэ.
	CallbackUrl string `json:"callbackUrl,omitempty"`
}

// ResetPasswordRequest нь POST /auth/password/reset-ийн body юм. Нууц үг
// сэргээх нь GeregeCloud Verify OTP-аар явдаг тул токены оронд email + код
// шаардана.
type ResetPasswordRequest struct {
	Email       string `json:"email" validate:"required,email,max=50"`
	Code        string `json:"code" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=12,max=72,strongpassword"`
}

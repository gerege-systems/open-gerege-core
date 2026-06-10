// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

import (
	"template/internal/business/domain"
)

// RegisterRequest нь POST /auth/register-ийн body юм.
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=25"`
	Email    string `json:"email" validate:"required,email,max=50"`
	Password string `json:"password" validate:"required,min=12,max=72,strongpassword"`
}

func (r RegisterRequest) ToV1Domain() *domain.User {
	return &domain.User{
		Username: r.Username,
		Email:    r.Email,
		Password: r.Password,
		RoleID:   2, // бүртгүүлсэн хүн бүр энгийн хэрэглэгч байна
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

// RefreshRequest нь POST /auth/refresh болон POST /auth/logout-ийн body
// юм — хоёулаа refresh токен ашиглана.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
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

// ResetPasswordRequest нь POST /auth/password/reset-ийн body юм. Нууц үг
// сэргээх нь GeregeCloud Verify OTP-аар явдаг тул токены оронд email + код
// шаардана.
type ResetPasswordRequest struct {
	Email       string `json:"email" validate:"required,email,max=50"`
	Code        string `json:"code" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=12,max=72,strongpassword"`
}

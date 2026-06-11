// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package auth нь credential баталгаажуулалт, session-ийн амьдралын мөчлөг
// (access + refresh токенууд), OTP идэвхжүүлэлт болон нууц үгийн амьдралын
// мөчлөгийг (солих / мартсан / шинэчлэх) хариуцдаг.
package auth

import (
	"context"

	"template/internal/business/domain"
)

// Usecase нь HTTP handler-ийн харьцдаг оролтын хил (input boundary) юм. Method
// бүр Request struct авч, (буцаах өгөгдөлтэй үед) Response struct буцаадаг тул
// талбар нэмэх нь хувилбаруудын хооронд буцах нийцтэй (backward-compatible)
// хэвээр үлддэг.
type Usecase interface {
	// Register нь шинэ (идэвхгүй) бүртгэл үүсгэнэ; идэвхжүүлэхэд OTP урсгал шаардлагатай.
	Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error)
	// Login нь credential-ийг шалгаж, шинэ access+refresh токен хосыг буцаана.
	Login(ctx context.Context, req LoginRequest) (LoginResponse, error)
	// SendOTP нь 6 оронтой кодыг email-ээр илгээж, TTL-тэйгээр Redis-д хадгална.
	SendOTP(ctx context.Context, req SendOTPRequest) error
	// VerifyOTP нь кодыг хэрэглэж, бүртгэлийг идэвхжүүлнэ; email тус бүрд rate-limit-тэй.
	VerifyOTP(ctx context.Context, req VerifyOTPRequest) error
	// Refresh нь refresh токеныг эргүүлдэг: шинэ хос үүсгэж, хуучин jti-г хүчингүй болгоно.
	Refresh(ctx context.Context, req RefreshRequest) (LoginResponse, error)
	// Logout нь refresh токены jti-г устгаснаар дахин ашиглах боломжгүй болгоно.
	Logout(ctx context.Context, req LogoutRequest) error
	// ChangePassword нь баталгаажсан хэрэглэгчийн нууц үгийг солино.
	// Session булаах (hijacking)-ийг таслан зогсоохын тулд одоогийн нууц үгийг шаарддаг.
	ChangePassword(ctx context.Context, req ChangePasswordRequest) error
	// ForgotPassword нь GeregeCloud Verify-ээр email рүү OTP код илгээж нууц үг
	// шинэчлэх урсгалыг эхлүүлнэ. Хэрэглэгчийн тооллогыг (enumeration) таслахын
	// тулд тодорхойгүй email-д үргэлж nil буцаана.
	ForgotPassword(ctx context.Context, req ForgotPasswordRequest) error
	// ResetPassword нь email рүү илгээсэн OTP кодыг Verify-ээр баталгаажуулж,
	// шинэ нууц үгийг тохируулна.
	ResetPassword(ctx context.Context, req ResetPasswordRequest) error
}

// Usecase-ийн хилд зориулсан Request / Response төрлүүд. Struct-д талбар нэмэх
// нь дуудагчдыг эвддэггүй, харин method-ийн гарын үсэгт (signature) параметр
// нэмэх нь эвддэг — Uncle Bob-ийн "Input/Output Boundary" зөвлөмжийг бодит
// байдлаар хэрэгжүүлсэн нь.
type (
	RegisterRequest struct {
		User *domain.User
	}
	RegisterResponse struct {
		User domain.User
	}

	LoginRequest struct {
		Email    string
		Password string
	}

	LoginResponse struct {
		User         domain.User
		AccessToken  string
		RefreshToken string
	}

	SendOTPRequest struct {
		Email string
	}

	VerifyOTPRequest struct {
		Email   string
		OTPCode string
	}

	RefreshRequest struct {
		RefreshToken string
	}

	LogoutRequest struct {
		RefreshToken string
		// AccessToken нь сонголттой — өгвөл jti-г нь deny-list-д нэмж
		// access токеныг хугацаа дуусахаас өмнө шууд хүчингүй болгоно.
		AccessToken string
	}

	ChangePasswordRequest struct {
		UserID          string
		CurrentPassword string
		NewPassword     string
	}

	ForgotPasswordRequest struct {
		Email string
	}

	ResetPasswordRequest struct {
		Email       string
		Code        string
		NewPassword string
	}
)

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package auth нь credential баталгаажуулалт, session-ийн амьдралын мөчлөг
// (access + refresh токенууд), OTP идэвхжүүлэлт болон нууц үгийн амьдралын
// мөчлөгийг (солих / мартсан / шинэчлэх) хариуцдаг.
package auth

import (
	"context"

	"template/internal/business/domain"
	"template/pkg/eid"
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

	// EIDStart нь eID (QR/deep-link) нэвтрэлтийг IdP дээр эхлүүлж, клиент
	// харуулах session мэдээллийг (session_id, device_link_url, verification_code,
	// expires_at) буцаана.
	EIDStart(ctx context.Context) (EIDStartResponse, error)
	// EIDStartByNationalID нь иргэний РД (national_id)-аар нэвтрэлтийг IdP дээр
	// эхлүүлж, тухайн РД-тэй холбоотой төхөөрөмж рүү баталгаажуулах prompt push
	// хийлгэнэ. device_link шаардлагагүй тул зөвхөн session_id, verification_code,
	// expires_at буцна; дуусгахдаа QR урсгалтай ижил EIDPoll ашиглана.
	EIDStartByNationalID(ctx context.Context, nationalID string) (EIDStartResponse, error)
	// EIDPoll нь session-ийн төлвийг long-poll-оор асууна. COMPLETE болоход
	// IdP-ийн identity-аар хэрэглэгчийг upsert хийж, access+refresh токен хос
	// олгож буцаана; бусад (RUNNING/EXPIRED/REFUSED) үед зөвхөн State буцаана.
	EIDPoll(ctx context.Context, req EIDPollRequest) (EIDPollResponse, error)
	// EIDRepresentations нь нэвтэрсэн хэрэглэгчийн (userID-аар) төлөөлдөг
	// байгууллагуудыг eID-ээс буцаана. Хэрэглэгч eID-ээр нэвтрээгүй (civil_id
	// байхгүй) бол хоосон slice.
	EIDRepresentations(ctx context.Context, userID string) ([]eid.Representation, error)
	// EID PKI самбарын нэгдсэн/дэлгэрэнгүй мэдээлэл (PKI_READ эрхтэй RP-д).
	// eID хэрэглэгч биш бол nil; эрхгүй бол apperror.Forbidden.
	EIDSummary(ctx context.Context, userID string) (*eid.PersonSummary, error)
	EIDCertificates(ctx context.Context, userID string) (*eid.PersonCertificates, error)
	EIDDevices(ctx context.Context, userID string) (*eid.PersonDevices, error)
	EIDActivity(ctx context.Context, userID string, limit, offset int) (*eid.PersonActivity, error)
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

	// EIDStartResponse нь /eid/start-ийн үр дүн — клиент үүгээр QR/deep-link
	// харуулж, /eid/poll руу SessionID-г дамжуулна.
	EIDStartResponse struct {
		SessionID        string
		DeviceLinkURL    string
		VerificationCode string
		ExpiresAt        string
	}

	EIDPollRequest struct {
		SessionID string
	}

	// EIDPollResponse нь /eid/poll-ийн үр дүн. State нь IdP-ийн session төлөв
	// (RUNNING / COMPLETE / EXPIRED / REFUSED). COMPLETE үед User + токенууд
	// дүүрэн байна (Login-той ижил хэлбэрээр клиентэд буудаг).
	EIDPollResponse struct {
		State        string
		User         domain.User
		AccessToken  string
		RefreshToken string
	}
)

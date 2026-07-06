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

	// EIDStart нь eID device-link нэвтрэлтийг IdP дээр эхлүүлж, клиент харуулах session мэдээллийг
	// буцаана. callbackURL хоосон = CROSS-DEVICE (desktop QR); хоосон биш = SAME-DEVICE (mobile
	// browser App2App — approve-ийн дараа browser callback руу буцна).
	EIDStart(ctx context.Context, callbackURL string) (EIDStartResponse, error)
	// EIDStartByNationalID нь иргэний РД (national_id)-аар нэвтрэлтийг IdP дээр
	// эхлүүлж, тухайн РД-тэй холбоотой төхөөрөмж рүү баталгаажуулах prompt push
	// хийлгэнэ. device_link шаардлагагүй тул зөвхөн session_id, verification_code,
	// expires_at буцна; дуусгахдаа QR урсгалтай ижил EIDPoll ашиглана.
	EIDStartByNationalID(ctx context.Context, nationalID, callbackURL string) (EIDStartResponse, error)
	// EIDPoll нь session-ийн төлвийг long-poll-оор асууна. COMPLETE болоход
	// IdP-ийн identity-аар хэрэглэгчийг upsert хийж, access+refresh токен хос
	// олгож буцаана; бусад (RUNNING/EXPIRED/REFUSED) үед зөвхөн State буцаана.
	EIDPoll(ctx context.Context, req EIDPollRequest) (EIDPollResponse, error)
	// EIDRepresentations нь нэвтэрсэн хэрэглэгчийн (userID-аар) төлөөлдөг
	// байгууллагуудыг eID-ээс буцаана. Хэрэглэгч eID-ээр нэвтрээгүй (civil_id
	// байхгүй) бол хоосон slice.
	EIDRepresentations(ctx context.Context, userID string) ([]eid.Representation, error)
	// RegisterEIDOrganization нь улсын бүртгэлээс (XYP) байгууллагыг regNo-гоор
	// хайж, нэвтэрсэн хэрэглэгчийг (eID РД нь тухайн байгууллагын эрх бүхий
	// этгээд бол) eidmongolia-д төлөөлөл болгон холбоно. Иргэний бүх төлөөллийг
	// буцаана. Байгууллага олдоогүй → NotFound; эрхгүй → Forbidden.
	RegisterEIDOrganization(ctx context.Context, userID, regNo string) ([]eid.Representation, error)
	// UnlinkEIDOrganization нь нэвтэрсэн хэрэглэгч өөрийн байгууллагын төлөөллөө цуцлана.
	UnlinkEIDOrganization(ctx context.Context, userID, orgRegister string) ([]eid.Representation, error)
	// ListEIDOrgSigners нь байгууллагын гарын үсэг зурагчдыг буцаана (хэрэглэгч төлөөлөгч байх ёстой).
	ListEIDOrgSigners(ctx context.Context, userID, orgRegister string) ([]eid.Signer, error)
	// AddEIDOrgSigner нь байгууллагад өөр иргэнийг (РД) гарын үсэг зурах эрхтэй (MANAGER) болгож
	// нэмж, тэр хүн рүү sign-push илгээнэ. Жагсаалт + хүлээгдэж буй баталгаажуулалтыг буцаана.
	AddEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo, role string) (*eid.SignersResult, error)
	// RemoveEIDOrgSigner нь байгууллагаас гарын үсэг зурагчийг (РД) хасна.
	RemoveEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo string) ([]eid.Signer, error)
	// ResendEIDOrgSigner нь баталгаажаагүй гарын үсэг зурагч руу sign-push дахин илгээнэ.
	ResendEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo string) (*eid.SignersResult, error)
	// GoogleLogin нь Google authorization code-ийг боловсруулна: холбогдсон
	// account бол шууд нэвтрүүлж, эс бол eID-ээр баталгаажуулах LinkToken буцаана.
	GoogleLogin(ctx context.Context, code, redirectURI string) (GoogleLoginResponse, error)
	// UnlinkGoogleFromUser нь нэвтэрсэн хэрэглэгчийн Google холболтыг арилгана
	// (холбох нь зөвхөн login урсгалаар).
	UnlinkGoogleFromUser(ctx context.Context, userID string) error
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
		// GoogleLinkToken нь Google-ээр эхний удаа нэвтэрсэн хэрэглэгч eID-ээр
		// баталгаажуулж байгаа үед л ирнэ — COMPLETE болоход тухайн Google
		// account-ийг энэ eID хэрэглэгчид холбоно. Хоосон бол зүгээр eID нэвтрэлт.
		GoogleLinkToken string
	}

	// GoogleLoginResponse нь Google callback-ийн үр дүн. Linked=true бол шууд
	// нэвтэрсэн (Login дүүрэн); false бол эхний удаа тул eID-ээр баталгаажуулах
	// шаардлагатай (LinkToken-ийг eID poll руу дамжуулна).
	GoogleLoginResponse struct {
		Linked    bool
		Login     LoginResponse
		LinkToken string
		Email     string
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

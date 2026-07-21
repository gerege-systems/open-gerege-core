// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package onboarding нь урилгаар хаалттай (invite-gated) super admin
// бүртгэлийн шидтэн болон MFA-тай super admin нэвтрэлтийн 2 дахь шатыг
// хариуцна.
//
// Бүртгэлийн урсгал (алхам бүр Redis дэх түр "pending" session-оор холбогдоно,
// ~30 мин):
//
//  1. google      — Google OAuth code солих; и-мэйл нь ХҮЛЭЭГДЭЖ БУЙ
//     superadmin_invites-д байх ёстой (эс бөгөөс 403).
//  2. eid         — eID-ээр бодит хүнийг баталгаажуулж identity (civil_id г.м.)
//     барина. АНХААР: энэ алхамд session ОЛГОГДОХГҮЙ.
//  3. email       — урилгын и-мэйл рүү OTP илгээж баталгаажуулна (Verify API).
//  4. totp        — authenticator app-д secret үүсгэж (otpauth URI), кодоор
//     баталгаажуулаад ТӨГСГӨНӨ (finalize).
//
// Finalize нь super admin хэрэглэгчийг (Google + eID + и-мэйл, mfa_enabled,
// шифрлэгдсэн totp_secret, RoleSuperAdmin) service RLS дор upsert хийж, нөөц
// кодуудыг hash-лан хадгалж, урилгыг accepted болгож, session олгоно. Энгийн
// текст нөөц кодууд ЗӨВХӨН энд, ЗӨВХӨН НЭГ УДАА буцна.
//
// Давтагдах нэвтрэлт (SuperadminMFA) нь auth.GoogleLogin/EIDPoll-ийн үүсгэсэн
// mfa_token-ийг TOTP эсвэл нөөц кодоор баталгаажуулж session олгоно.
package onboarding

import (
	"context"

	"template/internal/business/domain"
)

// Бүртгэлийн шидтэний алхмууд — pending session-д хадгалагдаж, клиент дараагийн
// дуудлагаа үүгээр сонгоно. Алхмыг алгасах оролдлого apperror.BadRequest болно.
const (
	StepEID   = "eid"   // Google баталгаажсан → eID хүлээж байна
	StepEmail = "email" // eID баталгаажсан → и-мэйл OTP хүлээж байна
	StepTOTP  = "totp"  // и-мэйл баталгаажсан → TOTP тохируулга хүлээж байна
	StepDone  = "done"  // төгссөн (super admin үүссэн)
)

// Usecase нь HTTP handler-ийн харьцдаг оролтын хил (input boundary). Method бүр
// Request struct авч Response struct буцаадаг тул талбар нэмэх нь буцах
// нийцтэй хэвээр үлдэнэ.
type Usecase interface {
	// Google нь OAuth code-ийг солиж, и-мэйлийг superadmin_invites-ийн эсрэг
	// шалгаад (урилгагүй/ашиглагдсан бол Forbidden) шинэ pending session
	// (onboard_token) үүсгэнэ.
	Google(ctx context.Context, req GoogleRequest) (GoogleResponse, error)
	// EIDStart нь бүртгэлийн eID алхмыг QR/deep-link-ээр эхлүүлнэ.
	EIDStart(ctx context.Context, req EIDStartRequest) (EIDStartResponse, error)
	// EIDStartByNationalID нь eID алхмыг иргэний РД-аар (утас руу push) эхлүүлнэ.
	EIDStartByNationalID(ctx context.Context, req EIDStartByNationalIDRequest) (EIDStartResponse, error)
	// EIDPoll нь eID session-ийн төлвийг long-poll-оор асууна. COMPLETE үед
	// identity-г pending session-д БАРИНА (session олгохгүй) ба алхам "email"
	// болно.
	EIDPoll(ctx context.Context, req EIDPollRequest) (EIDPollResponse, error)
	// EmailSend нь урилгын и-мэйл рүү OTP илгээнэ (Verify API).
	EmailSend(ctx context.Context, req TokenRequest) (StepResponse, error)
	// EmailVerify нь OTP кодыг шалгаж, алхмыг "totp" болгоно.
	EmailVerify(ctx context.Context, req EmailVerifyRequest) (StepResponse, error)
	// TOTPInit нь шинэ TOTP secret үүсгэж pending session-д хадгалаад,
	// authenticator app-д уншуулах otpauth:// URI-г буцаана. Дахин дуудвал
	// шинэ secret үүснэ (QR дахин уншуулах боломж).
	TOTPInit(ctx context.Context, req TokenRequest) (TOTPInitResponse, error)
	// TOTPVerify нь кодыг secret-тэй тулгаж, амжилттай бол бүртгэлийг
	// ТӨГСГӨНӨ: super admin үүсгэж/ахиулж, нөөц кодуудыг хадгалж, урилгыг
	// accepted болгож, session олгоно.
	TOTPVerify(ctx context.Context, req TOTPVerifyRequest) (FinalizeResponse, error)
	// SuperadminMFA нь нэвтрэлтийн 2 дахь шат: auth.GoogleLogin/EIDPoll-ийн
	// үүсгэсэн mfa_token-ийг TOTP код ЭСВЭЛ нөөц кодоор баталгаажуулж session
	// олгоно. Нөөц код нэг удаагийн (хэрэглэгдмэгц идэвхгүй болно).
	SuperadminMFA(ctx context.Context, req MFARequest) (MFAResponse, error)
}

// Usecase-ийн хилд зориулсан Request / Response төрлүүд.
type (
	GoogleRequest struct {
		Code        string
		RedirectURI string
	}
	// GoogleResponse нь шидтэний эхлэл — OnboardToken-ийг дараагийн бүх алхамд
	// дамжуулна.
	GoogleResponse struct {
		OnboardToken string
		Email        string
		Step         string
	}

	// TokenRequest нь зөвхөн pending session-ийн токен шаардах алхмуудад.
	TokenRequest struct {
		OnboardToken string
	}

	EIDStartRequest struct {
		OnboardToken string
		CallbackURL  string
	}
	EIDStartByNationalIDRequest struct {
		OnboardToken string
		NationalID   string
		CallbackURL  string
	}
	EIDStartResponse struct {
		SessionID        string
		DeviceLinkURL    string
		VerificationCode string
		ExpiresAt        string
	}

	EIDPollRequest struct {
		OnboardToken string
		SessionID    string
	}
	// EIDPollResponse нь IdP-ийн төлөв (RUNNING/COMPLETE/EXPIRED/REFUSED).
	// COMPLETE үед Step нь "email" болно — токен/хэрэглэгч буцахгүй.
	EIDPollResponse struct {
		State string
		Step  string
	}

	EmailVerifyRequest struct {
		OnboardToken string
		Code         string
	}

	// StepResponse нь шидтэний дараагийн алхмыг мэдэгдэнэ.
	StepResponse struct {
		Step string
	}

	// TOTPInitResponse нь authenticator app тохируулах мэдээлэл. Secret нь QR
	// уншуулах боломжгүй үед гараар оруулахад; OtpauthURL-аас QR зурна.
	TOTPInitResponse struct {
		Secret     string
		OtpauthURL string
		Step       string
	}

	TOTPVerifyRequest struct {
		OnboardToken string
		Code         string
	}
	// FinalizeResponse нь бүртгэл төгссөний хариу — session + НЭГ УДАА харагдах
	// энгийн текст нөөц кодууд (дахин хэзээ ч авах боломжгүй).
	FinalizeResponse struct {
		User          domain.User
		AccessToken   string
		RefreshToken  string
		RecoveryCodes []string
		Step          string
	}

	MFARequest struct {
		MFAToken string
		// Code нь TOTP код эсвэл нөөц код (аль нь болохыг сервер өөрөө таана).
		Code string
	}
	MFAResponse struct {
		User         domain.User
		AccessToken  string
		RefreshToken string
		// RecoveryCodesLeft нь нөөц кодоор нэвтэрсэн үед үлдсэн кодын тоог
		// мэдэгдэнэ (UI сануулга харуулна).
		RecoveryCodesLeft int
		// UsedRecoveryCode нь нөөц код хэрэглэгдсэн эсэх.
		UsedRecoveryCode bool
	}
)

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/datasources/caches"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/crypto"
	"template/pkg/eid"
	"template/pkg/google"
	"template/pkg/jwt"
	"template/pkg/logger"
	"template/pkg/verify"
)

// GoogleClient нь Google OAuth code солих хэрэгцээт хэсэг (auth-ийнхтэй ижил
// хэлбэр). Дотоод interface болгосон нь тест fake хийхэд хялбар.
type GoogleClient interface {
	Configured() bool
	Exchange(ctx context.Context, code, redirectURI string) (*google.User, error)
}

// Config нь onboarding usecase-ийн тохиргоо.
type Config struct {
	// Issuer нь authenticator app-д харагдах нэр (жишээ "Government Template Platform V3.0").
	Issuer string
	// PendingTTL нь бүртгэлийн шидтэний pending session-ий амьдрах хугацаа.
	PendingTTL time.Duration
	// OTPTTL / OTPMaxAttempts нь и-мэйл OTP алхмын хугацаа ба оролдлогын дээд тоо.
	OTPTTL         time.Duration
	OTPMaxAttempts int
	// MFAMaxAttempts нь нэг mfa_token дээрх буруу кодын дээд тоо (brute-force).
	MFAMaxAttempts int
	// EIDDisplayText нь eID prompt-д харагдах RP-ийн нэр.
	EIDDisplayText string
	// RecoveryCodeCount нь үүсгэх нөөц кодын тоо (0 → recovery.DefaultCount).
	RecoveryCodeCount int
}

// usecase нь бүртгэлийн шидтэн + MFA нэвтрэлтийг хэрэгжүүлнэ. Хэрэглэгчийн
// бичилтийг users usecase-ээр биш repository-оор шууд хийдэг нь Clean
// Architecture-ийн дүрэмд нийцнэ (usecase → repositories/interface) бөгөөд
// нэвтрэхээс өмнөх (pre-auth) урсгал тул service RLS дор ажиллана.
type usecase struct {
	google          GoogleClient
	eid             eid.Client
	verifier        verify.Sender
	users           repointerface.UserRepository
	recovery        repointerface.RecoveryCodeRepository
	superadminAccts repointerface.SuperadminAccountRepository
	invites         repointerface.SuperadminInviteRepository
	jwtService      jwt.JWTService
	redisCache      caches.RedisCache
	cipher          *crypto.Cipher
	cfg             Config
}

// NewUsecase нь onboarding урсгалуудыг холбоно. encKey нь TOTP secret-ийг
// storage-д шифрлэх түлхүүр (INTEGRATION_ENC_KEY) — хоосон бол алдаа
// (fail-closed: secret-ийг ил текстээр хадгалахыг зөвшөөрөхгүй).
func NewUsecase(
	googleClient GoogleClient,
	eidClient eid.Client,
	verifier verify.Sender,
	usersRepo repointerface.UserRepository,
	recoveryRepo repointerface.RecoveryCodeRepository,
	superadminAcctsRepo repointerface.SuperadminAccountRepository,
	invitesRepo repointerface.SuperadminInviteRepository,
	jwtService jwt.JWTService,
	redisCache caches.RedisCache,
	encKey string,
	cfg Config,
) (Usecase, error) {
	if encKey == "" {
		return nil, fmt.Errorf("superadmin onboarding: encryption key (INTEGRATION_ENC_KEY) is required")
	}
	cipher, err := crypto.New(encKey)
	if err != nil {
		return nil, fmt.Errorf("superadmin onboarding: init cipher: %w", err)
	}
	if cfg.PendingTTL <= 0 {
		cfg.PendingTTL = 30 * time.Minute
	}
	if cfg.MFAMaxAttempts <= 0 {
		cfg.MFAMaxAttempts = 5
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "Government Template Platform V3.0"
	}
	return &usecase{
		google:          googleClient,
		eid:             eidClient,
		verifier:        verifier,
		users:           usersRepo,
		recovery:        recoveryRepo,
		superadminAccts: superadminAcctsRepo,
		invites:         invitesRepo,
		jwtService:      jwtService,
		redisCache:      redisCache,
		cipher:          cipher,
		cfg:             cfg,
	}, nil
}

// ── Redis түлхүүрүүд ──

// OnboardKey нь бүртгэлийн шидтэний pending session (JSON)-ийг хадгална.
func OnboardKey(token string) string { return fmt.Sprintf("superadmin_onboard:%s", token) }

// OnboardOTPKey нь и-мэйл алхмын Verify request_id-г хадгална.
func OnboardOTPKey(token string) string { return fmt.Sprintf("superadmin_onboard_otp:%s", token) }

// OnboardOTPAttemptsKey нь и-мэйл алхмын буруу оролдлогыг тоолно.
func OnboardOTPAttemptsKey(token string) string {
	return fmt.Sprintf("superadmin_onboard_otp_attempts:%s", token)
}

// MFAAttemptsKey нь нэг mfa_token дээрх буруу кодын оролдлогыг тоолно
// (TOTP нь ердөө 6 орон тул brute-force-оос хамгаална).
func MFAAttemptsKey(token string) string { return fmt.Sprintf("superadmin_mfa_attempts:%s", token) }

// superadminMFAAttemptsTTL нь оролдлогын тоологчийн амьдрах хугацаа —
// mfa_token-ийн TTL (5 мин)-ээс урт байх нь тоологчийг токеноос өмнө
// "мартагдахаас" сэргийлнэ (эс бөгөөс оролдлогын хязгаарыг тойрч болно).
const superadminMFAAttemptsTTL = 15 * time.Minute

// ── Pending session ──

// pendingSession нь шидтэний алхмуудын хооронд Redis-д зөөгддөг төлөв.
// АНХААР: PendingTOTPSecret нь ХАРААХАН баталгаажаагүй (ил текст) secret —
// зөвхөн энэ түр session-д амьдарч, finalize үед л шифрлэгдэж DB-д бичигдэнэ.
type pendingSession struct {
	// Google профайл (эхний алхмаас).
	GoogleSub           string `json:"google_sub"`
	Email               string `json:"email"`
	Name                string `json:"name"`
	Picture             string `json:"picture"`
	GoogleEmailVerified bool   `json:"google_email_verified"`
	// eID identity (2 дахь алхмаас).
	CivilID     string `json:"civil_id"`
	NationalID  string `json:"national_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	FirstNameEn string `json:"first_name_en"`
	LastNameEn  string `json:"last_name_en"`
	KYCLevel    string `json:"kyc_level"`
	// Баталгаажуулалтын төлөв.
	EmailVerified     bool   `json:"email_verified"`
	PendingTOTPSecret string `json:"pending_totp_secret"`
	Step              string `json:"step"`
}

// newOnboardToken нь 32 hex тэмдэгтийн (16 байт) crypto/rand токен үүсгэнэ.
func newOnboardToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// loadPending нь токеноор pending session-ийг уншина. Байхгүй/хугацаа дууссан/
// эвдэрсэн бол Forbidden — fail-closed (шидтэнг дахин эхлүүлнэ).
func (uc *usecase) loadPending(ctx context.Context, token string) (pendingSession, error) {
	if token == "" {
		return pendingSession{}, apperror.BadRequest("onboard_token is required")
	}
	raw, err := uc.redisCache.Get(ctx, OnboardKey(token))
	if err != nil || raw == "" {
		logger.WarnWithContext(ctx, "superadmin onboarding: pending session олдсонгүй", logger.Fields{
			"usecase": "superadmin_onboarding", "method": "loadPending", "has_error": err != nil,
		})
		return pendingSession{}, apperror.Forbidden("Бүртгэлийн session хүчингүй эсвэл хугацаа нь дууссан байна. Дахин эхлүүлнэ үү.")
	}
	var s pendingSession
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return pendingSession{}, apperror.Forbidden("Бүртгэлийн session хүчингүй байна. Дахин эхлүүлнэ үү.")
	}
	return s, nil
}

// savePending нь pending session-ийг TTL-тэйгээр бичнэ. Redis алдаа нь fatal —
// төлөв хадгалагдахгүй бол шидтэн үргэлжлэх боломжгүй (fail-closed).
func (uc *usecase) savePending(ctx context.Context, token string, s pendingSession) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return apperror.InternalCause(fmt.Errorf("marshal pending session: %w", err))
	}
	key := OnboardKey(token)
	if err := uc.redisCache.Set(ctx, key, string(payload)); err != nil {
		return apperror.InternalCause(fmt.Errorf("store pending session: %w", err))
	}
	if err := uc.redisCache.Expire(ctx, key, uc.cfg.PendingTTL); err != nil {
		return apperror.InternalCause(fmt.Errorf("expire pending session: %w", err))
	}
	return nil
}

// requireStep нь шидтэний алхмыг алгасахаас сэргийлнэ (жишээ нь eID-гүйгээр
// шууд TOTP тохируулах). Алхам таарахгүй бол BadRequest.
func requireStep(s pendingSession, want string) error {
	if s.Step != want {
		return apperror.BadRequest(fmt.Sprintf("энэ алхам одоогоор боломжгүй (хүлээгдэж буй алхам: %s)", s.Step))
	}
	return nil
}

// mintSession нь хэрэглэгчид access+refresh токен хос үүсгэж, refresh-ийг
// Redis-д тэмдэглэнэ. auth.mintSession-той ИЖИЛ хэлбэр — refresh түлхүүрийн
// нэрийг auth.RefreshKey-ээс дахин ашигласнаар /refresh, /logout урсгалууд энэ
// замаар олгогдсон session-д мөн адил ажиллана.
func (uc *usecase) mintSession(ctx context.Context, user domain.User) (jwt.TokenPair, error) {
	pair, err := uc.jwtService.GenerateTokenPair(user.ID, user.IsAdmin(), user.RoleID, user.Email)
	if err != nil {
		return jwt.TokenPair{}, apperror.InternalCause(fmt.Errorf("generate token pair: %w", err))
	}
	ttl := time.Until(pair.RefreshExpiresAt)
	if ttl <= 0 {
		return jwt.TokenPair{}, apperror.InternalCause(fmt.Errorf("refresh token already expired"))
	}
	key := auth.RefreshKey(pair.RefreshJTI)
	if err := uc.redisCache.Set(ctx, key, pair.RefreshJTI); err != nil {
		return jwt.TokenPair{}, apperror.InternalCause(fmt.Errorf("persist refresh: %w", err))
	}
	if err := uc.redisCache.Expire(ctx, key, ttl); err != nil {
		return jwt.TokenPair{}, apperror.InternalCause(fmt.Errorf("expire refresh: %w", err))
	}
	return pair, nil
}

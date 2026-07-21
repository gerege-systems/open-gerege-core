//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package testenv

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/internal/config"
	"template/internal/datasources/caches"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	"template/pkg/eid"
	"template/pkg/google"
	"template/pkg/helpers"
	"template/pkg/jwt"
	"template/pkg/verify"
)

// AuthFixture нь end-to-end тестүүдэд ашиглагддаг бүрэн холбогдсон auth
// хэсэг юм: жинхэнэ Postgres, жинхэнэ Redis, жинхэнэ Ristretto, жинхэнэ
// JWT — зөвхөн гадагш чиглэсэн GeregeCloud Verify үйлчилгээ л хуурамчаар
// (FakeVerifier) хийгдсэн, учир нь бид OTP кодуудыг барьж аваад
// VerifyOTP/ResetPassword руу буцаан өгөх хэрэгтэй бөгөөд гадны API-г
// CI-д дуудах нь үнэ цэнэгүй.
//
// Хоёр bounded context хоёулаа илчлэгдсэн: туршиж буй auth урсгалуудад
// Auth, хэрэглэгчийн бичлэгүүдийг шууд унших эсвэл өөрчлөх шаардлагатай
// аливаа тохиргоо / баталгаажуулалтын алхамд Users.
type AuthFixture struct {
	Auth     auth.Usecase
	Users    users.Usecase
	Verifier *FakeVerifier
	EID      *FakeEID
	JWT      jwt.JWTService
}

// FakeVerifier нь verify.Sender-г локалаар хангадаг — real gecloud API-руу
// алхдаггүй. Send нь 6 оронтой код + request_id үүсгэж барьж авдаг тул
// тестүүд LastCode-оор кодыг гаргаж VerifyOTP руу буцаан өгнө; Check нь
// тэр request_id-ийн кодтой тулгаж шалгана.
type FakeVerifier struct {
	mu         sync.Mutex
	byRequest  map[string]string // request_id → code
	byReceiver map[string]string // receiver → last code
	seq        int
}

func (v *FakeVerifier) Send(_ context.Context, to, _ string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.byRequest == nil {
		v.byRequest = map[string]string{}
		v.byReceiver = map[string]string{}
	}
	code, err := helpers.GenerateOTPCode(6)
	if err != nil {
		return "", err
	}
	v.seq++
	reqID := fmt.Sprintf("gcv_fake_%d", v.seq)
	v.byRequest[reqID] = code
	v.byReceiver[to] = code
	return reqID, nil
}

func (v *FakeVerifier) Check(_ context.Context, requestID, code string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if want, ok := v.byRequest[requestID]; !ok || want != code {
		return verify.ErrNotApproved
	}
	return nil
}

// LastCode нь хүлээн авагчид зориулж сүүлд үүсгэсэн OTP-г буцаана.
func (v *FakeVerifier) LastCode(t *testing.T, receiver string) string {
	t.Helper()
	v.mu.Lock()
	defer v.mu.Unlock()
	if code, ok := v.byReceiver[receiver]; ok {
		return code
	}
	t.Fatalf("no OTP captured for %s", receiver)
	return ""
}

// FakeEID нь eid.Client-г локалаар хангадаг — гадаад eID IdP-г дуудалгүйгээр
// integration тестүүдийг ажиллуулна. Тест бүр QRInitiate/Session-ийн хариуг
// талбаруудаар тохируулж болно (анхдагч нь хоосон — eID урсгалыг туршихгүй
// тестүүдэд хангалттай).
type FakeEID struct {
	StartResult   *eid.StartResult
	SessionResult *eid.SessionResult
	Reps          []eid.Representation
	Summary       *eid.PersonSummary
	Certs         *eid.PersonCertificates
	Devices       *eid.PersonDevices
	Activity      *eid.PersonActivity
	InitiateErr   error
	SessionErr    error
}

func (f *FakeEID) QRInitiate(_ context.Context, _, _, _ string) (*eid.StartResult, error) {
	if f.InitiateErr != nil {
		return nil, f.InitiateErr
	}
	if f.StartResult != nil {
		return f.StartResult, nil
	}
	return &eid.StartResult{SessionID: "fake-session"}, nil
}

// Initiate нь РД-ээр (push) flow-ийн fake — QRInitiate-тэй ижил хариу өгнө.
func (f *FakeEID) Initiate(_ context.Context, _, _, _ string) (*eid.StartResult, error) {
	if f.InitiateErr != nil {
		return nil, f.InitiateErr
	}
	if f.StartResult != nil {
		return f.StartResult, nil
	}
	return &eid.StartResult{SessionID: "fake-session"}, nil
}

func (f *FakeEID) Session(_ context.Context, _ string, _ int) (*eid.SessionResult, error) {
	if f.SessionErr != nil {
		return nil, f.SessionErr
	}
	if f.SessionResult != nil {
		return f.SessionResult, nil
	}
	return &eid.SessionResult{State: "RUNNING"}, nil
}

// Representations нь fake — Reps талбарыг буцаана (default хоосон).
func (f *FakeEID) Representations(_ context.Context, _ string) ([]eid.Representation, error) {
	return f.Reps, nil
}

// Байгууллага холбох / гарын үсэг зурагч удирдах fake-ууд — тэг утга (энэ урсгалыг
// туршихгүй интеграц тестүүдэд хангалттай; eid.Client интерфейсийг л хангана).
func (f *FakeEID) AddRepresentation(_ context.Context, _ string, _ eid.AddRepresentationInput) ([]eid.Representation, error) {
	return f.Reps, nil
}

func (f *FakeEID) RemoveRepresentation(_ context.Context, _, _ string) ([]eid.Representation, error) {
	return f.Reps, nil
}

func (f *FakeEID) OrgSigners(_ context.Context, _, _ string) ([]eid.Signer, error) {
	return nil, nil
}

func (f *FakeEID) AddSigner(_ context.Context, _, _ string, _ eid.AddSignerInput) (*eid.SignersResult, error) {
	return &eid.SignersResult{}, nil
}

func (f *FakeEID) RemoveSigner(_ context.Context, _, _, _ string) ([]eid.Signer, error) {
	return nil, nil
}

func (f *FakeEID) ResendSigner(_ context.Context, _, _, _ string) (*eid.SignersResult, error) {
	return &eid.SignersResult{}, nil
}

func (f *FakeEID) UpdateOrgNameLatin(_ context.Context, _, _, _ string) ([]eid.Representation, error) {
	return f.Reps, nil
}

// Person* fake-ууд — default nil/тэг (PKI боломжийг туршихгүй тестүүдэд хангалттай).
func (f *FakeEID) PersonSummary(_ context.Context, _ string) (*eid.PersonSummary, error) {
	return f.Summary, nil
}
func (f *FakeEID) PersonCertificates(_ context.Context, _ string) (*eid.PersonCertificates, error) {
	return f.Certs, nil
}
func (f *FakeEID) PersonDevices(_ context.Context, _ string) (*eid.PersonDevices, error) {
	return f.Devices, nil
}
func (f *FakeEID) PersonActivity(_ context.Context, _ string, _, _ int) (*eid.PersonActivity, error) {
	return f.Activity, nil
}

// NewAuthFixture нь хоёр bounded context-г шинэ Postgres + Redis
// контейнеруудтай холбоно. Тохируулж болох тохиргоонууд (OTP оролдлого,
// JWT secret-ийн урт, bcrypt cost) нь боломжийн өгөгдмөл утгуудаас
// seed хийгддэг — тэдгээрийг өөрчлөх шаардлагатай тестүүд дуудахаасаа
// өмнө config.AppConfig-г шууд дарж бичиж болно.
func NewAuthFixture(t *testing.T) *AuthFixture {
	t.Helper()
	db := StartPostgres(t)
	redis := StartRedis(t)

	if config.AppConfig.OTPMaxAttempts == 0 {
		config.AppConfig.OTPMaxAttempts = 5
	}
	if config.AppConfig.REDISExpired == 0 {
		config.AppConfig.REDISExpired = 5
	}
	if config.AppConfig.BcryptCost == 0 {
		// register нь дуудалт бүрт 100ms+ нэмэхгүй байхын тулд тестүүдэд
		// cost-г бууруул.
		config.AppConfig.BcryptCost = 4
	}
	if config.AppConfig.JWTSecret == "" {
		config.AppConfig.JWTSecret = "integration-test-secret-thirty-two-chars!"
	}
	if config.AppConfig.JWTIssuer == "" {
		config.AppConfig.JWTIssuer = "integration-test"
	}
	if config.AppConfig.JWTExpired == 0 {
		config.AppConfig.JWTExpired = 1
	}
	if config.AppConfig.JWTRefreshExpired == 0 {
		config.AppConfig.JWTRefreshExpired = 7
	}

	ristretto, err := caches.NewRistrettoCache()
	require.NoError(t, err)

	jwtSvc := jwt.NewJWTServiceWithRefresh(
		config.AppConfig.JWTSecret,
		config.AppConfig.JWTIssuer,
		config.AppConfig.JWTExpired,
		config.AppConfig.JWTRefreshExpired,
	)

	verifier := &FakeVerifier{}
	eidClient := &FakeEID{}
	repo := userspostgres.NewUserRepository(db)
	usersUC := users.NewUsecase(repo, ristretto, users.Config{
		BcryptCost: config.AppConfig.BcryptCost,
	})
	authUC := auth.NewUsecase(usersUC, jwtSvc, verifier, eidClient, nil, google.NewClient("", ""), redis, auth.Config{
		OTPMaxAttempts:    5,
		OTPTTL:            5 * time.Minute,
		PasswordResetTTL:  30 * time.Minute,
		BcryptCost:        config.AppConfig.BcryptCost,
		LoginMaxAttempts:  10,
		LoginLockoutTTL:   15 * time.Minute,
		ForgotMaxAttempts: 3,
		ForgotLockoutTTL:  15 * time.Minute,
		EIDCallbackURL:    "https://template.dgov.mn/login/verify",
		EIDDisplayText:    "template.dgov.mn",
	})

	return &AuthFixture{
		Auth:     authUC,
		Users:    usersUC,
		Verifier: verifier,
		EID:      eidClient,
		JWT:      jwtSvc,
	}
}

//go:build integration

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package testenv

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/internal/config"
	"template/internal/datasources/caches"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	"template/pkg/jwt"
)

// AuthFixture нь end-to-end тестүүдэд ашиглагддаг бүрэн холбогдсон auth
// хэсэг юм: жинхэнэ Postgres, жинхэнэ Redis, жинхэнэ Ristretto, жинхэнэ
// JWT — зөвхөн гадагш чиглэсэн SMTP mailer л хуурамчаар хийгдсэн, учир
// нь бид OTP кодуудыг барьж аваад VerifyOTP руу буцаан өгөх хэрэгтэй
// бөгөөд SMTP-г CI-д ажиллуулах нь ямар ч байсан үнэ цэнэгүй.
//
// Хоёр bounded context хоёулаа илчлэгдсэн: туршиж буй auth урсгалуудад
// Auth, хэрэглэгчийн бичлэгүүдийг шууд унших эсвэл өөрчлөх шаардлагатай
// аливаа тохиргоо / баталгаажуулалтын алхамд Users.
type AuthFixture struct {
	Auth   auth.Usecase
	Users  users.Usecase
	Mailer *CapturingMailer
	JWT    jwt.JWTService
}

// CapturingMailer нь OTP+хүлээн авагч хос бүрийг бүртгэдэг тул тестүүд
// 6 санамсаргүй оронг таахын оронд кодыг гаргаж авч чадна.
type CapturingMailer struct {
	mu       sync.Mutex
	captured []otpCapture
}

type otpCapture struct{ Code, Receiver string }

// SendOTP нь mailer.OTPMailer-г хангадаг. Үргэлж амжилттай болдог.
func (m *CapturingMailer) SendOTP(_ context.Context, code, receiver string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.captured = append(m.captured, otpCapture{Code: code, Receiver: receiver})
	return nil
}

func (m *CapturingMailer) SendPasswordReset(ctx context.Context, token, receiver string) error {
	return m.SendOTP(ctx, token, receiver)
}

// LastOTP нь хүлээн авагчид зориулж хамгийн сүүлд барьж авсан OTP-г
// буцаана, эсвэл нэг ч илгээгээгүй бол тестийг унагана.
func (m *CapturingMailer) LastOTP(t *testing.T, receiver string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.captured) - 1; i >= 0; i-- {
		if m.captured[i].Receiver == receiver {
			return m.captured[i].Code
		}
	}
	t.Fatalf("no OTP captured for %s", receiver)
	return ""
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

	mailer := &CapturingMailer{}
	repo := userspostgres.NewUserRepository(db)
	usersUC := users.NewUsecase(repo, ristretto, users.Config{
		BcryptCost: config.AppConfig.BcryptCost,
	})
	authUC := auth.NewUsecase(usersUC, jwtSvc, mailer, redis, auth.Config{
		OTPMaxAttempts:    5,
		OTPTTL:            5 * time.Minute,
		PasswordResetTTL:  30 * time.Minute,
		BcryptCost:        config.AppConfig.BcryptCost,
		LoginMaxAttempts:  10,
		LoginLockoutTTL:   15 * time.Minute,
		ForgotMaxAttempts: 3,
		ForgotLockoutTTL:  15 * time.Minute,
	})

	return &AuthFixture{
		Auth:   authUC,
		Users:  usersUC,
		Mailer: mailer,
		JWT:    jwtSvc,
	}
}

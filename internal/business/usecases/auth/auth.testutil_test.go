// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/test/mocks"
	"template/pkg/helpers"
)

// fixture нь auth багцын тест тус бүрийн холболт юм. Тест бүр newFixture()-ээр
// дамжуулан шинэ mock-уудын багц үүсгэдэг тул тестүүдийн хооронд хуваалцсан
// төлөв байхгүй.
type fixture struct {
	usecase auth.Usecase
	users   *mocks.UsersUsecase
	jwt     *mocks.JWTService
	mailer  *mocks.OTPMailer
	redis   *mocks.RedisCache
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	usersUC := mocks.NewUsersUsecase(t)
	jwtSvc := mocks.NewJWTService(t)
	otpMailer := mocks.NewOTPMailer(t)
	redis := mocks.NewRedisCache(t)
	return &fixture{
		usecase: auth.NewUsecase(usersUC, jwtSvc, otpMailer, redis, auth.Config{
			OTPMaxAttempts:    5,
			OTPTTL:            5 * time.Minute,
			PasswordResetTTL:  30 * time.Minute,
			BcryptCost:        bcrypt.MinCost,
			LoginMaxAttempts:  5,
			LoginLockoutTTL:   15 * time.Minute,
			ForgotMaxAttempts: 3,
			ForgotLockoutTTL:  15 * time.Minute,
		}),
		users:  usersUC,
		jwt:    jwtSvc,
		mailer: otpMailer,
		redis:  redis,
	}
}

// activeUser нь мэдэгдэж буй энгийн текст нууц үгтэй ("Pwd_123!") тогтвортой
// хэрэглэгчийн бичлэгийг буцаадаг бөгөөд түүний bcrypt hash-ийг нэг удаа
// тооцоолж, тестүүдийн хооронд дахин ашигладаг.
func activeUser(t *testing.T) domain.User {
	t.Helper()
	hash, err := helpers.GenerateHash("Pwd_123!")
	if err != nil {
		t.Fatalf("hash sample password: %v", err)
	}
	return domain.User{
		ID:       "user-1",
		Username: "patrick",
		Email:    "patrick@example.com",
		Password: hash,
		Active:   true,
		RoleID:   2,
	}
}

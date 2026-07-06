// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/test/mocks"
	"template/pkg/google"
	"template/pkg/helpers"
)

// fixture нь auth багцын тест тус бүрийн холболт юм. Тест бүр newFixture()-ээр
// дамжуулан шинэ mock-уудын багц үүсгэдэг тул тестүүдийн хооронд хуваалцсан
// төлөв байхгүй.
type fixture struct {
	usecase  auth.Usecase
	users    *mocks.UsersUsecase
	jwt      *mocks.JWTService
	verifier *mocks.Verifier
	eid      *mocks.EIDClient
	redis    *mocks.RedisCache
	google   *fakeGoogle
}

// fakeGoogle нь auth.GoogleClient-ийн тест fake — Exchange-ийн хариуг тохируулна.
type fakeGoogle struct {
	user *google.User
	err  error
}

func (f *fakeGoogle) Configured() bool { return true }
func (f *fakeGoogle) Exchange(_ context.Context, _, _ string) (*google.User, error) {
	return f.user, f.err
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	usersUC := mocks.NewUsersUsecase(t)
	jwtSvc := mocks.NewJWTService(t)
	verifier := mocks.NewVerifier(t)
	eidClient := mocks.NewEIDClient(t)
	redis := mocks.NewRedisCache(t)
	fg := &fakeGoogle{}
	return &fixture{
		usecase: auth.NewUsecase(usersUC, jwtSvc, verifier, eidClient, nil, fg, redis, auth.Config{
			OTPMaxAttempts:    5,
			OTPTTL:            5 * time.Minute,
			PasswordResetTTL:  30 * time.Minute,
			BcryptCost:        bcrypt.MinCost,
			LoginMaxAttempts:  5,
			LoginLockoutTTL:   15 * time.Minute,
			ForgotMaxAttempts: 3,
			ForgotLockoutTTL:  15 * time.Minute,
			EIDCallbackURL:    "https://template.gerege.mn/login/verify",
			EIDDisplayText:    "template.gerege.mn",
		}),
		users:    usersUC,
		jwt:      jwtSvc,
		verifier: verifier,
		eid:      eidClient,
		redis:    redis,
		google:   fg,
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
		RoleID:   domain.RoleUser,
	}
}

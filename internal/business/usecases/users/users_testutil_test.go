// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users_test

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
	"template/internal/test/mocks"
)

// fixture нь тест тус бүрийн холболт юм. Тест бүр цэвэр mock-уудын багц авахын
// тулд newFixture()-ийг дуудна — тестүүдийн хооронд хуваалцсан хувьсагч төлөв
// байхгүй.
type fixture struct {
	usecase users.Usecase
	repo    *mocks.UserRepository
	rc      *mocks.RistrettoCache
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := mocks.NewUserRepository(t)
	rc := mocks.NewRistrettoCache(t)
	return &fixture{
		// MinCost нь Store-ийн bcrypt hashing-ийг тестэд хурдан байлгана (12
		// гэсэн production cost дээрх ~80мс-ийн оронд ~1мс). Зан төлөв ижил.
		usecase: users.NewUsecase(repo, rc, users.Config{BcryptCost: bcrypt.MinCost}),
		repo:    repo,
		rc:      rc,
	}
}

// sampleUser нь бэлэн repo гаралт болгон ашигладаг тогтвортой UserDomain буцаана.
func sampleUser() domain.User {
	return domain.User{
		ID:        "11111111-1111-1111-1111-111111111111",
		Username:  "patrick",
		Email:     "patrick@example.com",
		Password:  "$2a$10$hashedpasswordhashedpasswordhash",
		Active:    true,
		RoleID:    domain.RoleUser,
		CreatedAt: time.Now(),
	}
}

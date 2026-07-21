// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package helpers

import (
	"errors"

	"template/internal/config"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost нь тохируулсан cost-г буцаах бөгөөд config эхлүүлэгдээгүй үед
// (тест, .env ачаалалгүйгээр helpers-г import хийдэг хэрэгслүүд) bcrypt.DefaultCost
// руу шилждэг. bcrypt-ийн өөрийнх нь хүчинтэй мужид хязгаарласан тул муу
// тохиргоо үүнийг panic болгож чадахгүй.
func bcryptCost() int {
	c := config.AppConfig.BcryptCost
	if c < bcrypt.MinCost || c > bcrypt.MaxCost {
		return bcrypt.DefaultCost
	}
	return c
}

func GenerateHash(passwd string) (string, error) {
	if passwd == "" {
		return "", errors.New("password cannot empty")
	}

	result, err := bcrypt.GenerateFromPassword([]byte(passwd), bcryptCost())
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func ValidateHash(secret, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret))
	return err == nil
}

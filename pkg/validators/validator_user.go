// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package validators

import (
	"unicode"

	"github.com/go-playground/validator/v10"
)

// StrongPassword нь нууц үгийн хамгийн бага нарийн төвөгтэй байдлыг шаардана:
// дор хаяж нэг том үсэг, нэг жижиг үсэг, нэг цифр болон нэг тусгай тэмдэгт.
// Уртыг "min" tag-аар тусад нь шаарддаг.
func StrongPassword(fl validator.FieldLevel) bool {
	password, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSpecial
}

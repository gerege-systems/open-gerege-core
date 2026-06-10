// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package validators

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"template/pkg/helpers"
)

// FieldError нь бүтэлгүй болсон нэг баталгаажуулалтын дүрмийг тодорхойлно. API
// хариунд буцаагдах бөгөөд ингэснээр клиент тэгш (flat) тэмдэгт мөр задлан
// уншихгүйгээр аль талбар буруу байгааг мэдэх боломжтой.
type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

// ValidationErrors нь ValidatePayloads-ийн буцаадаг бүтэцлэгдсэн бүтэлгүйтлийн
// утга юм. Энэ нь `error`-г хэрэгжүүлдэг тул алдаа буцаах хэвшмэл бичлэгийн
// үлдсэн хэсэгтэй жам ёсоор зохицдог; handler-ууд type-assert хийж, талбар
// тус бүрийн дэлгэрэнгүйг дүрсэлж болно.
type ValidationErrors struct {
	Errors []FieldError
}

func (v *ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return "validation failed"
	}
	parts := make([]string, 0, len(v.Errors))
	for _, e := range v.Errors {
		parts = append(parts, e.Field+": "+e.Message)
	}
	return strings.Join(parts, "; ")
}

var mapHelper = map[string]string{
	"required":       "is a required field",
	"email":          "is not a valid email address",
	"lowercase":      "must contain at least one lowercase letter",
	"uppercase":      "must contain at least one uppercase letter",
	"numeric":        "must contain at least one digit",
	"strongpassword": "must contain uppercase, lowercase, digit, and special character",
}

var needParam = []string{"min", "max", "containsany"}

// sharedValidate нь дуудлагуудын дунд дахин ашиглагддаг; validator.New() нь
// харьцангуй өртөгтэй бөгөөд нэг удаа байгуулагдсаны дараа зэрэгцээ ашиглахад аюулгүй.
var (
	sharedValidate     *validator.Validate
	sharedValidateOnce sync.Once
)

func getValidator() *validator.Validate {
	sharedValidateOnce.Do(func() {
		sharedValidate = validator.New()
		_ = sharedValidate.RegisterValidation("strongpassword", StrongPassword)
		// Талбарын нэрийг json tag-аар буцаана (жишээ: NewPassword →
		// "new_password") — ингэснээр клиент талбар тус бүрийн алдааг DTO-ийн
		// json түлхүүрээр шууд тааруулна.
		sharedValidate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" || name == "" {
				return strings.ToLower(fld.Name)
			}
			return name
		})
	})
	return sharedValidate
}

// ValidatePayloads нь struct-ийн validate tag-уудыг ажиллуулж, амжилттай үед
// nil буцаана. Бүтэлгүй үед бүтэлгүй болсон талбар тус бүрт нэг бичлэгтэй
// *ValidationErrors-г буцаадаг тул дуудагчид бүтэцлэгдсэн хариу дүрсэлж болно.
func ValidatePayloads(payload interface{}) error {
	if err := getValidator().Struct(payload); err != nil {
		var ve validator.ValidationErrors
		if !errors.As(err, &ve) {
			return err
		}
		out := &ValidationErrors{Errors: make([]FieldError, 0, len(ve))}
		for _, e := range ve {
			out.Errors = append(out.Errors, FieldError{
				Field:   e.Field(), // json tag (RegisterTagNameFunc-ээр)
				Tag:     e.Tag(),
				Message: messageFor(e),
			})
		}
		return out
	}
	return nil
}

// messageFor нь тэгш (flat) .Error() тэмдэгт мөр болон бүтэцлэгдсэн
// FieldError.Message хоёуланд ашиглагддаг хүн уншихад ойлгомжтой мөрийг үүсгэнэ.
func messageFor(e validator.FieldError) string {
	tag := e.Tag()
	param := e.Param()

	value := ""
	if s, ok := e.Value().(string); ok {
		value = s
	}

	if helpers.IsArrayContains(needParam, tag) {
		return paramMessage(value, tag, param)
	}
	if msg, ok := mapHelper[tag]; ok {
		if value != "" {
			return fmt.Sprintf("'%s' %s", value, msg)
		}
		return msg
	}
	return fmt.Sprintf("failed validation on %q", tag)
}

func paramMessage(value, tag, param string) string {
	switch tag {
	case "min":
		return fmt.Sprintf("must be at least %s characters long", param)
	case "max":
		return fmt.Sprintf("must be less than %s characters", param)
	case "containsany":
		return fmt.Sprintf("must contain at least one symbol of '%s'", param)
	}
	return fmt.Sprintf("failed %s validation", tag)
}

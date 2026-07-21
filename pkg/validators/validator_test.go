// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Validator-ийн unit тест: StrongPassword-ийн нарийн төвөгтэй байдлын дүрэм,
// ValidatePayloads-ийн бүтэцлэгдсэн FieldError (json tag-аар талбарын нэр),
// required/min/email/strongpassword tag-уудын хэрэгжилт.
package validators_test

import (
	"errors"
	"testing"

	"template/pkg/validators"
)

func TestStrongPassword(t *testing.T) {
	type dto struct {
		Password string `json:"password" validate:"strongpassword"`
	}
	cases := map[string]bool{
		"Abcdef1!": true,  // бүх ангилал
		"abcdef1!": false, // том үсэггүй
		"ABCDEF1!": false, // жижиг үсэггүй
		"Abcdefg!": false, // цифргүй
		"Abcdef12": false, // тусгай тэмдэггүй
		"":         false,
	}
	for pw, wantValid := range cases {
		err := validators.ValidatePayloads(dto{Password: pw})
		if wantValid && err != nil {
			t.Errorf("%q: хүчинтэй байх ёстой, авсан %v", pw, err)
		}
		if !wantValid && err == nil {
			t.Errorf("%q: хүчингүй байх ёстой", pw)
		}
	}
}

func TestValidatePayloadsStructuredErrors(t *testing.T) {
	type dto struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}
	err := validators.ValidatePayloads(dto{Email: "not-an-email", Password: "x"})
	var ve *validators.ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("*ValidationErrors хүлээж байсан, авсан %T: %v", err, err)
	}
	if len(ve.Errors) != 2 {
		t.Fatalf("2 талбарын алдаа хүлээсэн, авсан %d: %+v", len(ve.Errors), ve.Errors)
	}
	// Талбарын нэр json tag-аар (Field/field биш) буцаагдана.
	fields := map[string]bool{}
	for _, e := range ve.Errors {
		fields[e.Field] = true
	}
	if !fields["email"] || !fields["password"] {
		t.Errorf("талбарын нэрс json tag-аар байх ёстой, авсан %+v", ve.Errors)
	}
}

func TestValidatePayloadsSuccess(t *testing.T) {
	type dto struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := validators.ValidatePayloads(dto{Email: "user@example.com"}); err != nil {
		t.Fatalf("хүчинтэй payload алдаа өгөв: %v", err)
	}
}

func TestValidationErrorsErrorString(t *testing.T) {
	ve := &validators.ValidationErrors{Errors: []validators.FieldError{
		{Field: "email", Tag: "email", Message: "invalid email"},
	}}
	if ve.Error() == "" {
		t.Error("Error() хоосон байх ёсгүй")
	}
	empty := &validators.ValidationErrors{}
	if empty.Error() == "" {
		t.Error("хоосон ValidationErrors ч мессежтэй байх ёстой")
	}
}

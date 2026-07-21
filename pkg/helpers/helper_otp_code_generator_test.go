// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package helpers_test

import (
	"testing"

	"template/pkg/helpers"
)

func TestGenerateOTPCode(t *testing.T) {
	length := 6

	code1, err := helpers.GenerateOTPCode(length)
	if err != nil {
		t.Error("error occurred while generating code 1")
	}

	code2, err := helpers.GenerateOTPCode(length)
	if err != nil {
		t.Error("error occurred while generating code 2")
	}

	if code1 == code2 {
		t.Error("function have to generate difference code when execute twice ")
	}

	if len(code1) != length {
		t.Error("invalid length")
	}
}

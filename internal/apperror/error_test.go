// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// apperror-ийн unit тест: constructor бүрийн Type, InternalCause-ийн cause
// нуулт (клиент рүү generic мессеж), Wrap/Unwrap-ийн errors.Is/As гинж.
package apperror_test

import (
	"errors"
	"fmt"
	"testing"

	"template/internal/apperror"
)

func TestConstructorsSetType(t *testing.T) {
	cases := []struct {
		err  *apperror.DomainError
		want apperror.ErrorType
	}{
		{apperror.NotFound("x"), apperror.ErrTypeNotFound},
		{apperror.Unauthorized("x"), apperror.ErrTypeUnauthorized},
		{apperror.Forbidden("x"), apperror.ErrTypeForbidden},
		{apperror.Conflict("x"), apperror.ErrTypeConflict},
		{apperror.BadRequest("x"), apperror.ErrTypeBadRequest},
		{apperror.Internal("x"), apperror.ErrTypeInternal},
	}
	for _, tc := range cases {
		if tc.err.Type != tc.want {
			t.Errorf("Type = %v, want %v", tc.err.Type, tc.want)
		}
		if tc.err.Error() != "x" {
			t.Errorf("Error() = %q, want x", tc.err.Error())
		}
	}
}

func TestInternalCauseHidesCause(t *testing.T) {
	cause := errors.New("pq: connection refused to 10.0.0.1")
	err := apperror.InternalCause(cause)

	if err.Type != apperror.ErrTypeInternal {
		t.Errorf("Type = %v", err.Type)
	}
	// Хэрэглэгчид харагдах мессеж нь generic — дотоод cause алдагдахгүй.
	if err.Error() != "internal server error" {
		t.Errorf("Error() = %q, дотоод cause алдагдсан байж магадгүй", err.Error())
	}
	// Cause нь errors.Is/Unwrap-аар лог/дебагт хүртээмжтэй хэвээр.
	if !errors.Is(err, cause) {
		t.Error("cause нь Unwrap-аар хүртээмжтэй байх ёстой")
	}
}

func TestWrapUnwrap(t *testing.T) {
	cause := errors.New("root")
	wrapped := apperror.Wrap(apperror.Conflict("conflict"), cause)
	if wrapped.Type != apperror.ErrTypeConflict {
		t.Errorf("Wrap нь Type-ийг хадгалах ёстой, авсан %v", wrapped.Type)
	}
	if !errors.Is(wrapped, cause) {
		t.Error("Wrap-ийн cause нь errors.Is-ээр хүртээмжтэй байх ёстой")
	}
}

func TestErrorsAsThroughFmtWrap(t *testing.T) {
	err := fmt.Errorf("usecase failed: %w", apperror.BadRequest("bad input"))
	var domErr *apperror.DomainError
	if !errors.As(err, &domErr) {
		t.Fatal("errors.As нь fmt.Errorf-ийн %w-ийг нэвтлэх ёстой")
	}
	if domErr.Type != apperror.ErrTypeBadRequest {
		t.Errorf("Type = %v", domErr.Type)
	}
}

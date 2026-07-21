// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Хариу дугтуй + алдаа→HTTP статус буулгалтын unit тест. Энэ давхарга нь бүх
// handler-ийн нийтлэг гаралт тул: domain алдааны төрөл бүр зөв статус авах,
// validation алдаа 422 + талбарын дэлгэрэнгүй, дотоод (5xx) алдааны cause
// клиент рүү алдагдахгүй (generic мессеж), амжилтын дугтуйн бүтэц.
package v1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"template/internal/apperror"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("хариу JSON биш: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestRespondWithErrorStatusMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"not found → 404", apperror.NotFound("nope"), http.StatusNotFound},
		{"unauthorized → 401", apperror.Unauthorized("no"), http.StatusUnauthorized},
		{"forbidden → 403", apperror.Forbidden("no"), http.StatusForbidden},
		{"conflict → 409", apperror.Conflict("dup"), http.StatusConflict},
		{"bad request → 400", apperror.BadRequest("bad"), http.StatusBadRequest},
		{"internal → 500", apperror.Internal("x"), http.StatusInternalServerError},
		{"plain error → 500", errAny("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			_ = v1.RespondWithError(rec, r, tc.err)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			body := decodeBody(t, rec)
			if body["status"] != false {
				t.Errorf("алдааны хариунд status=false байх ёстой")
			}
		})
	}
}

func TestRespondWithErrorHidesInternalCause(t *testing.T) {
	secret := errAny("pq: password=hunter2 host=10.0.0.5")
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	_ = v1.RespondWithError(rec, r, apperror.InternalCause(secret))

	body := decodeBody(t, rec)
	msg, _ := body["message"].(string)
	if msg != "internal server error" {
		t.Fatalf("дотоод мессеж = %q — cause алдагдсан байж магадгүй", msg)
	}
	if got := rec.Body.String(); contains(got, "hunter2") || contains(got, "10.0.0.5") {
		t.Errorf("дотоод cause хариунд алдагдсан: %s", got)
	}
}

func TestRespondWithValidationErrors(t *testing.T) {
	ve := &validators.ValidationErrors{Errors: []validators.FieldError{
		{Field: "email", Tag: "email", Message: "invalid email"},
	}}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	_ = v1.RespondWithError(rec, r, ve)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validation → status = %d, want 422", rec.Code)
	}
	body := decodeBody(t, rec)
	data, _ := body["data"].(map[string]any)
	if _, ok := data["errors"]; !ok {
		t.Errorf("422 хариунд талбарын алдаа (data.errors) байх ёстой: %v", body)
	}
}

func TestSuccessResponseEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	_ = v1.NewSuccessResponse(rec, r, http.StatusOK, "ok", map[string]string{"k": "v"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["status"] != true || body["message"] != "ok" {
		t.Errorf("амжилтын дугтуй буруу: %v", body)
	}
	data, _ := body["data"].(map[string]any)
	if data["k"] != "v" {
		t.Errorf("data = %v", body["data"])
	}
}

// errAny нь domain биш энгийн алдаа.
type errAny string

func (e errAny) Error() string { return string(e) }

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

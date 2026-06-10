// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"template/internal/http/middlewares"
)

// chi port-ийн тэмдэглэл: анхны Gin тест нь цэвэр
// AccessLogFormatter(gin.LogFormatterParams) функцийг шалгадаг байсан.
// net/http-д ижил төстэй formatter hook байхгүй — access log нь middleware
// дотроо гардаг — тиймээс formatter-ийн нэгж тестийг middleware нь доош
// дамжих handler-ийн статус код эсвэл body-г өөрчилдөггүй бөгөөд гинж нь
// амжилт болон алдааны статусуудын аль алинд нь дуустлаа ажилладгийг
// баталгаажуулдаг зан төлөвийн дамжуулах тестээр сольсон.
func TestAccessLogMiddleware_PassesThrough(t *testing.T) {
	mw := middlewares.AccessLogMiddleware()

	serve := func(t *testing.T, status int, body string) *httptest.ResponseRecorder {
		t.Helper()
		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("2xx passes through unchanged", func(t *testing.T) {
		rec := serve(t, http.StatusOK, "ok")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "ok", rec.Body.String())
	})

	t.Run("5xx passes through unchanged", func(t *testing.T) {
		rec := serve(t, http.StatusInternalServerError, "boom")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, "boom", rec.Body.String())
	})
}

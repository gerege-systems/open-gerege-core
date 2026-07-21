// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"template/internal/config"
	"template/internal/constants"
	"template/internal/http/middlewares"

	"github.com/stretchr/testify/assert"
)

func serveSec(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	mw := middlewares.SecurityHeadersMiddleware()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", http.NoBody))
	return rec
}

func TestSecurityHeaders_DevDefaults(t *testing.T) {
	config.AppConfig.Environment = constants.EnvironmentDevelopment
	rec := serveSec(t)

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'none'")
	// HSTS-г development-д илгээх ЁСГҮЙ — localhost browser-уудад энгийн
	// HTTP-г татгалзахыг заах болно.
	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_ProductionAddsHSTS(t *testing.T) {
	config.AppConfig.Environment = constants.EnvironmentProduction
	t.Cleanup(func() { config.AppConfig.Environment = constants.EnvironmentDevelopment })

	rec := serveSec(t)

	hsts := rec.Header().Get("Strict-Transport-Security")
	assert.Contains(t, hsts, "max-age=")
	assert.Contains(t, hsts, "includeSubDomains")
}

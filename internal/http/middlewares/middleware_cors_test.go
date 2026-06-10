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

func TestCORSMiddleware(t *testing.T) {
	mw := middlewares.CORSMiddleware()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("test"))
	})
	h := mw(next)

	t.Run("Test 1 | CORS Headers Are Set", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	})
	t.Run("Test 2 | Preflight OPTIONS Returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", http.NoBody)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	})
	t.Run("Test 3 | Extra Headers Are Not Blocked", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("X-Custom-Header", "something")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

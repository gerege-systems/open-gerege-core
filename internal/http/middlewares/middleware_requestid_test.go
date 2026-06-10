// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"template/internal/http/middlewares"
	"template/pkg/logger"
)

func TestRequestIDMiddleware_BridgesIDsToContext(t *testing.T) {
	mw := middlewares.RequestIDMiddleware()

	var seenRequestID, seenTraceID string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		seenRequestID = logger.GetRequestIDFromContext(ctx)
		seenTraceID = logger.GetTraceIDFromContext(ctx)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	req.Header.Set("X-Request-ID", "abc-from-client")
	h.ServeHTTP(rec, req)

	assert.Equal(t, "abc-from-client", seenRequestID,
		"request_id from client header must be visible to handlers via logger.GetRequestIDFromContext")
	assert.Empty(t, seenTraceID,
		"trace_id stays empty when the tracing middleware isn't mounted; populated end-to-end in production")
	assert.Equal(t, "abc-from-client", rec.Header().Get("X-Request-ID"),
		"request_id must be echoed in the response header")
}

func TestRequestIDMiddleware_GeneratesWhenAbsent(t *testing.T) {
	mw := middlewares.RequestIDMiddleware()

	var seen string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = logger.GetRequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	h.ServeHTTP(rec, req)

	assert.NotEmpty(t, seen, "middleware must generate a UUID when no header is present")
	assert.Equal(t, seen, rec.Header().Get("X-Request-ID"))
}

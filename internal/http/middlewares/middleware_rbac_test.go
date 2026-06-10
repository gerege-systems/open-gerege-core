// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"template/internal/constants"
	"template/pkg/jwt"
)

// withClaim нь auth middleware-ийн доош нийтэлдэг claim-г дуурайн context-д
// суулгана.
func withClaim(r *http.Request, claim jwt.JwtCustomClaim) *http.Request {
	ctx := context.WithValue(r.Context(), constants.CtxAuthenticatedUserKey, claim)
	return r.WithContext(ctx)
}

func TestRequireAdmin(t *testing.T) {
	reached := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name       string
		setClaim   bool
		isAdmin    bool
		wantStatus int
	}{
		{"no claim => 401", false, false, http.StatusUnauthorized},
		{"authenticated non-admin => 403", true, false, http.StatusForbidden},
		{"admin => next (200)", true, true, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
			if tc.setClaim {
				req = withClaim(req, jwt.JwtCustomClaim{UserID: "u1", Email: "a@b.mn", IsAdmin: tc.isAdmin})
			}
			rec := httptest.NewRecorder()
			RequireAdmin()(reached).ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

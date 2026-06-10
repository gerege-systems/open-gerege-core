// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics"))
	})
}

func TestObservabilityGate(t *testing.T) {
	const token = "s3cr3t-ops-token"

	cases := []struct {
		name         string
		isProduction bool
		token        string
		authHeader   string
		wantStatus   int
	}{
		{"dev always open (no token, no header)", false, "", "", http.StatusOK},
		{"dev open even with token configured", false, token, "", http.StatusOK},
		{"prod + empty token => 404 (fully closed)", true, "", "Bearer " + token, http.StatusNotFound},
		{"prod + correct token => 200", true, token, "Bearer " + token, http.StatusOK},
		{"prod + correct token, case-insensitive prefix => 200", true, token, "bearer " + token, http.StatusOK},
		{"prod + wrong token => 404", true, token, "Bearer wrong", http.StatusNotFound},
		{"prod + missing header => 404", true, token, "", http.StatusNotFound},
		{"prod + malformed header (no bearer) => 404", true, token, token, http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gate := ObservabilityGate(tc.isProduction, tc.token)
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			gate(okHandler()).ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

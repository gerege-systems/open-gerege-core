// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// RequirePermission-ийн бүрэн matrix тест: admin bypass, эрхтэй/эрхгүй role,
// хуучин токены roleID=0 fallback (=user role), resolver-ийн алдаанд
// fail-closed (403), claim-гүй хүсэлтэд 401.
package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"template/internal/business/domain"
	"template/pkg/jwt"
)

// stubResolver нь PermissionResolver-ийн хамгийн жижиг fake: roleID→эрхүүд
// эсвэл алдаа. Дуудалт бүрийг тэмдэглэж admin bypass-ийг батлахад ашиглана.
type stubResolver struct {
	perms map[int][]string
	err   error
	calls int
}

func (s *stubResolver) Resolve(_ context.Context, roleID int) ([]string, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.perms[roleID], nil
}

func TestRequirePermission(t *testing.T) {
	reached := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Seed-тэй ижил matrix: user, manager (role ID-ууд domain тогтмолоор).
	perms := map[int][]string{
		domain.RoleUser:    {"dashboard.view", "personal.view"},
		domain.RoleManager: {"dashboard.view", "manager.view", "users.manage"},
	}

	cases := []struct {
		name       string
		setClaim   bool
		claim      jwt.JwtCustomClaim
		perm       string
		resolver   *stubResolver
		wantStatus int
	}{
		{
			name: "no claim => 401", setClaim: false, perm: "users.manage",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusUnauthorized,
		},
		{
			name: "admin bypasses resolver", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u1", IsAdmin: true, RoleID: domain.RoleAdmin}, perm: "roles.manage",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusOK,
		},
		{
			name: "manager has users.manage => 200", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u2", RoleID: domain.RoleManager}, perm: "users.manage",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusOK,
		},
		{
			name: "manager lacks roles.manage => 403", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u2", RoleID: domain.RoleManager}, perm: "roles.manage",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusForbidden,
		},
		{
			name: "user lacks users.manage => 403", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u3", RoleID: domain.RoleUser}, perm: "users.manage",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusForbidden,
		},
		{
			name: "user has personal.view => 200", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u3", RoleID: domain.RoleUser}, perm: "personal.view",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusOK,
		},
		{
			name: "legacy token roleID=0 falls back to user role", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u4", RoleID: 0}, perm: "personal.view",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusOK,
		},
		{
			name: "legacy token roleID=0 still lacks admin perms", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u4", RoleID: 0}, perm: "users.manage",
			resolver: &stubResolver{perms: perms}, wantStatus: http.StatusForbidden,
		},
		{
			name: "resolver error fails closed => 403", setClaim: true,
			claim: jwt.JwtCustomClaim{UserID: "u5", RoleID: domain.RoleManager}, perm: "users.manage",
			resolver: &stubResolver{err: errors.New("db down")}, wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.setClaim {
				req = withClaim(req, tc.claim)
			}
			rec := httptest.NewRecorder()
			RequirePermission(tc.resolver, tc.perm)(reached).ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.setClaim && tc.claim.IsAdmin && tc.resolver.calls != 0 {
				t.Fatalf("admin үед resolver дуудагдах ёсгүй, дуудагдсан=%d", tc.resolver.calls)
			}
		})
	}
}

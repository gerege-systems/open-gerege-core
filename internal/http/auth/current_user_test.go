// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// CurrentUserFromContext-ийн unit тест: claim байхгүй / буруу төрөл үед
// ErrNotAuthenticated, зөв claim үед бүх талбарын зохицуулалт.
package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"template/internal/constants"
	httpauth "template/internal/http/auth"
	"template/pkg/jwt"
)

func reqWith(ctxVal any) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	if ctxVal != nil {
		r = r.WithContext(context.WithValue(r.Context(), constants.CtxAuthenticatedUserKey, ctxVal))
	}
	return r
}

func TestCurrentUserFromContext(t *testing.T) {
	t.Run("no claim → ErrNotAuthenticated", func(t *testing.T) {
		_, err := httpauth.CurrentUserFromContext(reqWith(nil))
		if !errors.Is(err, httpauth.ErrNotAuthenticated) {
			t.Fatalf("want ErrNotAuthenticated, got %v", err)
		}
	})

	t.Run("wrong type → ErrNotAuthenticated", func(t *testing.T) {
		_, err := httpauth.CurrentUserFromContext(reqWith("not-a-claim"))
		if !errors.Is(err, httpauth.ErrNotAuthenticated) {
			t.Fatalf("want ErrNotAuthenticated, got %v", err)
		}
	})

	t.Run("valid claim maps all fields", func(t *testing.T) {
		claim := jwt.JwtCustomClaim{UserID: "u1", Email: "a@b.mn", IsAdmin: true, RoleID: 3}
		claim.ID = "jti-1"
		u, err := httpauth.CurrentUserFromContext(reqWith(claim))
		if err != nil {
			t.Fatal(err)
		}
		if u.ID != "u1" || u.Email != "a@b.mn" || !u.IsAdmin || u.RoleID != 3 || u.JTI != "jti-1" {
			t.Errorf("mapped user = %+v", u)
		}
	})
}

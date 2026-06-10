// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"context"
	"net/http"

	httpauth "template/internal/http/auth"
	V1Handler "template/internal/http/handlers/v1"
)

// PermissionResolver нь нэг role-ийн эрхүүдийг буцаана (rbac.Usecase үүнийг
// хангадаг). Энд interface болгож тодорхойлсон нь import cycle-ээс сэргийлж,
// middlewares-ийг RBAC хэрэгжилтээс салгана.
type PermissionResolver interface {
	Resolve(ctx context.Context, roleID int) ([]string, error)
}

// RequirePermission нь тухайн эрхгүй хэрэглэгчийг 403-аар татгалзана.
// AuthMiddleware-ийн ДАРАА ажиллах ёстой (CurrentUser context-д байх ёстой).
// admin (IsAdmin) бүх шалгалтыг давна. Resolve алдаа гарвал fail-closed (403).
func RequirePermission(resolver PermissionResolver, perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := httpauth.CurrentUserFromContext(r)
			if err != nil {
				_ = V1Handler.NewErrorResponse(w, r, http.StatusUnauthorized, "invalid token")
				return
			}
			if user.IsAdmin {
				next.ServeHTTP(w, r)
				return
			}
			// Хуучин токенд RoleID байхгүй (=0) — энгийн хэрэглэгч (2) гэж үзнэ.
			roleID := user.RoleID
			if roleID == 0 {
				roleID = 2
			}
			perms, err := resolver.Resolve(r.Context(), roleID)
			if err != nil {
				_ = V1Handler.NewErrorResponse(w, r, http.StatusForbidden, "you don't have access for this action")
				return
			}
			for _, p := range perms {
				if p == perm {
					next.ServeHTTP(w, r)
					return
				}
			}
			_ = V1Handler.NewErrorResponse(w, r, http.StatusForbidden, "you don't have access for this action")
		})
	}
}

// RequireAdmin нь зөвхөн admin (IsAdmin) хэрэглэгчид route-д хандахыг
// зөвшөөрөх declarative authorization middleware юм. AuthMiddleware-ийн
// ДАРАА ажиллах ёстой — баталгаажсан claim (CurrentUser) context-д байх
// шаардлагатай.
//
// Хариу:
//   - claim байхгүй (auth middleware суулгаагүй / токен дээд урсгалд
//     татгалзагдсан) → 401.
//   - admin биш → 403 (fail-closed).
//   - admin → next.
//
// Жишээ ашиглалт (route-д):
//
//	r.With(authMiddleware, middlewares.RequireAdmin()).
//	    Get("/admin/users", v1.Wrap(h.ListUsers))
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := httpauth.CurrentUserFromContext(r)
			if err != nil {
				_ = V1Handler.NewErrorResponse(w, r, http.StatusUnauthorized, "invalid token")
				return
			}
			if !user.IsAdmin {
				_ = V1Handler.NewErrorResponse(w, r, http.StatusForbidden, "you don't have access for this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

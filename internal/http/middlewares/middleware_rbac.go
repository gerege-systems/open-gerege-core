// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"

	httpauth "template/internal/http/auth"
	V1Handler "template/internal/http/handlers/v1"
)

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

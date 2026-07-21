// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"template/internal/constants"
	V1Handler "template/internal/http/handlers/v1"
	"template/pkg/logger"
)

// RecovererMiddleware нь доош урсгал дахь handler/middleware-ийн panic-ийг
// барьж, stack trace + request_id-г логд бичээд клиентэд нэгдсэн 500
// BaseResponse дугтуй буцаана. net/http нь panic-ийг per-connection л
// барьдаг тул (нэгдсэн хариу/лог-гүй) — энэ нь Fiber-ийн ErrorHandler
// panic-recovery-г орлоно. http.ErrAbortHandler-г net/http-д үлдээнэ.
func RecovererMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if rec == http.ErrAbortHandler {
						panic(rec)
					}
					logger.ErrorWithContext(r.Context(), "panic recovered in HTTP handler", logger.Fields{
						constants.LoggerCategory: constants.LoggerCategoryHTTP,
						"path":                   r.URL.Path,
						"panic":                  fmt.Sprintf("%v", rec),
						"stack":                  string(debug.Stack()),
					})
					_ = V1Handler.NewErrorResponse(w, r, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

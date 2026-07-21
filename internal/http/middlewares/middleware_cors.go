// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"
	"strconv"
	"strings"

	"template/internal/config"
)

// CORS тохиргооны тогтмол утгууд. Эдгээр нь өмнөх Fiber cors.Config-ийн
// утгуудтай яг таарна.
var (
	corsAllowMethods  = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsAllowHeaders  = []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "Accept", "Cache-Control", "X-Requested-With", "X-Request-ID"}
	corsExposeHeaders = []string{"Content-Length", "X-Request-ID"}
	corsMaxAge        = 12 * 60 * 60 // 12 цаг, секундээр
)

// CORSMiddleware нь тохируулсан зөвшөөрөгдсөн origin-уудын жагсаалтаас
// CORS header-уудыг тогтоодог chi middleware бүтээдэг. Цорын ганц origin нь
// wildcard "*" байх үед credentials идэвхгүй болдог (спецификаци нь
// credentials + wildcard-г хориглодог); тодорхой allow-list-д credentials
// идэвхждэг.
func CORSMiddleware() func(http.Handler) http.Handler {
	origins := config.AppConfig.AllowedOriginsList()

	allowAll := len(origins) == 1 && origins[0] == "*"
	allowCredentials := !allowAll

	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[o] = struct{}{}
	}

	allowMethods := strings.Join(corsAllowMethods, ", ")
	allowHeaders := strings.Join(corsAllowHeaders, ", ")
	exposeHeaders := strings.Join(corsExposeHeaders, ", ")
	maxAge := strconv.Itoa(corsMaxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// origin-г тусгах ёстой эсэхийг шийднэ. wildcard горимд бид "*"
			// буцаана; allow-list горимд зөвхөн жагсаалтад буй origin-г
			// эгшиглүүлнэ (Vary: Origin-тэй).
			var allowOrigin string
			if allowAll {
				allowOrigin = "*"
			} else if origin != "" {
				if _, ok := allowed[origin]; ok {
					allowOrigin = origin
				}
			}

			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				if !allowAll {
					w.Header().Add("Vary", "Origin")
				}
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
			}

			// Preflight (OPTIONS) хүсэлтэд богино хариулна — зөвшөөрөгдсөн
			// method / header / max-age-г зарлаж 204-ээр төгсгөнө.
			if r.Method == http.MethodOptions {
				if allowOrigin != "" {
					w.Header().Set("Access-Control-Allow-Methods", allowMethods)
					w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
					w.Header().Set("Access-Control-Max-Age", maxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

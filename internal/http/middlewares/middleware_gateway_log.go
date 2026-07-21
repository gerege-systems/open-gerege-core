// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// GatewayRequestRecorder нь нэг бодит /api хүсэлтийн телеметрийг хүлээн авна.
// Хэрэгжүүлэлт (server.go) үүнийг detached context дээр async бичдэг тул
// хариуны хоцролт нэмэгдэхгүй.
type GatewayRequestRecorder func(method, path, clientIP string, status, latencyMS int)

// isRPGatewayPath нь тухайн зам гуравдагч талын RP-ийн gateway хүсэлт мөн эсэхийг
// шалгана. Зөвхөн эдгээрийг лог-лоно — DAN-ий ӨӨРИЙН админ/апп-ын дотоод API
// (rbac/users/themes/gateway/applications г.м.) лог-д ОРОХГҮЙ:
//   - /rp/sign        — RP-ийн eID цахим гарын үсэг relay
//   - /api/v1/provider — RP-ийн OIDC (Login with DAN) login/consent
func isRPGatewayPath(p string) bool {
	return strings.HasPrefix(p, "/rp/sign") || strings.HasPrefix(p, "/api/v1/provider")
}

// GatewayRequestLogMiddleware нь гуравдагч талын RP-ийн gateway хүсэлт бүрийг
// (method/path/status/latency/ip) API Gateway-ийн хүсэлтийн лог руу бичнэ.
// DAN-ий өөрийн first-party API трафикийг лог-лохгүй (isRPGatewayPath).
func GatewayRequestLogMiddleware(record GatewayRequestRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isRPGatewayPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			record(r.Method, r.URL.Path, clientIP(r), status, int(time.Since(start).Milliseconds()))
		})
	}
}

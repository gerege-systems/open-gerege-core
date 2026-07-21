// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"fmt"
	"net/http"
	"time"

	"template/pkg/logger"

	"github.com/go-chi/chi/v5/middleware"
)

// access-log өнгөний кодууд (xterm SGR background).
const (
	accessLogRed    = "41"
	accessLogYellow = "43"
	accessLogGreen  = "42"
)

// AccessLogMiddleware нь хүсэлт тус бүрд нэг мөр access log үзүүлнэ.
// Статус код өнгөтэй болгогдсон тул энгийн `tail -f` session-д 5xx / 4xx
// тодорч харагдана. Энэ нь Gin-ийн LoggerWithFormatter(AccessLogFormatter)-ийн
// net/http-д төрөлх орлуулагч юм.
func AccessLogMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Статус кодыг барихын тулд хариуны бичигчийг ороодог.
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			latency := time.Since(start)
			status := ww.Status()
			if status == 0 {
				// Handler статус кодыг шууд бичээгүй бол net/http нь 200-г
				// илгээдэг.
				status = http.StatusOK
			}

			var color string
			switch {
			case status >= 500:
				color = accessLogRed
			case status >= 400:
				color = accessLogYellow
			default:
				color = accessLogGreen
			}

			requestID := "-"
			if v, ok := r.Context().Value(logger.RequestIDKey).(string); ok && v != "" {
				requestID = v
			}

			fmt.Printf("[LOGGING HTTP] [%s] req=%s \033[%sm %d \033[0m %s %s %s %s %s\n",
				start.Format("2006-01-02 15:04:05"),
				requestID,
				color,
				status,
				r.Method,
				r.URL.Path,
				latency,
				clientIP(r),
				r.Header.Get("User-Agent"),
			)
		})
	}
}

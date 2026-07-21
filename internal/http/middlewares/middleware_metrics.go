// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

// MetricsMiddleware нь route тус бүрийн хүсэлтийн тоо + үргэлжлэх
// хугацааг бүртгэдэг. Өндөр-кардиналтай path параметрүүд метрик цувааг
// тэсрүүлэхгүйн тулд таарсан route загвар (chi RouteContext-ийн
// RoutePattern)-г path шошго болгон ашигладаг.
func MetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			statusCode := ww.Status()
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			status := strconv.Itoa(statusCode)

			// Route загварыг chi RouteContext-оос унших нь next ажилласны
			// дараа боломжтой (router энэ үед хүсэлтийг тааруулсан байна).
			path := chi.RouteContext(r.Context()).RoutePattern()
			if path == "" {
				path = "unknown"
			}

			httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		})
	}
}

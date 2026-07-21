// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"

	"template/pkg/observability"

	"github.com/go-chi/chi/v5/middleware"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware нь net/http-д зориулсан гар хийцийн OpenTelemetry
// middleware юм.
//
// Үндэслэл: otelgin (анхны boilerplate-д ашигласан Gin хэмжилт) нь
// арчилгаатай хувилбаргүй; otelhttp.NewHandler-г шууд ашиглаж болох ч бид
// span нэр / attribute-уудаа яг хяналтад байлгахын тулд хүсэлт тус бүрд
// глобал tracer-ээс (pkg/observability.SetupTracing-ээр тохируулагдсан)
// span эхлүүлдэг. Tracing идэвхгүй үед глобал provider нь OTel-ийн no-op
// байх тул энэ нь бараг ямар ч зардалгүй.
//
// Үүнийг ЭХЭНД суулга — ингэснээр RequestIDMiddleware нь span context
// (trace_id)-г logger context руу гүүрлэхээр хүрэхээс өмнө тогтоогдох
// бөгөөд stack-ийн доор гарсан span-ууд (DB, Redis) энэ серверийн
// span-ийн child болж автоматаар үүснэ.
func TracingMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := observability.Tracer()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(
				r.Context(),
				// span нэр = "<METHOD> <path>" (service.name нь tracer
				// provider-ийн resource attribute-д тусдаа тогтоогддог).
				r.Method+" "+r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPath(r.URL.Path),
					semconv.ServerAddress(r.Host),
				),
			)
			defer span.End()

			// Статус кодыг span attribute-д тусгахын тулд хариуны бичигчийг
			// ороодог.
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// span-тай context-г доош түгээ — ингэснээр DB / Redis span-ууд
			// түүн доор үүрлэх бөгөөд RequestIDMiddleware нь trace_id-г
			// гаргаж авч чадна.
			next.ServeHTTP(ww, r.WithContext(ctx))

			statusCode := ww.Status()
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			span.SetAttributes(semconv.HTTPResponseStatusCode(statusCode))
		})
	}
}

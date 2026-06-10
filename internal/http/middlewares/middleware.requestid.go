// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"template/pkg/logger"
)

const RequestIDHeader = "X-Request-ID"

// RequestIDMiddleware нь ирж буй X-Request-ID-г хүлээж авна (эсвэл
// байхгүй бол UUID үүсгэдэг), хариунд буцаан тусгаж, хүсэлтийн context руу
// хоёр корреляцийн ID-г гүүрлэдэг тул logger.*WithContext нь тэдгээрийг log
// мөр бүрд гаргадаг:
//
//   - request_id: гадаад клиентэд харагдах ID. Үйлчилгээнүүдийн хооронд
//     ч клиентэд эхнээс эцэс хүртэл ижил хэвээр үлддэг.
//   - traceId: OTel-ийн үүсгэсэн W3C trace ID. tracing backend
//     (Jaeger / Tempo / г.м.) дахь span-уудтай log-уудыг холбоход
//     ашиглагддаг.
//
// Үүнийг tracing middleware-ийн ДАРАА суулга — ингэснээр бид trace ID-г
// гаргаж авахаар хүрэх үед OTel span context аль хэдийн тогтоогдсон байна.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(RequestIDHeader)
			if requestID == "" {
				requestID = uuid.New().String()
			}

			w.Header().Set(RequestIDHeader, requestID)

			ctx := context.WithValue(r.Context(), logger.RequestIDKey, requestID)
			if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
				ctx = context.WithValue(ctx, logger.TraceIDKey, span.SpanContext().TraceID().String())
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

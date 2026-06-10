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

// maxRequestIDLen нь клиентийн өгсөн X-Request-ID-ийн зөвшөөрөгдөх дээд урт.
const maxRequestIDLen = 128

// validRequestID нь клиентийн өгсөн корреляцийн ID-г баталгаажуулна —
// log-flooding / log-injection (terminal escape, parser хуурах)-аас
// сэргийлж урт болон тэмдэгтийн багцыг хязгаарлана.
func validRequestID(s string) bool {
	if len(s) == 0 || len(s) > maxRequestIDLen {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

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
			if !validRequestID(requestID) {
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

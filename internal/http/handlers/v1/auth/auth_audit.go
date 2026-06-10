// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"net"
	"net/http"
	"strings"

	"template/pkg/audit"
	"template/pkg/logger"
)

// auditFromRequest нь audit Event-ийн HTTP-context хэсгийг (IP,
// user-agent, request_id, trace_id) бүтээдэг тул дуудах газрууд зөвхөн
// event-д хамаарах талбаруудыг бөглөхөд хангалттай. Корреляцийн ID-ууд
// нь audit бичлэгүүдийг бүтэцлэгдсэн аппликейшний log-ууд руу болон
// (trace_id-ээр дамжуулан) tracing backend дахь span-ууд руу буцаан
// холбох боломжийг олгоно.
//
// chi port-ийн тэмдэглэл: request-id middleware нь корреляцийн ID-г
// хүсэлтийн context дотор logger.RequestIDKey дор хадгалдаг; trace ID нь
// tracing middleware-ийн тогтоосон span-тай хүсэлтийн context-ээс
// татагддаг. Клиентийн IP-г reverse-proxy-ийн ард ажиллах үед
// X-Forwarded-For-ийн эхний хаягаас, эс бөгөөс r.RemoteAddr-ийн host
// хэсгээс авна.
func auditFromRequest(r *http.Request) audit.Event {
	ctx := r.Context()
	requestID := ""
	if v, ok := ctx.Value(logger.RequestIDKey).(string); ok {
		requestID = v
	}
	return audit.Event{
		IP:        clientIP(r),
		UserAgent: r.Header.Get("User-Agent"),
		RequestID: requestID,
		TraceID:   logger.GetTraceIDFromContext(ctx),
	}
}

// clientIP нь хүсэлтийн клиентийн IP-г тогтооно. Эхлээд X-Forwarded-For
// header-ийн эхний (хамгийн гаднах) хаягийг авч, байхгүй бол
// r.RemoteAddr-аас host хэсгийг салгана.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

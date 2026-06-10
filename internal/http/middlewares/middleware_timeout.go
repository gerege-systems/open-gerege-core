// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"context"
	"net/http"
	"time"
)

// DefaultRequestTimeout нь нэг хүсэлтийн боловсруулалтын дээд хугацаа.
// Удаан гацсан handler / query нь холболтыг хэт удаан эзлэхээс сэргийлэх
// хамгаалалт (secure_system_guide §5.3, OWASP API4 Unrestricted Resource
// Consumption). Mailer зэрэг урт ажил нь хүсэлтийн замаас гадуур async
// ажилладаг тул энэ хязгаар тэдгээрт нөлөөлөхгүй.
const DefaultRequestTimeout = 30 * time.Second

// TimeoutMiddleware нь хүсэлтийн context дээр deadline тогтооно. Уг
// deadline нь handler-аас usecase → repository руу дамжиж, эцэст нь
// GORM-ийн WithContext(ctx) query-д хүрдэг тул хугацаа хэтэрсэн query
// автоматаар цуцлагдана. Энэ нь tracing / request-id middleware-ийн
// дараа байрлах ёстой — ингэснээр deadline-тай context нь тэдгээрийн
// тавьсан утгуудыг (trace_id, request_id) хадгална.
func TimeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

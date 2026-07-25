// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// DefaultRequestTimeout нь нэг хүсэлтийн боловсруулалтын дээд хугацаа.
// Удаан гацсан handler / query нь холболтыг хэт удаан эзлэхээс сэргийлэх
// хамгаалалт (secure_system_guide §5.3, OWASP API4 Unrestricted Resource
// Consumption). Гадны үйлчилгээ рүү хийх дуудлагууд (жишээ нь GeregeCloud
// Verify) өөрийн client timeout-той тул энэ хязгаараас тусдаа хязгаарлагдана.
const DefaultRequestTimeout = 30 * time.Second

// AIRequestTimeout нь Gemini рүү дуудлага хийдэг endpoint-уудын дээд хугацаа.
// TTS/STT/орчуулга нь ердийн үед 10-20 секунд (урт текст дээр бүр удаан)
// зарцуулдаг тул 30с-ийн ерөнхий хязгаарт мөргөж 500 өгч байв. 50с нь урд
// талын nginx-ийн proxy_read_timeout (60с)-аас доогуур зайтай — тиймээс клиент
// цэвэр хариу (эсвэл 503) авна, тасарсан холболт биш.
const AIRequestTimeout = 50 * time.Second

// slowPathPrefixes нь ерөнхий хязгаараас урт хугацаа шаардах замууд.
// Ерөнхий хамгаалалтыг сулруулахгүйн тулд ЗӨВХӨН эдгээр зам онцгойлогдоно.
var slowPathPrefixes = map[string]time.Duration{
	"/api/v1/ai/": AIRequestTimeout,
}

// timeoutFor нь тухайн замд тохирох deadline-ыг сонгоно.
func timeoutFor(path string, base time.Duration) time.Duration {
	for prefix, d := range slowPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return d
		}
	}
	return base
}

// TimeoutMiddleware нь хүсэлтийн context дээр deadline тогтооно. Уг
// deadline нь handler-аас usecase → repository руу дамжиж, эцэст нь
// GORM-ийн WithContext(ctx) query-д хүрдэг тул хугацаа хэтэрсэн query
// автоматаар цуцлагдана. Энэ нь tracing / request-id middleware-ийн
// дараа байрлах ёстой — ингэснээр deadline-тай context нь тэдгээрийн
// тавьсан утгуудыг (trace_id, request_id) хадгална.
func TimeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeoutFor(r.URL.Path, d))
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

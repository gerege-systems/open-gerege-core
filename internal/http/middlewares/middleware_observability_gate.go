// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// ObservabilityGate нь /metrics ба /swagger/doc.json гэх мэт операторын
// endpoint-уудыг хамгаалах нимгэн middleware юм. Эдгээр endpoint нь дотоод
// үйл ажиллагааны мэдрэмжтэй мэдээллийг (DB pool статистик, хүсэлтийн эзлэхүүн,
// route нэрс, алдааны түвшин) болон бүх API гадаргуугийн тодорхойлолтыг ил
// гаргадаг тул нийтэд задгай байх нь reconnaissance-д тусалдаг.
//
// Стратеги:
//   - production биш үед: үргэлж зөвшөөрнө (dev UX-ийг хадгална).
//   - production-д token хоосон үед: 404 буцаана. endpoint бүхэлдээ байхгүй мэт
//     харагдах нь reconnaissance-ыг хүндрүүлнэ.
//   - production-д token тохируулсан үед: "Authorization: Bearer <token>" яг
//     тааравал зөвшөөрнө; өөр бол 404 (401 биш — token шаардлагатай гэдгийг,
//     улмаар endpoint оршин байгааг ил гаргахгүй).
//
// Token харьцуулалт нь crypto/subtle.ConstantTimeCompare ашиглан timing
// oracle-ыг хаана. Bearer prefix-ийг case-insensitive нөхдөг.
func ObservabilityGate(isProduction bool, token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isProduction {
				next.ServeHTTP(w, r)
				return
			}
			if token == "" {
				http.NotFound(w, r)
				return
			}
			header := r.Header.Get("Authorization")
			const prefix = "bearer "
			if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
				http.NotFound(w, r)
				return
			}
			provided := strings.TrimSpace(header[len(prefix):])
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

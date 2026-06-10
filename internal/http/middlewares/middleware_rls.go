// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"

	"template/internal/datasources/rls"
)

// ServiceRLSContext нь хүсэлтийн context-г RLS-ийн "service" үүргээр тэмдэглэнэ.
// Нэвтрэхээс өмнөх auth урсгалууд (register / login / OTP / нууц үг сэргээх) нь
// хараахан баталгаажаагүй хэрэглэгчийн мөрд хандах шаардлагатай тул энэ
// middleware-г тухайн route бүлэгт суулгана.
//
// AuthMiddleware суусан route дээр (жишээ /auth/password/change) түүний тогтоосон
// user/admin identity нь дараа нь ажиллаж энэ "service"-г дарж бичдэг тул
// баталгаажсан үйлдлүүд хатуу хэвээр (least-privilege) үлддэг.
func ServiceRLSContext() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(rls.WithService(r.Context())))
		})
	}
}

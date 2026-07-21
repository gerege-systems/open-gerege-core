// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"

	"template/internal/datasources/rls"
	httpauth "template/internal/http/auth"
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

// OfficerRLSContext нь хүсэлтийн RLS үүргийг "officer" болгож ӨРГӨТГӨНӨ — иргэний
// хүсэлт хянадаг менежер бусад хүний мөрийг харах шаардлагатай (migration 44).
//
// ЗӨВХӨН officer route-уудад (RequirePermission(gov.review)-ийн ДАРАА) суулгана.
// Глобалаар суулгаж БОЛОХГҮЙ: 'officer' нь users зэрэг хүснэгтэд бодлогогүй тул
// менежер өөрийн профайлаа ч харахаа болино.
//
// Admin-ыг ХӨНДӨХГҮЙ — RoleAdmin аль хэдийн бүх gov бодлогод багтдаг тул
// доош нь "officer" болгох нь эрхийг нь хумих байсан.
func OfficerRLSContext() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := httpauth.CurrentUserFromContext(r)
			if err != nil || user.IsAdmin {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(rls.WithOfficer(r.Context(), user.ID)))
		})
	}
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net/http"

	"template/internal/config"
	"template/internal/constants"
)

// SecurityHeadersMiddleware нь хариу болгон дээр жижиг боловч өндөр
// үр нөлөөтэй багц browser-талын аюулгүй байдлын header-уудыг тогтооно.
// API өөрөө HTML render хийдэггүй боловч browser-ээс ирэх credential-тэй
// XHR / fetch дуудалтууд ашиг хүртдэг бөгөөд эдгээр header нь header
// дутуу байх нь жинхэнэ эмзэг байдал болох HTML үйлчилдэг (admin самбар,
// и-мэйл урьдчилан үзэх г.м.) ирээдүйн endpoint-уудын эсрэг хямд даатгал юм.
//
//	X-Content-Type-Options: nosniff             — MIME sniffing-г идэвхгүй болгоно
//	X-Frame-Options:        DENY                — <iframe>-ээр clickjacking-г хаана
//	Referrer-Policy:        strict-origin-...   — Referer-д алдагдах өгөгдлийг хязгаарлана
//	Content-Security-Policy: default-src 'none' — API-ууд JSON буцаадаг, юу ч ачаалах хэрэггүй
//	Strict-Transport-Security                    — зөвхөн production, бодит HTTPS шаардана
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	isProduction := config.AppConfig.Environment == constants.EnvironmentProduction
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// API хариунууд JSON; тэд хэзээ ч хууль ёсоор script, style,
			// frame, эсвэл зураг ачаалдаггүй. default-src 'none' нь
			// санамсаргүй ямар ч HTML хариуг browser-т идэвхгүй болгоно.
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			// Permissions-Policy: өгөгдмөлөөр бүгдийг татгалзана. Хямд бөгөөд
			// API гадаргуу хэзээ ч хэрэглэх ёсгүй хүчирхэг API-уудыг (camera,
			// geolocation, г.м.) хамардаг.
			h.Set("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")
			// Cross-origin isolation header-ууд (secure_system_guide §4.6/4.7).
			// COOP нь browsing-context-ийн opener-ийг тусгаарлаж Spectre маягийн
			// side-channel-аас хамгаална; CORP нь хариуг өөр site-аас no-cors-оор
			// embed хийхээс хаана (CORS fetch-д нөлөөлөхгүй тул frontend гэмтэхгүй);
			// COEP нь document-д хамаарах тул JSON хариунд бараг идэвхгүй ч
			// гарын авлагын багцыг бүрэн биелүүлэхээр тогтоов.
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-site")
			h.Set("Cross-Origin-Embedder-Policy", "require-corp")
			if isProduction {
				// HSTS зөвхөн production-д — http://localhost дээрх dev
				// сервер-ээс илгээх нь browser-т тухайн host-д энгийн HTTP-г
				// нэг жилийн турш татгалзахыг заадаг бөгөөд энэ нь өөртөө
				// буудах юм.
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

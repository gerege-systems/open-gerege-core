// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"github.com/go-chi/chi/v5"

	onboarding "template/internal/business/usecases/superadmin_onboarding"
	v1 "template/internal/http/handlers/v1"
	superadminonboardhandler "template/internal/http/handlers/v1/superadminonboard"
	"template/internal/http/middlewares"
)

// superadminOnboardRoute нь /v1/auth/superadmin/* бүлгийг холбоно: урилгаар
// хаалттай super admin бүртгэлийн шидтэн (Google → eID → и-мэйл OTP → TOTP)
// болон MFA-тай super admin нэвтрэлтийн 2 дахь шат.
//
// Бүх route НЭВТРЭЭГҮЙ (нэвтрэхээс өмнөх гадаргуу) тул authMiddleware АВАХГҮЙ —
// оронд нь:
//   - authRateLimiter (IP тус бүрт ~5/мин) — brute-force / нэвтрэлт оролдлого;
//   - AuthBodyMaxBytes (4 KiB) — auth-ийн жижиг JSON payload-д хангалттай;
//   - ServiceRLSContext — нэвтрээгүй хэрэглэгчийн мөрд хандах (invite хайлт,
//     хэрэглэгч upsert, нөөц код) шаардлагатай "service" RLS identity.
//
// Бодит хаалт нь: урилгын allow-list (Google алхам), шидтэний onboard_token
// (бусад алхам) болон mfa_token + TOTP/нөөц код (/mfa) дээр тогтоно.
type superadminOnboardRoute struct {
	handler     superadminonboardhandler.Handler
	router      chi.Router
	rateLimiter *middlewares.RateLimiter
	// pollRateLimiter нь eID poll-д зориулсан тусдаа СУЛ хязгаарлагч —
	// long-poll (≤25с) нь чанга 5/мин хязгаарт орвол байнга 429 болно.
	pollRateLimiter *middlewares.RateLimiter
}

// NewSuperAdminOnboardRoute нь route модулийг бүтээнэ. Rate limiter-уудыг
// дуудагч эзэмшдэг (graceful shutdown үед Stop хийнэ) — /auth бүлэгтэй
// хуваалцана.
func NewSuperAdminOnboardRoute(router chi.Router, onboardUC onboarding.Usecase, rateLimiter, pollRateLimiter *middlewares.RateLimiter) *superadminOnboardRoute {
	return &superadminOnboardRoute{
		handler:         superadminonboardhandler.NewHandler(onboardUC),
		router:          router,
		rateLimiter:     rateLimiter,
		pollRateLimiter: pollRateLimiter,
	}
}

func (rt *superadminOnboardRoute) Routes() {
	rt.router.Route("/v1/auth/superadmin", func(r chi.Router) {
		r.Use(middlewares.BodySizeLimitMiddleware(middlewares.AuthBodyMaxBytes))
		r.Use(middlewares.ServiceRLSContext())

		// Чанга хязгаарлагчтай дэд бүлэг — бүртгэлийн алхмууд ба MFA.
		r.Group(func(rl chi.Router) {
			rl.Use(rt.rateLimiter.Middleware())

			// Нэвтрэлтийн 2 дахь шат (давтагдах нэвтрэлт).
			rl.Post("/mfa", v1.Wrap(rt.handler.MFA))

			// Бүртгэлийн шидтэн.
			rl.Post("/onboard/google", v1.Wrap(rt.handler.Google))
			rl.Post("/onboard/eid/start", v1.Wrap(rt.handler.EIDStart))
			rl.Post("/onboard/eid/start-id", v1.Wrap(rt.handler.EIDStartByNationalID))
			rl.Post("/onboard/email/send", v1.Wrap(rt.handler.EmailSend))
			rl.Post("/onboard/email/verify", v1.Wrap(rt.handler.EmailVerify))
			rl.Post("/onboard/totp/init", v1.Wrap(rt.handler.TOTPInit))
			rl.Post("/onboard/totp/verify", v1.Wrap(rt.handler.TOTPVerify))
		})

		// /onboard/eid/poll — frontend ~2.5с тутам long-poll хийдэг тул чанга
		// 5/мин хязгаарт орвол COMPLETE хэзээ ч гарахгүй (/auth/eid/poll-ийн
		// адил тусдаа сул limiter).
		r.Group(func(pl chi.Router) {
			pl.Use(rt.pollRateLimiter.Middleware())
			pl.Post("/onboard/eid/poll", v1.Wrap(rt.handler.EIDPoll))
		})
	})
}

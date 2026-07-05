// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"template/internal/business/usecases/audit"
	"template/internal/business/usecases/auth"
	v1 "template/internal/http/handlers/v1"
	authhandler "template/internal/http/handlers/v1/auth"
	"template/internal/http/middlewares"
)

// authRoute нь /auth/* бүлгийг холбоно. "Login with eID" нь цорын ганц
// нэвтрэх арга тул нууц үг/OTP/бүртгэлийн route-ууд хасагдсан; зөвхөн
// eID нэвтрэлт (/eid/start, /eid/poll) болон session-ийн амьдралын мөчлөг
// (/refresh, /logout) үлдсэн. Бүгд rate limiter + чанга body хязгаар авдаг.
type authRoute struct {
	handler         authhandler.Handler
	router          chi.Router
	rateLimiter     *middlewares.RateLimiter
	pollRateLimiter *middlewares.RateLimiter
	authMiddleware  func(http.Handler) http.Handler
}

// NewAuthRoute нь route модулийг бүтээдэг. Rate limiter-уудыг дуудагч
// эзэмшдэг тул тэдгээрийн cleanup goroutine-г graceful shutdown үед Stop()
// хийж болно; auth middleware нь users route-той хуваалцагддаг. pollRateLimiter
// нь /eid/poll-д зориулсан тусдаа сул хязгаарлагч (long-poll-ийг 429-дэхгүй).
func NewAuthRoute(router chi.Router, authUC auth.Usecase, auditUC audit.Usecase, authMiddleware func(http.Handler) http.Handler, rateLimiter, pollRateLimiter *middlewares.RateLimiter) *authRoute {
	return &authRoute{
		handler:         authhandler.NewHandlerWithAudit(authUC, auditUC),
		router:          router,
		rateLimiter:     rateLimiter,
		pollRateLimiter: pollRateLimiter,
		authMiddleware:  authMiddleware,
	}
}

// Routes нь /v1/auth бүлэг болон түүний endpoint-уудыг суулгана. Бүлгийг хоёр
// дэд бүлэгт хуваана: rate limiter-тэй (нэвтрэлт эхлүүлэх / session lifecycle)
// ба rate limiter-гүй (poll). Хоёулаа body хязгаар + ServiceRLSContext авна.
func (rt *authRoute) Routes() {
	rt.router.Route("/v1/auth", func(r chi.Router) {
		// Auth payload-ууд жижиг JSON хэсгүүд — 4 KiB-д хязгаарлах нь
		// хэт том payload-ийн дайралтыг хаадаг. RLS: нэвтрэхээс өмнөх
		// урсгалууд (eID upsert SELECT/INSERT, refresh дэх identity хайлт)
		// баталгаажаагүй хэрэглэгчийн мөрд хандах тул "service" identity
		// тавина. Энэ хоёр middleware бүлгийн БҮХ endpoint-д үйлчилнэ.
		r.Use(middlewares.BodySizeLimitMiddleware(middlewares.AuthBodyMaxBytes))
		r.Use(middlewares.ServiceRLSContext())

		// Rate limiter-тэй дэд бүлэг — IP тус бүрт минутанд ~5 хүсэлт. Нэвтрэлт
		// эхлүүлэх (start/start-id) ба session lifecycle (refresh/logout) нь
		// ховор дуудагддаг тул чанга хязгаар тохирно.
		r.Group(func(rl chi.Router) {
			rl.Use(rt.rateLimiter.Middleware())
			// eID нэвтрэлт эхлүүлэх. /eid/start QR/deep-link эхлүүлнэ.
			rl.Post("/eid/start", v1.Wrap(rt.handler.EIDStart))
			// /eid/start-id — иргэний РД-аар нэвтрэлт эхлүүлж, бүртгэлтэй
			// төхөөрөмж рүү push хийлгэнэ (gerege.mn-ийн "РД оруулах → push").
			rl.Post("/eid/start-id", v1.Wrap(rt.handler.EIDStartByNationalID))
			// Google OAuth callback — code exchange + eID холболт/шууд нэвтрэлт.
			rl.Post("/google", v1.Wrap(rt.handler.GoogleLogin))
			// Session-ийн амьдралын мөчлөг — нэвтрэх аргаас үл хамаарна.
			rl.Post("/refresh", v1.Wrap(rt.handler.Refresh))
			rl.Post("/logout", v1.Wrap(rt.handler.Logout))
		})

		// Нэвтэрсэн хэрэглэгч Google холболтоо САЛГАХ (integrations/dashboard
		// картаас). Холбох нь зөвхөн login урсгалаар хийгддэг. authMiddleware.
		r.Group(func(pr chi.Router) {
			pr.Use(rt.authMiddleware)
			pr.Delete("/google/link", v1.Wrap(rt.handler.GoogleUnlink))
		})

		// /eid/poll — frontend нь session-ийг ~2.5с тутамд long-poll-оор
		// асуудаг тул /auth-ийн чанга 5/мин хязгаарт орвол байнга 429 болж
		// амжилттай COMPLETE хэзээ ч гарахгүй. Иймд тусдаа СУЛ limiter
		// (~60/мин, burst 30): хууль ёсны poll-д хангалттай зайтай ч нэг
		// IP-гээс хязгааргүй concurrent 25с long-poll эхлүүлэх slow-DoS-д тааз
		// тавина (body хязгаар + ServiceRLSContext бүлгийн түвшинд хэвээр).
		r.Group(func(pl chi.Router) {
			pl.Use(rt.pollRateLimiter.Middleware())
			pl.Post("/eid/poll", v1.Wrap(rt.handler.EIDPoll))
		})
	})
}

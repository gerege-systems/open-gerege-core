// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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
	handler        authhandler.Handler
	router         chi.Router
	rateLimiter    *middlewares.RateLimiter
	authMiddleware func(http.Handler) http.Handler
}

// NewAuthRoute нь route модулийг бүтээдэг. Rate limiter-г дуудагч
// эзэмшдэг тул түүний cleanup goroutine-г graceful shutdown үед Stop()
// хийж болно; auth middleware нь users route-той хуваалцагддаг.
func NewAuthRoute(router chi.Router, authUC auth.Usecase, authMiddleware func(http.Handler) http.Handler, rateLimiter *middlewares.RateLimiter) *authRoute {
	return &authRoute{
		handler:        authhandler.NewHandler(authUC),
		router:         router,
		rateLimiter:    rateLimiter,
		authMiddleware: authMiddleware,
	}
}

// Routes нь /v1/auth бүлэг болон түүний endpoint-уудыг суулгана.
func (rt *authRoute) Routes() {
	rt.router.Route("/v1/auth", func(r chi.Router) {
		// Auth payload-ууд жижиг JSON хэсгүүд — 4 KiB-д хязгаарлах нь
		// нэрээ нууцалсан урсгал хүлээн авдаг цорын ганц route-уудын
		// эсрэг хэт том payload-ийн дайралтыг хаадаг. Rate limiter нь
		// IP тус бүрт минутанд 5 хүсэлт зөвшөөрнө.
		r.Use(rt.rateLimiter.Middleware())
		r.Use(middlewares.BodySizeLimitMiddleware(middlewares.AuthBodyMaxBytes))
		// RLS: нэвтрэхээс өмнөх урсгалууд (eID upsert SELECT/INSERT, refresh
		// дэх email/identity хайлт) баталгаажаагүй хэрэглэгчийн мөрд хандах тул
		// "service" identity тавина.
		r.Use(middlewares.ServiceRLSContext())

		// eID нэвтрэлт — цорын ганц нэвтрэх арга. /eid/start QR/deep-link
		// эхлүүлж, /eid/poll session-ийг long-poll-оор хүлээж токен олгоно.
		r.Post("/eid/start", v1.Wrap(rt.handler.EIDStart))
		// /eid/start-id — иргэний РД-аар нэвтрэлт эхлүүлж, бүртгэлтэй
		// төхөөрөмж рүү push хийлгэнэ (gerege.mn-ийн "РД оруулах → push").
		r.Post("/eid/start-id", v1.Wrap(rt.handler.EIDStartByNationalID))
		r.Post("/eid/poll", v1.Wrap(rt.handler.EIDPoll))
		// Session-ийн амьдралын мөчлөг — нэвтрэх аргаас үл хамаарна.
		r.Post("/refresh", v1.Wrap(rt.handler.Refresh))
		r.Post("/logout", v1.Wrap(rt.handler.Logout))
	})
}

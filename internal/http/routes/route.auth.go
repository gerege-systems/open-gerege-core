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

// authRoute нь /auth/* бүлгийг холбоно — register / login / OTP /
// refresh / logout / forgot / reset. Нэрээ нууцалсан endpoint-ууд нь
// rate limiter болон чанга body хязгаар авдаг; /password/change нь
// нэмэлтээр JWT шаарддаг.
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

		r.Post("/register", v1.Wrap(rt.handler.Register))
		r.Post("/login", v1.Wrap(rt.handler.Login))
		r.Post("/send-otp", v1.Wrap(rt.handler.SendOTP))
		r.Post("/verify-otp", v1.Wrap(rt.handler.VerifyOTP))
		r.Post("/refresh", v1.Wrap(rt.handler.Refresh))
		r.Post("/logout", v1.Wrap(rt.handler.Logout))
		r.Post("/password/forgot", v1.Wrap(rt.handler.ForgotPassword))
		r.Post("/password/reset", v1.Wrap(rt.handler.ResetPassword))
		// /password/change нь JWT шаарддаг — бусадтай ижил rate limiter /
		// body хязгаарын дээр auth middleware авдаг.
		r.With(rt.authMiddleware).Put("/password/change", v1.Wrap(rt.handler.ChangePassword))
	})
}

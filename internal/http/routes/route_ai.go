// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	aiuc "template/internal/business/usecases/ai"
	v1 "template/internal/http/handlers/v1"
	aihandler "template/internal/http/handlers/v1/ai"
	"template/internal/http/middlewares"
)

// aiRoute нь /ai/* бүлгийг холбоно. Gemini дуудлага үнэтэй тул нэвтэрсэн
// хэрэглэгч шаардахаас гадна тусдаа (auth-аас чангавтар) rate limiter авдаг.
type aiRoute struct {
	handler        aihandler.Handler
	router         chi.Router
	rateLimiter    *middlewares.RateLimiter
	authMiddleware func(http.Handler) http.Handler
}

// NewAIRoute нь route модулийг бүтээдэг. Rate limiter-г дуудагч эзэмшдэг
// тул graceful shutdown үед Stop() хийгддэг (auth limiter-тэй ижил загвар).
func NewAIRoute(router chi.Router, aiUC aiuc.Usecase, authMiddleware func(http.Handler) http.Handler, rateLimiter *middlewares.RateLimiter) *aiRoute {
	return &aiRoute{
		handler:        aihandler.NewHandler(aiUC),
		router:         router,
		rateLimiter:    rateLimiter,
		authMiddleware: authMiddleware,
	}
}

// Routes нь /v1/ai бүлэг болон түүний endpoint-уудыг суулгана.
func (rt *aiRoute) Routes() {
	rt.router.Route("/v1/ai", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Use(rt.rateLimiter.Middleware())
		// Audio (base64 ~700 KB) + текст payload нь глобал 1 MiB хязгаарт
		// багтана — энд тусдаа чангалалт хэрэггүй (глобал давхарга барина).

		r.Post("/chat", v1.Wrap(rt.handler.Chat))
		r.Post("/stt", v1.Wrap(rt.handler.Transcribe))
		r.Post("/tts", v1.Wrap(rt.handler.Speak))
		r.Post("/translate", v1.Wrap(rt.handler.Translate))
	})
}

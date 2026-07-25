// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"github.com/go-chi/chi/v5"

	aiuc "template/internal/business/usecases/ai"
	v1 "template/internal/http/handlers/v1"
	aihandler "template/internal/http/handlers/v1/ai"
	"template/internal/http/middlewares"
)

// publicAIRoute нь НЭВТРЭЛТГҮЙ чат гадаргуу (/v1/public/ai/*) — нүүр
// хуудасны хөвөгч чат виджет үүнийг дуудна.
//
// Аюулгүй байдлын хил:
//   - auth байхгүй тул хамгаалалт нь rate limit + богино payload хязгаар
//     (requests.AIPublicChatRequest) + нэмэлт system-prompt хориг (Anonymous),
//   - дуудагч нь тусдаа usecase instance дамжуулна: зөвхөн нийтэд аюулгүй
//     tool-той (мэдлэгийн сангийн хайлт). Хэрэглэгчийн өгөгдөл уншдаг
//     tool-ыг энэ багцад ХЭЗЭЭ Ч бүү нэм — нэргүй зочин дуудах боломжтой.
type publicAIRoute struct {
	handler     aihandler.Handler
	router      chi.Router
	rateLimiter *middlewares.RateLimiter
}

// NewPublicAIRoute нь нээлттэй чатын route модулийг бүтээнэ. aiUC нь нийтэд
// зориулсан (хязгаарлагдмал tool-той) usecase байх ёстой.
func NewPublicAIRoute(router chi.Router, aiUC aiuc.Usecase, rateLimiter *middlewares.RateLimiter) *publicAIRoute {
	return &publicAIRoute{
		handler:     aihandler.NewHandler(aiUC),
		router:      router,
		rateLimiter: rateLimiter,
	}
}

func (rt *publicAIRoute) Routes() {
	rt.router.Route("/v1/public/ai", func(r chi.Router) {
		r.Use(rt.rateLimiter.Middleware())

		r.Post("/chat", v1.Wrap(rt.handler.PublicChat))
		// Урсгалаар (SSE) — виджет үүнийг хэрэглэнэ: хариулт бичигдэж
		// байхад нь харагдана, дуут мессежийн хуулбар эхний event-ээр ирнэ.
		r.Post("/chat/stream", v1.Wrap(rt.handler.PublicChatStream))
		// «Сонсох» товч — хариултыг дуут болгоно (нэг дуудалт = нэг мессеж).
		r.Post("/tts", v1.Wrap(rt.handler.PublicSpeak))
	})
}

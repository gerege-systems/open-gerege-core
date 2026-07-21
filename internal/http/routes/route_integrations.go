// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	integrationsuc "template/internal/business/usecases/integrations"
	v1 "template/internal/http/handlers/v1"
	integrationshandler "template/internal/http/handlers/v1/integrations"

	"github.com/go-chi/chi/v5"
)

// integrationsRoute нь /v1/integrations/* бүлгийг холбоно — хэрэглэгчийн
// гуравдагч этгээдийн OAuth холболтыг удирдах endpoint-ууд. Бүгд auth-шаардлагатай.
type integrationsRoute struct {
	handler        integrationshandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewIntegrationsRoute(router chi.Router, uc integrationsuc.Usecase, authMiddleware func(http.Handler) http.Handler) *integrationsRoute {
	return &integrationsRoute{
		handler:        integrationshandler.NewHandler(uc),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *integrationsRoute) Routes() {
	rt.router.Route("/v1/integrations", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Get("/", v1.Wrap(rt.handler.List))
		r.Post("/", v1.Wrap(rt.handler.Connect))
		r.Get("/{provider}/token", v1.Wrap(rt.handler.GetToken))
		r.Delete("/{provider}", v1.Wrap(rt.handler.Disconnect))
	})
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	applicationsuc "template/internal/business/usecases/applications"
	rbacuc "template/internal/business/usecases/rbac"
	v1 "template/internal/http/handlers/v1"
	applicationshandler "template/internal/http/handlers/v1/applications"
	"template/internal/http/middlewares"
)

// applicationsRoute нь /applications/* бүлгийг холбоно. Бүх endpoint нь
// 'gateway.manage' эрх шаардана (admin автоматаар давна). Зөвхөн Hydra
// тохируулагдсан үед л server.go энэ route-ыг холбоно.
type applicationsRoute struct {
	handler        applicationshandler.Handler
	resolver       rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewApplicationsRoute(router chi.Router, appsUC applicationsuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *applicationsRoute {
	return &applicationsRoute{
		handler:        applicationshandler.NewHandler(appsUC),
		resolver:       rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *applicationsRoute) Routes() {
	manage := middlewares.RequirePermission(rt.resolver, domain.PermGatewayManage)
	rt.router.Route("/v1/applications", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Use(manage)

		r.Get("/", v1.Wrap(rt.handler.List))
		r.Post("/", v1.Wrap(rt.handler.Create))
		r.Get("/{id}", v1.Wrap(rt.handler.Get))
		r.Put("/{id}", v1.Wrap(rt.handler.Update))
		r.Delete("/{id}", v1.Wrap(rt.handler.Delete))
		r.Post("/{id}/rotate-secret", v1.Wrap(rt.handler.RotateSecret))
		r.Put("/{id}/secret", v1.Wrap(rt.handler.SetSecret))
		r.Put("/{id}/services", v1.Wrap(rt.handler.SetServices))
	})
}

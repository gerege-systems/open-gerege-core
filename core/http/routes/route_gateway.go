// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/platform-core/core/business/domain"
	gatewayuc "github.com/gerege-systems/platform-core/core/business/usecases/gateway"
	rbacuc "github.com/gerege-systems/platform-core/core/business/usecases/rbac"
	v1 "github.com/gerege-systems/platform-core/core/http/handlers/v1"
	gatewayhandler "github.com/gerege-systems/platform-core/core/http/handlers/v1/gateway"
	"github.com/gerege-systems/platform-core/core/http/middlewares"
)

// gatewayRoute нь /gateway/* бүлгийг холбоно. Бүх endpoint нь 'gateway.manage'
// эрх шаардана (admin автоматаар давна). rbac usecase нь эрх шалгагч (resolver).
type gatewayRoute struct {
	handler        gatewayhandler.Handler
	resolver       rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewGatewayRoute(router chi.Router, gatewayUC gatewayuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *gatewayRoute {
	return &gatewayRoute{
		handler:        gatewayhandler.NewHandler(gatewayUC),
		resolver:       rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *gatewayRoute) Routes() {
	manage := middlewares.RequirePermission(rt.resolver, domain.PermGatewayManage)
	rt.router.Route("/v1/gateway", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Use(manage)

		// Telemetry
		r.Get("/overview", v1.Wrap(rt.handler.Overview))
		r.Get("/logs", v1.Wrap(rt.handler.ListLogs))

		// Services
		r.Get("/services", v1.Wrap(rt.handler.ListServices))
		r.Post("/services", v1.Wrap(rt.handler.CreateService))
		r.Put("/services/{id}", v1.Wrap(rt.handler.UpdateService))
		r.Delete("/services/{id}", v1.Wrap(rt.handler.DeleteService))
	})
}

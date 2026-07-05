// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	gatewayuc "template/internal/business/usecases/gateway"
	rbacuc "template/internal/business/usecases/rbac"
	v1 "template/internal/http/handlers/v1"
	gatewayhandler "template/internal/http/handlers/v1/gateway"
	"template/internal/http/middlewares"
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

		// Routes
		r.Get("/routes", v1.Wrap(rt.handler.ListRoutes))
		r.Post("/routes", v1.Wrap(rt.handler.CreateRoute))
		r.Put("/routes/{id}", v1.Wrap(rt.handler.UpdateRoute))
		r.Delete("/routes/{id}", v1.Wrap(rt.handler.DeleteRoute))

		// Consumers + keys
		r.Get("/consumers", v1.Wrap(rt.handler.ListConsumers))
		r.Post("/consumers", v1.Wrap(rt.handler.CreateConsumer))
		r.Put("/consumers/{id}", v1.Wrap(rt.handler.UpdateConsumer))
		r.Delete("/consumers/{id}", v1.Wrap(rt.handler.DeleteConsumer))
		r.Get("/consumers/{id}/keys", v1.Wrap(rt.handler.ListKeys))
		r.Post("/consumers/{id}/keys", v1.Wrap(rt.handler.CreateKey))
		r.Post("/keys/{keyId}/revoke", v1.Wrap(rt.handler.RevokeKey))
		r.Delete("/keys/{keyId}", v1.Wrap(rt.handler.DeleteKey))

		// Policies
		r.Get("/policies", v1.Wrap(rt.handler.ListPolicies))
		r.Post("/policies", v1.Wrap(rt.handler.CreatePolicy))
		r.Put("/policies/{id}", v1.Wrap(rt.handler.UpdatePolicy))
		r.Delete("/policies/{id}", v1.Wrap(rt.handler.DeletePolicy))
	})
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	rbacuc "template/internal/business/usecases/rbac"
	"template/internal/business/usecases/users"
	v1 "template/internal/http/handlers/v1"
	adminhandler "template/internal/http/handlers/v1/admin"
	"template/internal/http/middlewares"
)

// adminRoute нь /admin/* удирдлагын бүлгийг холбоно. Бүгд auth + 'users.manage'
// эрх шаардана (admin автоматаар давна; manager-д энэ эрх олгогдсон).
type adminRoute struct {
	handler        adminhandler.Handler
	rbacUC         rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewAdminRoute(router chi.Router, usersUC users.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *adminRoute {
	return &adminRoute{
		handler:        adminhandler.NewHandler(usersUC),
		rbacUC:         rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *adminRoute) Routes() {
	rt.router.Route("/v1/admin", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		manage := middlewares.RequirePermission(rt.rbacUC, domain.PermUsersManage)
		r.With(manage).Get("/users", v1.Wrap(rt.handler.ListUsers))
		r.With(manage).Put("/users/{id}/role", v1.Wrap(rt.handler.UpdateUserRole))
		r.With(manage).Put("/users/{id}/active", v1.Wrap(rt.handler.SetUserActive))
		r.With(manage).Delete("/users/{id}", v1.Wrap(rt.handler.DeleteUser))
	})
}

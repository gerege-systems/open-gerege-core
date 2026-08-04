// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	adminhandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/admin"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
)

// adminRoute нь /admin/users* хэрэглэгчийн удирдлагын бүлгийг холбоно
// ('users.manage' эрх; admin автоматаар давна).
//
// /admin/ai/* нь ЭНД БИШ: манифестийн эзэмшил (хамгийн урт угтвар) түүнийг
// ai модульд өгдөг тул route_admin_ai.go-д тусад нь бүртгэгдэнэ. Ингэснээр
// core модуль business модулиас хамаарахаа болив.
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
		r.With(manage).Post("/users", v1.Wrap(rt.handler.CreateUser))
		r.With(manage).Put("/users/{id}/role", v1.Wrap(rt.handler.UpdateUserRole))
		r.With(manage).Put("/users/{id}/active", v1.Wrap(rt.handler.SetUserActive))
		r.With(manage).Delete("/users/{id}", v1.Wrap(rt.handler.DeleteUser))
	})
}

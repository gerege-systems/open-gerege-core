// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	aiuc "template/internal/business/usecases/ai"
	rbacuc "template/internal/business/usecases/rbac"
	"template/internal/business/usecases/users"
	v1 "template/internal/http/handlers/v1"
	adminhandler "template/internal/http/handlers/v1/admin"
	aihandler "template/internal/http/handlers/v1/ai"
	"template/internal/http/middlewares"
)

// adminRoute нь /admin/* удирдлагын бүлгийг холбоно. Хэрэглэгчийн удирдлага
// 'users.manage', AI prompt тохиргоо 'settings.manage' эрх шаардана (admin
// автоматаар давна).
type adminRoute struct {
	handler        adminhandler.Handler
	aiHandler      aihandler.Handler
	rbacUC         rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewAdminRoute(router chi.Router, usersUC users.Usecase, rbacUC rbacuc.Usecase, aiUC aiuc.Usecase, authMiddleware func(http.Handler) http.Handler) *adminRoute {
	return &adminRoute{
		handler:        adminhandler.NewHandler(usersUC),
		aiHandler:      aihandler.NewHandler(aiUC),
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

		// AI prompt давхаргын тохиргоо — системийн тохиргооны эрхээр.
		manageSettings := middlewares.RequirePermission(rt.rbacUC, domain.PermSettingsManage)
		r.With(manageSettings).Get("/ai/prompts", v1.Wrap(rt.aiHandler.ListPrompts))
		r.With(manageSettings).Put("/ai/prompts/{key}", v1.Wrap(rt.aiHandler.SetPrompt))
	})
}

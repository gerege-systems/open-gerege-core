// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	aiuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/ai"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	adminhandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/admin"
	aihandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/ai"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
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
		r.With(manage).Post("/users", v1.Wrap(rt.handler.CreateUser))
		r.With(manage).Put("/users/{id}/role", v1.Wrap(rt.handler.UpdateUserRole))
		r.With(manage).Put("/users/{id}/active", v1.Wrap(rt.handler.SetUserActive))
		r.With(manage).Delete("/users/{id}", v1.Wrap(rt.handler.DeleteUser))

		// AI prompt давхаргын тохиргоо — системийн тохиргооны эрхээр.
		manageSettings := middlewares.RequirePermission(rt.rbacUC, domain.PermSettingsManage)
		r.With(manageSettings).Get("/ai/prompts", v1.Wrap(rt.aiHandler.ListPrompts))
		r.With(manageSettings).Put("/ai/prompts/{key}", v1.Wrap(rt.aiHandler.SetPrompt))
		// Мэдлэгийн сангийн вектор индексийг гараар шинэчлэх (агуулга засварласны дараа).
		r.With(manageSettings).Post("/ai/knowledge/reindex", v1.Wrap(rt.aiHandler.ReindexKnowledge))
	})
}

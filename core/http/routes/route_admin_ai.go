// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	aiuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/ai"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	aihandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/ai"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
)

// adminAIRoute нь /admin/ai/* AI-ийн удирдлагын бүлгийг холбоно
// ('settings.manage' эрх). Манифестийн эзэмшлээр (/api/v1/admin/ai/ нь
// /api/v1/admin/-ээс УРТ угтвар тул ai ялна) энэ бүлэг ai модульд харьяалагдах
// бөгөөд ai модуль өөрөө бүртгэнэ.
//
// chi нь /v1/admin болон /v1/admin/ai хоёрыг зэрэгцээ Mount хийхийг зөвшөөрч,
// тодорхой (static) сегментийг эрхэмлэн чиглүүлдэг — тиймээс хоёр бүлэг
// зөрчилдөхгүй.
type adminAIRoute struct {
	handler        aihandler.Handler
	rbacUC         rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

// NewAdminAIRoute нь route модулийг бүтээдэг.
func NewAdminAIRoute(router chi.Router, aiUC aiuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *adminAIRoute {
	return &adminAIRoute{
		handler:        aihandler.NewHandler(aiUC),
		rbacUC:         rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *adminAIRoute) Routes() {
	rt.router.Route("/v1/admin/ai", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		manageSettings := middlewares.RequirePermission(rt.rbacUC, domain.PermSettingsManage)
		r.With(manageSettings).Get("/prompts", v1.Wrap(rt.handler.ListPrompts))
		r.With(manageSettings).Put("/prompts/{key}", v1.Wrap(rt.handler.SetPrompt))
		// Мэдлэгийн сангийн вектор индексийг гараар шинэчлэх (агуулга засварласны дараа).
		r.With(manageSettings).Post("/knowledge/reindex", v1.Wrap(rt.handler.ReindexKnowledge))
	})
}

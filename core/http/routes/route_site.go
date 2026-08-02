// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	siteuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/site"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	sitehandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/site"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
)

// siteRoute нь /site/* бүлгийг холбоно. GET /site/appearance нь нийтийн
// (landing уншина, auth-гүй); PUT /site/appearance нь админ ('settings.manage').
type siteRoute struct {
	handler        sitehandler.Handler
	rbacUC         rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewSiteRoute(router chi.Router, siteUC siteuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler, auth sitehandler.AuthSurface) *siteRoute {
	return &siteRoute{
		handler:        sitehandler.NewHandler(siteUC, auth),
		rbacUC:         rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *siteRoute) Routes() {
	rt.router.Route("/v1/site", func(r chi.Router) {
		// Нийтийн уншилт — landing/anon зочин (нэвтрэлт шаардахгүй).
		r.Get("/appearance", v1.Wrap(rt.handler.GetAppearance))
		// Нэвтрэх гадаргууны горим — landing болон /login үүгээр нэвтрэх карт
		// үзүүлэх үү, SSO руу шилжүүлэх үү гэдгээ шийднэ (мөн auth-гүй).
		r.Get("/auth", v1.Wrap(rt.handler.GetAuth))

		// Админ засвар — нэвтрэлт + settings.manage эрх.
		manageSettings := middlewares.RequirePermission(rt.rbacUC, domain.PermSettingsManage)
		r.With(rt.authMiddleware).With(manageSettings).Put("/appearance", v1.Wrap(rt.handler.SetAppearance))
	})
}

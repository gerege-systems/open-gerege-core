// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	rbacuc "template/internal/business/usecases/rbac"
	siteuc "template/internal/business/usecases/site"
	v1 "template/internal/http/handlers/v1"
	sitehandler "template/internal/http/handlers/v1/site"
	"template/internal/http/middlewares"
)

// siteRoute нь /site/* бүлгийг холбоно. GET /site/appearance нь нийтийн
// (landing уншина, auth-гүй); PUT /site/appearance нь админ ('settings.manage').
type siteRoute struct {
	handler        sitehandler.Handler
	rbacUC         rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewSiteRoute(router chi.Router, siteUC siteuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *siteRoute {
	return &siteRoute{
		handler:        sitehandler.NewHandler(siteUC),
		rbacUC:         rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *siteRoute) Routes() {
	rt.router.Route("/v1/site", func(r chi.Router) {
		// Нийтийн уншилт — landing/anon зочин (нэвтрэлт шаардахгүй).
		r.Get("/appearance", v1.Wrap(rt.handler.GetAppearance))

		// Админ засвар — нэвтрэлт + settings.manage эрх.
		manageSettings := middlewares.RequirePermission(rt.rbacUC, domain.PermSettingsManage)
		r.With(rt.authMiddleware).With(manageSettings).Put("/appearance", v1.Wrap(rt.handler.SetAppearance))
	})
}

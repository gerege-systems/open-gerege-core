// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	rbacuc "template/internal/business/usecases/rbac"
	themeuc "template/internal/business/usecases/theme"
	v1 "template/internal/http/handlers/v1"
	themehandler "template/internal/http/handlers/v1/theme"
	"template/internal/http/middlewares"
)

// themeRoute нь /themes/* бүлгийг холбоно. GET /themes/active нь нийтийн (landing
// уншина, auth-гүй); бусад CRUD/идэвхжүүлэлт нь админ ('settings.manage').
// Тусдаа /v1/themes prefix ашигласнаар /v1/admin бүлгийн mount-той мөргөлдөхгүй.
type themeRoute struct {
	handler        themehandler.Handler
	rbacUC         rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewThemeRoute(router chi.Router, themeUC themeuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *themeRoute {
	return &themeRoute{
		handler:        themehandler.NewHandler(themeUC),
		rbacUC:         rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *themeRoute) Routes() {
	rt.router.Route("/v1/themes", func(r chi.Router) {
		// Нийтийн — идэвхтэй theme-ийг landing уншина (auth-гүй).
		r.Get("/active", v1.Wrap(rt.handler.GetActive))

		// Админ CRUD/идэвхжүүлэлт — нэвтрэлт + settings.manage эрх.
		r.Group(func(ar chi.Router) {
			ar.Use(rt.authMiddleware)
			ar.Use(middlewares.RequirePermission(rt.rbacUC, domain.PermSettingsManage))
			ar.Get("/", v1.Wrap(rt.handler.List))
			ar.Post("/", v1.Wrap(rt.handler.Create))
			ar.Get("/{id}", v1.Wrap(rt.handler.Get))
			ar.Put("/{id}", v1.Wrap(rt.handler.Update))
			ar.Delete("/{id}", v1.Wrap(rt.handler.Delete))
			ar.Put("/{id}/active", v1.Wrap(rt.handler.SetActive))
		})
	})
}

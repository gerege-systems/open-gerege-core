// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	superadminuc "template/internal/business/usecases/superadmin"
	v1 "template/internal/http/handlers/v1"
	superadminhandler "template/internal/http/handlers/v1/superadmin"
	"template/internal/http/middlewares"
)

// superadminRoute нь /superadmin/* бүлгийг холбоно — админ хэрэглэгчдийг
// удирдах (жагсаах/үүсгэх/эрх олгох/хасах). Бүх route нь RequireSuperAdmin-ээр
// хамгаалагдсан тул зөвхөн super admin (RoleSuperAdmin) хандана; энгийн admin ч
// хүрэхгүй (least-privilege).
type superadminRoute struct {
	handler        superadminhandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewSuperAdminRoute(router chi.Router, superadminUC superadminuc.Usecase, authMiddleware func(http.Handler) http.Handler) *superadminRoute {
	return &superadminRoute{
		handler:        superadminhandler.NewHandler(superadminUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *superadminRoute) Routes() {
	rt.router.Route("/v1/superadmin", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Use(middlewares.RequireSuperAdmin())
		r.Get("/admins", v1.Wrap(rt.handler.ListAdmins))
		r.Post("/admins", v1.Wrap(rt.handler.CreateAdmin))
		r.Put("/admins/{id}/grant", v1.Wrap(rt.handler.GrantAdmin))
		r.Delete("/admins/{id}", v1.Wrap(rt.handler.RevokeAdmin))
	})
}

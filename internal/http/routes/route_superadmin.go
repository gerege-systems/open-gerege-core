// Government Template Platform V3.0
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
		r.Get("/admins/by-register", v1.Wrap(rt.handler.LookupByRegister))
		r.Post("/admins/by-register", v1.Wrap(rt.handler.AddAdminByRegister))
		r.Put("/admins/{id}/grant", v1.Wrap(rt.handler.GrantAdmin))
		r.Delete("/admins/{id}", v1.Wrap(rt.handler.RevokeAdmin))
		// Super admin урилга (allow-list) — урилга нь эрхийг ШУУД олгодоггүй,
		// зөвхөн /auth/superadmin/onboard шидтэнг эхлүүлэх хаалгыг нээнэ.
		r.Get("/invites", v1.Wrap(rt.handler.ListInvites))
		r.Post("/invites", v1.Wrap(rt.handler.CreateInvite))
		r.Delete("/invites/{email}", v1.Wrap(rt.handler.DeleteInvite))
		// Платформын хандалтын горим (public|private) — зөвхөн super admin.
		r.Get("/access-mode", v1.Wrap(rt.handler.GetAccessMode))
		r.Put("/access-mode", v1.Wrap(rt.handler.SetAccessMode))
	})
}

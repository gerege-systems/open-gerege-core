// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	audituc "template/internal/business/usecases/audit"
	rbacuc "template/internal/business/usecases/rbac"
	v1 "template/internal/http/handlers/v1"
	rbachandler "template/internal/http/handlers/v1/rbac"
	"template/internal/http/middlewares"
)

// rbacRoute нь /rbac/* бүлгийг холбоно. /rbac/me нь нэвтэрсэн хэрэглэгч бүрт
// нээлттэй (өөрийн эрхээ авах); бусад нь 'roles.manage' эрх шаардана.
type rbacRoute struct {
	handler        rbachandler.Handler
	usecase        rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewRBACRoute(router chi.Router, rbacUC rbacuc.Usecase, auditUC audituc.Usecase, authMiddleware func(http.Handler) http.Handler) *rbacRoute {
	return &rbacRoute{
		handler:        rbachandler.NewHandlerWithAudit(rbacUC, auditUC),
		usecase:        rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *rbacRoute) Routes() {
	rt.router.Route("/v1/rbac", func(r chi.Router) {
		r.Use(rt.authMiddleware)

		// Нэвтэрсэн хэрэглэгч бүр өөрийн эрхүүдээ авч болно (frontend цэс шүүхэд).
		r.Get("/me", v1.Wrap(rt.handler.MyPermissions))

		// Удирдлага — 'roles.manage' эрх шаардана (admin автоматаар давна).
		manage := middlewares.RequirePermission(rt.usecase, domain.PermRolesManage)
		r.With(manage).Get("/roles", v1.Wrap(rt.handler.ListRoles))
		r.With(manage).Get("/permissions", v1.Wrap(rt.handler.ListPermissions))
		r.With(manage).Post("/roles", v1.Wrap(rt.handler.CreateRole))
		r.With(manage).Put("/roles/{id}", v1.Wrap(rt.handler.UpdateRole))
		r.With(manage).Put("/roles/{id}/permissions", v1.Wrap(rt.handler.SetRolePermissions))
		r.With(manage).Delete("/roles/{id}", v1.Wrap(rt.handler.DeleteRole))
	})
}

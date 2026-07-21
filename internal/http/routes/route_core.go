// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	coreuc "template/internal/business/usecases/core"
	rbacuc "template/internal/business/usecases/rbac"
	v1 "template/internal/http/handlers/v1"
	corehandler "template/internal/http/handlers/v1/core"
	"template/internal/http/middlewares"
)

// coreRoute нь Gerege Core (core.gerege.mn)-ийн хайлтын /core/* бүлгийг холбоно.
// Энэ нь privileged service token-оор үндэсний бүртгэлээс иргэн/байгууллагыг
// РД-гээр хайдаг тул зөвхөн 'users.manage' эрхтэй ажилтан хандана (admin давна) —
// эс бөгөөс дурын нэвтэрсэн хэрэглэгч иргэдийн PII-г чөлөөтэй хайх боломжтой болно.
type coreRoute struct {
	handler        corehandler.Handler
	rbacUC         rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewCoreRoute(router chi.Router, coreUC coreuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *coreRoute {
	return &coreRoute{
		handler:        corehandler.NewHandler(coreUC),
		rbacUC:         rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *coreRoute) Routes() {
	rt.router.Route("/v1/core", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		manage := middlewares.RequirePermission(rt.rbacUC, domain.PermUsersManage)
		r.With(manage).Get("/users", v1.Wrap(rt.handler.FindUsers))
		r.With(manage).Get("/organizations", v1.Wrap(rt.handler.FindOrganizations))
	})
}

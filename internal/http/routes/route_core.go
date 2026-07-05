// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	coreuc "template/internal/business/usecases/core"
	v1 "template/internal/http/handlers/v1"
	corehandler "template/internal/http/handlers/v1/core"
)

// coreRoute нь Gerege Core (core.gerege.mn)-ийн хайлтын /core/* бүлгийг холбоно.
// Нэвтэрсэн хэрэглэгч шаардана (service token нь backend-д нуугдсан).
type coreRoute struct {
	handler        corehandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewCoreRoute(router chi.Router, coreUC coreuc.Usecase, authMiddleware func(http.Handler) http.Handler) *coreRoute {
	return &coreRoute{
		handler:        corehandler.NewHandler(coreUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *coreRoute) Routes() {
	rt.router.Route("/v1/core", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Get("/users", v1.Wrap(rt.handler.FindUsers))
		r.Get("/organizations", v1.Wrap(rt.handler.FindOrganizations))
	})
}

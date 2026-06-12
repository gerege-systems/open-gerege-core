// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	audituc "template/internal/business/usecases/audit"
	v1 "template/internal/http/handlers/v1"
	audithandler "template/internal/http/handlers/v1/audit"
	"template/internal/http/middlewares"
)

// auditRoute нь /v1/audit/* бүлгийг холбоно. Бүх endpoint нь нэвтрэлт +
// admin шаардана (rbac/admin route-уудтай ижил RequireAdmin gating).
type auditRoute struct {
	handler        audithandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

// NewAuditRoute нь route модулийг бүтээдэг.
func NewAuditRoute(router chi.Router, auditUC audituc.Usecase, authMiddleware func(http.Handler) http.Handler) *auditRoute {
	return &auditRoute{
		handler:        audithandler.NewHandler(auditUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

// Routes нь /v1/audit бүлэг болон түүний endpoint-уудыг суулгана (admin-only).
func (rt *auditRoute) Routes() {
	rt.router.Route("/v1/audit", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Use(middlewares.RequireAdmin())

		r.Get("/", v1.Wrap(rt.handler.List))
		r.Get("/verify", v1.Wrap(rt.handler.Verify))
	})
}

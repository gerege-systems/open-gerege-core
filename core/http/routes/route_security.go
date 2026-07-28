// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	securityuc "github.com/gerege-systems/public-gerege-core/core/business/usecases/security"
	v1 "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1"
	securityhandler "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1/security"
	"github.com/gerege-systems/public-gerege-core/core/http/middlewares"
)

// securityRoute нь /v1/security/* бүлгийг холбоно. POST /events нь нэвтэрсэн
// хэрэглэгч бүрт нээлттэй (зөвхөн authMiddleware); GET /events нь admin-only.
type securityRoute struct {
	handler        securityhandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

// NewSecurityRoute нь route модулийг бүтээдэг.
func NewSecurityRoute(router chi.Router, securityUC securityuc.Usecase, authMiddleware func(http.Handler) http.Handler) *securityRoute {
	return &securityRoute{
		handler:        securityhandler.NewHandler(securityUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

// Routes нь /v1/security бүлэг болон endpoint-уудыг суулгана.
func (rt *securityRoute) Routes() {
	rt.router.Route("/v1/security", func(r chi.Router) {
		r.Use(rt.authMiddleware)

		// Ингест — нэвтэрсэн хэрэглэгч бүрт нээлттэй (RLS нь user_id-г баталгаажуулна).
		r.Post("/events", v1.Wrap(rt.handler.Ingest))

		// Жагсаалт — зөвхөн admin.
		r.With(middlewares.RequireAdmin()).Get("/events", v1.Wrap(rt.handler.List))
	})
}

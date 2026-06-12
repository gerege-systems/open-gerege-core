// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	audituc "template/internal/business/usecases/audit"
	orguc "template/internal/business/usecases/org"
	v1 "template/internal/http/handlers/v1"
	orghandler "template/internal/http/handlers/v1/org"
)

// orgRoute нь /v1/org/* бүлгийг холбоно — байгууллага болон гишүүнчлэлийн бүх
// endpoint. Бүх endpoint authMiddleware-ийн ард (нэвтрэлт заавал) байрлана;
// эрх олголт (owner/admin эсэх) нь usecase давхаргад хэрэгждэг.
type orgRoute struct {
	handler        orghandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

// NewOrgRoute нь route модулийг бүтээдэг. Auth middleware-г дамжуулдаг тул ижил
// JWT баталгаажуулдаг middleware нь хамгаалагдсан route бүлэг бүрд хуваалцагдана.
func NewOrgRoute(router chi.Router, orgUC orguc.Usecase, auditUC audituc.Usecase, authMiddleware func(http.Handler) http.Handler) *orgRoute {
	return &orgRoute{
		handler:        orghandler.NewHandlerWithAudit(orgUC, auditUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

// Routes нь /v1/org бүлэг болон түүний endpoint-уудыг суулгана.
func (rt *orgRoute) Routes() {
	rt.router.Route("/v1/org", func(r chi.Router) {
		r.Use(rt.authMiddleware)

		r.Post("/", v1.Wrap(rt.handler.CreateOrganization))
		r.Get("/", v1.Wrap(rt.handler.ListMyOrganizations))
		r.Get("/lookup/{regNo}", v1.Wrap(rt.handler.LookupByRegNo))
		r.Get("/{id}", v1.Wrap(rt.handler.GetOrganization))
		r.Get("/{id}/members", v1.Wrap(rt.handler.ListMembers))
		r.Post("/{id}/members", v1.Wrap(rt.handler.AddMember))
		r.Put("/{id}/members/{userID}", v1.Wrap(rt.handler.UpdateMemberRole))
		r.Delete("/{id}/members/{userID}", v1.Wrap(rt.handler.RemoveMember))
	})
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	rbacuc "template/internal/business/usecases/rbac"
	registryuc "template/internal/business/usecases/registry"
	v1 "template/internal/http/handlers/v1"
	registryhandler "template/internal/http/handlers/v1/registry"
	"template/internal/http/middlewares"
)

// registryRoute нь Ring System · R1 — Үйлчилгээний нэгдсэн регистрийн
// /registry/* бүлгийг холбоно. Эрхийн хоёр түвшин:
//   - 'registry.view'   — уншилт (жагсаалт, паспорт, once-only самбар)
//   - 'registry.manage' — бичилт (үүсгэх/засах/устгах/нийтлэх)
//
// admin нь каталогийн бүх эрхэд авто-resolve хийдэг тул хоёуланд нь давна.
type registryRoute struct {
	handler        registryhandler.Handler
	resolver       rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewRegistryRoute(router chi.Router, registryUC registryuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *registryRoute {
	return &registryRoute{
		handler:        registryhandler.NewHandler(registryUC),
		resolver:       rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *registryRoute) Routes() {
	view := middlewares.RequirePermission(rt.resolver, domain.PermRegistryView)
	manage := middlewares.RequirePermission(rt.resolver, domain.PermRegistryManage)

	rt.router.Route("/v1/registry", func(r chi.Router) {
		r.Use(rt.authMiddleware)

		// ── Уншилт (registry.view) ────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(view)

			r.Get("/overview", v1.Wrap(rt.handler.Overview))
			// Каталог нь зөвхөн нийтлэгдсэн паспортыг буцаана (usecase-д
			// албадсан) — ноорог үйлчилгээ гадагш гарахгүй.
			r.Get("/catalog", v1.Wrap(rt.handler.Catalog))
			r.Get("/once-only", v1.Wrap(rt.handler.OnceOnlyViolations))

			r.Get("/services", v1.Wrap(rt.handler.ListServices))
			r.Get("/services/{id}", v1.Wrap(rt.handler.GetService))
			r.Get("/services/{id}/versions", v1.Wrap(rt.handler.ListVersions))
			r.Get("/services/{id}/once-only", v1.Wrap(rt.handler.CheckOnceOnly))

			r.Get("/evidences", v1.Wrap(rt.handler.ListEvidences))
			r.Get("/life-events", v1.Wrap(rt.handler.ListLifeEvents))
		})

		// ── Бичилт (registry.manage) ──────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(manage)

			r.Post("/services", v1.Wrap(rt.handler.CreateService))
			r.Put("/services/{id}", v1.Wrap(rt.handler.UpdateService))
			r.Delete("/services/{id}", v1.Wrap(rt.handler.DeleteService))
			r.Post("/services/{id}/archive", v1.Wrap(rt.handler.ArchiveService))
			r.Put("/services/{id}/evidences", v1.Wrap(rt.handler.SetEvidences))
			r.Post("/services/{id}/publish", v1.Wrap(rt.handler.Publish))

			r.Post("/evidences", v1.Wrap(rt.handler.CreateEvidence))
			r.Put("/evidences/{id}", v1.Wrap(rt.handler.UpdateEvidence))
			r.Delete("/evidences/{id}", v1.Wrap(rt.handler.DeleteEvidence))

			r.Post("/life-events", v1.Wrap(rt.handler.CreateLifeEvent))
			r.Delete("/life-events/{id}", v1.Wrap(rt.handler.DeleteLifeEvent))
		})
	})
}

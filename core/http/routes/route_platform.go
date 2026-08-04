// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// platformRoute нь /v1/platform/* бүлгийг холбоно — модулийн бүртгэлийн
// нийтэд харагдах гадаргуу. Frontend nav, mobile/desktop feature flag нь
// эндээс уншиж, идэвхгүй модулийн UI-г нуудаг.
type platformRoute struct {
	registry *module.Registry
	router   chi.Router
}

// NewPlatformRoute нь route модулийг бүтээдэг.
func NewPlatformRoute(router chi.Router, registry *module.Registry) *platformRoute {
	return &platformRoute{registry: registry, router: router}
}

// moduleStatusResponse — нэг модулийн нийтийн төлөв. Хувилбар, хамаарал,
// route зэрэг дотоод мэдээллийг ЗОРИУД задлахгүй (гадны тандалт багасгана);
// админ гадаргуу дараагийн шатанд дэлгэрэнгүйг тусдаа өгнө.
type moduleStatusResponse struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
}

// Routes нь /v1/platform бүлэг болон endpoint-уудыг суулгана.
func (rt *platformRoute) Routes() {
	rt.router.Route("/v1/platform", func(r chi.Router) {
		// Нэвтрэлт шаардахгүй: login-ээс ӨМНӨХ дэлгэц ч (landing) модулийн
		// идэвхээс хамаардаг (жишээ: нийтийн AI чат харуулах эсэх).
		r.Get("/modules", v1.Wrap(rt.listModules))
	})
}

// listModules godoc
//
//	@Summary		Идэвхтэй модулиудын жагсаалт
//	@Description	Платформд суусан модулиудын нийтийн төлөв (id, kind, enabled). Клиентүүд идэвхгүй модулийн цэс/дэлгэцээ нуухад ашиглана.
//	@Tags			platform
//	@Produce		json
//	@Success		200	{object}	v1.BaseResponse
//	@Router			/v1/platform/modules [get]
func (rt *platformRoute) listModules(w http.ResponseWriter, r *http.Request) error {
	list := rt.registry.List()
	out := make([]moduleStatusResponse, 0, len(list))
	for _, s := range list {
		out = append(out, moduleStatusResponse{
			ID:      s.Manifest.ID,
			Kind:    string(s.Manifest.Kind),
			Enabled: s.Enabled,
		})
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "Модулиудын жагсаалт", out)
}

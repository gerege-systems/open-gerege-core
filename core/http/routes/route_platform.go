// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	platformuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/platform"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
)

// platformRoute нь /v1/platform/* бүлгийг холбоно — модулийн бүртгэлийн
// гадаргуу. Нийтийн жагсаалт (frontend nav, mobile/desktop feature flag) +
// админы дэлгэрэнгүй жагсаалт ба асаах/унтраах toggle.
type platformRoute struct {
	uc             platformuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

// NewPlatformRoute нь route модулийг бүтээдэг.
func NewPlatformRoute(router chi.Router, uc platformuc.Usecase, authMiddleware func(http.Handler) http.Handler) *platformRoute {
	return &platformRoute{uc: uc, router: router, authMiddleware: authMiddleware}
}

// moduleStatusResponse — нэг модулийн нийтийн төлөв. Хувилбар, хамаарал,
// route зэрэг дотоод мэдээллийг ЗОРИУД задлахгүй (гадны тандалт багасгана);
// дэлгэрэнгүйг admin endpoint өгнө.
type moduleStatusResponse struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
}

// moduleAdminResponse — админд харагдах дэлгэрэнгүй төлөв.
type moduleAdminResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Enabled   bool     `json:"enabled"`
	DependsOn []string `json:"depends_on"`
	Routes    []string `json:"routes"`
}

// moduleToggleRequest — асаах/унтраах хүсэлтийн body.
type moduleToggleRequest struct {
	Enabled *bool `json:"enabled" validate:"required"`
}

// Routes нь /v1/platform бүлэг болон endpoint-уудыг суулгана.
func (rt *platformRoute) Routes() {
	rt.router.Route("/v1/platform", func(r chi.Router) {
		// Нэвтрэлт шаардахгүй: login-ээс ӨМНӨХ дэлгэц ч (landing) модулийн
		// идэвхээс хамаардаг (жишээ: нийтийн AI чат харуулах эсэх).
		r.Get("/modules", v1.Wrap(rt.listModules))

		// Админы удирдлага — модулийг restart-гүйгээр асаах/унтраах.
		r.Route("/admin/modules", func(admin chi.Router) {
			admin.Use(rt.authMiddleware)
			admin.Use(middlewares.RequireAdmin())
			admin.Get("/", v1.Wrap(rt.listModulesAdmin))
			admin.Put("/{id}", v1.Wrap(rt.toggleModule))
		})
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
	list := rt.uc.List(r.Context())
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

// listModulesAdmin godoc
//
//	@Summary		Модулиудын дэлгэрэнгүй жагсаалт (админ)
//	@Description	Нэр, хамаарал, route угтварын хамт бүх модулийн төлөв.
//	@Tags			platform
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	v1.BaseResponse
//	@Router			/v1/platform/admin/modules [get]
func (rt *platformRoute) listModulesAdmin(w http.ResponseWriter, r *http.Request) error {
	list := rt.uc.List(r.Context())
	out := make([]moduleAdminResponse, 0, len(list))
	for _, s := range list {
		out = append(out, moduleAdminResponse{
			ID:        s.Manifest.ID,
			Name:      s.Manifest.Name,
			Kind:      string(s.Manifest.Kind),
			Enabled:   s.Enabled,
			DependsOn: s.Manifest.DependsOn,
			Routes:    s.Manifest.RoutePrefixes,
		})
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "Модулиудын дэлгэрэнгүй жагсаалт", out)
}

// toggleModule godoc
//
//	@Summary		Модулийг асаах/унтраах (админ)
//	@Description	Business модулийг restart-гүйгээр асааж/унтраана. Core модуль, идэвхтэй хамаарагчтай модульд алдаа буцна. Төлөв DB-д хадгалагдаж дараагийн boot-д сэргэнэ.
//	@Tags			platform
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path	string				true	"Модулийн ID"
//	@Param			payload	body	moduleToggleRequest	true	"Шинэ төлөв"
//	@Success		200	{object}	v1.BaseResponse
//	@Failure		400	{object}	v1.BaseResponse
//	@Router			/v1/platform/admin/modules/{id} [put]
func (rt *platformRoute) toggleModule(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req moduleToggleRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.RespondWithError(w, r, apperror.BadRequest("Хүсэлтийн body буруу"))
	}
	if req.Enabled == nil {
		return v1.RespondWithError(w, r, apperror.BadRequest("enabled талбар шаардлагатай"))
	}
	if err := rt.uc.SetEnabled(r.Context(), id, *req.Enabled); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	msg := "Модуль унтарлаа"
	if *req.Enabled {
		msg = "Модуль аслаа"
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, msg, moduleStatusResponse{
		ID: id, Enabled: *req.Enabled,
	})
}

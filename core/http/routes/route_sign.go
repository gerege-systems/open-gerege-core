// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	assetsuc "github.com/gerege-systems/public-gerege-core/core/business/usecases/assets"
	signuc "github.com/gerege-systems/public-gerege-core/core/business/usecases/sign"
	"github.com/gerege-systems/public-gerege-core/core/business/usecases/users"
	v1 "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1"
	signhandler "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1/sign"

	"github.com/go-chi/chi/v5"
)

// signRoute нь /v1/sign/* бүлгийг холбоно — PDF гарын үсэг (PAdES) eidmongolia
// /v3-ээр. Бүгд нэвтэрсэн иргэн шаардана (authMiddleware).
type signRoute struct {
	handler        signhandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewSignRoute(router chi.Router, signUC signuc.Usecase, usersUC users.Usecase, assetsUC assetsuc.Usecase, authMiddleware func(http.Handler) http.Handler) *signRoute {
	return &signRoute{
		handler:        signhandler.NewHandler(signUC, usersUC, assetsUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *signRoute) Routes() {
	rt.router.Route("/v1/sign", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		// Баримтгүй гарын үсэг — зөвхөн SHA-256 хэшид (гүйлгээ/шилжүүлгийн апп).
		// Статик "status" сегмент нь доорх "{id}" wildcard-аас ӨМНӨ таарна.
		r.Post("/initiate", v1.Wrap(rt.handler.InitiateDigest))
		r.Get("/status/{sid}", v1.Wrap(rt.handler.DigestStatus))
		r.Post("/init", v1.Wrap(rt.handler.Init))
		r.Get("/{id}", v1.Wrap(rt.handler.Poll))
		r.Get("/{id}/download", v1.Wrap(rt.handler.Download))
	})
}

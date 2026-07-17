// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"github.com/go-chi/chi/v5"

	landinguc "template/internal/business/usecases/landing"
	v1 "template/internal/http/handlers/v1"
	landinghandler "template/internal/http/handlers/v1/landing"
)

// landingRoute нь /landing/* нийтийн бүлгийг холбоно — нүүр хуудасны
// тохируулдаг харагдацыг нэвтрээгүй зочид татна. landing_config нь глобал
// (RLS-гүй) тул auth middleware болон RLS context шаардахгүй; бичилт нь
// /admin/landing/config дор эрхтэй (route_admin.go).
type landingRoute struct {
	handler landinghandler.Handler
	router  chi.Router
}

func NewLandingRoute(router chi.Router, landingUC landinguc.Usecase) *landingRoute {
	return &landingRoute{
		handler: landinghandler.NewHandler(landingUC),
		router:  router,
	}
}

func (rt *landingRoute) Routes() {
	rt.router.Route("/v1/landing", func(r chi.Router) {
		r.Get("/config", v1.Wrap(rt.handler.GetConfig))
	})
}

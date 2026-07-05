// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"github.com/go-chi/chi/v5"

	ssouc "template/internal/business/usecases/sso"
	v1 "template/internal/http/handlers/v1"
	ssohandler "template/internal/http/handlers/v1/sso"
	"template/internal/http/middlewares"
)

// ssoRoute нь /sso/* бүлгийг холбоно — Gerege SSO (OIDC) нэвтрэлтийн 2 дахи
// урсгал. Нэвтрэхээс өмнөх тул ServiceRLSContext (callback-ийн upsert users
// хүснэгтэд хандана) + body хязгаар авна; auth middleware байхгүй.
type ssoRoute struct {
	handler ssohandler.Handler
	router  chi.Router
}

func NewSSORoute(router chi.Router, ssoUC ssouc.Usecase) *ssoRoute {
	return &ssoRoute{
		handler: ssohandler.NewHandler(ssoUC),
		router:  router,
	}
}

func (rt *ssoRoute) Routes() {
	rt.router.Route("/v1/sso", func(r chi.Router) {
		r.Use(middlewares.BodySizeLimitMiddleware(middlewares.AuthBodyMaxBytes))
		r.Use(middlewares.ServiceRLSContext())
		r.Post("/start", v1.Wrap(rt.handler.Start))
		r.Post("/callback", v1.Wrap(rt.handler.Callback))
		r.Post("/native", v1.Wrap(rt.handler.SSONative))
		r.Post("/logout", v1.Wrap(rt.handler.Logout))
	})
}

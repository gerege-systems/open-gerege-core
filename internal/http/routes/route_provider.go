// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	provideruc "template/internal/business/usecases/provider"
	v1 "template/internal/http/handlers/v1"
	providerhandler "template/internal/http/handlers/v1/provider"
	"template/internal/http/middlewares"
)

// providerRoute нь /v1/provider/* бүлгийг холбоно — dan-ийг OIDC provider
// болгосон login/consent/logout зохицуулалт (Next.js BFF-ийн /login, /consent,
// /logout хуудсууд дуудна). accept endpoint-ууд нэвтэрсэн иргэнийг (session)
// шаардана; get/reject/logout нь Hydra challenge-д тулгуурлана.
type providerRoute struct {
	handler        providerhandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewProviderRoute(router chi.Router, uc provideruc.Usecase, authMiddleware func(http.Handler) http.Handler) *providerRoute {
	return &providerRoute{
		handler:        providerhandler.NewHandler(uc),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *providerRoute) Routes() {
	rt.router.Route("/v1/provider", func(r chi.Router) {
		r.Use(middlewares.BodySizeLimitMiddleware(middlewares.AuthBodyMaxBytes))

		// Challenge-д тулгуурласан (нэвтрэлт шаардахгүй) — зөвхөн challenge-ийн
		// эзэмшигч л зөв утга дуудна.
		r.Get("/login", v1.Wrap(rt.handler.GetLogin))
		r.Get("/consent", v1.Wrap(rt.handler.GetConsent))
		r.Post("/login/reject", v1.Wrap(rt.handler.RejectLogin))
		r.Post("/consent/reject", v1.Wrap(rt.handler.RejectConsent))
		r.Post("/logout/accept", v1.Wrap(rt.handler.AcceptLogout))

		// Нэвтэрсэн иргэнийг шаардах — subject = dan user ID.
		r.Group(func(pr chi.Router) {
			pr.Use(rt.authMiddleware)
			pr.Post("/login/accept", v1.Wrap(rt.handler.AcceptLogin))
			pr.Post("/consent/accept", v1.Wrap(rt.handler.AcceptConsent))
		})
	})
}

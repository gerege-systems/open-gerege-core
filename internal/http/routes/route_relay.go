// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
	rbacuc "template/internal/business/usecases/rbac"
	relayuc "template/internal/business/usecases/relay"
	v1 "template/internal/http/handlers/v1"
	relayhandler "template/internal/http/handlers/v1/relay"
	"template/internal/http/middlewares"
)

// relayRoute нь /relay/* бүлгийг холбоно. Бүх endpoint нь JWT + relay эрх
// шаардана (relay.view — унших, relay.manage — бичих). Ingest/Respond нь энэ
// template scaffold-д JWT + relay.manage-аар хамгаалагдана; production-д дээд/доод
// platform-ууд эдгээрийг gateway (m2m OAuth)-аар дуудна.
type relayRoute struct {
	handler        relayhandler.Handler
	resolver       rbacuc.Usecase
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewRelayRoute(router chi.Router, relayUC relayuc.Usecase, rbacUC rbacuc.Usecase, authMiddleware func(http.Handler) http.Handler) *relayRoute {
	return &relayRoute{
		handler:        relayhandler.NewHandler(relayUC),
		resolver:       rbacUC,
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *relayRoute) Routes() {
	view := middlewares.RequirePermission(rt.resolver, domain.PermRelayView)
	manage := middlewares.RequirePermission(rt.resolver, domain.PermRelayManage)

	// Peer webhook (дээш/доош) — JWT-гүй, HMAC гарын үсгээр баталгаажна. authMiddleware-
	// ийн ГАДНА бүртгэнэ.
	rt.router.Post("/v1/relay/webhook", v1.Wrap(rt.handler.ReceiveWebhook))

	rt.router.Route("/v1/relay", func(r chi.Router) {
		r.Use(rt.authMiddleware)

		// Ingest / Respond (m2m урсгал — scaffold-д relay.manage-аар).
		r.With(manage).Post("/requests", v1.Wrap(rt.handler.Ingest))
		r.With(manage).Post("/assignments/{id}/respond", v1.Wrap(rt.handler.Respond))
		r.With(manage).Post("/requests/{id}/forward", v1.Wrap(rt.handler.ForwardUp))

		// Dashboard.
		r.With(view).Get("/overview", v1.Wrap(rt.handler.Overview))
		r.With(view).Get("/requests", v1.Wrap(rt.handler.ListRequests))
		r.With(view).Get("/requests/{id}", v1.Wrap(rt.handler.GetRequest))

		// Platforms / routes (admin config).
		r.With(view).Get("/platforms", v1.Wrap(rt.handler.ListPlatforms))
		r.With(manage).Post("/platforms", v1.Wrap(rt.handler.CreatePlatform))
		r.With(manage).Delete("/platforms/{id}", v1.Wrap(rt.handler.DeletePlatform))
		r.With(view).Get("/routes", v1.Wrap(rt.handler.ListRoutes))
		r.With(manage).Post("/routes", v1.Wrap(rt.handler.CreateRoute))
		r.With(manage).Delete("/routes/{id}", v1.Wrap(rt.handler.DeleteRoute))
	})
}

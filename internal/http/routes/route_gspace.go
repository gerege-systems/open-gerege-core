// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	gspaceuc "template/internal/business/usecases/gspace"
	v1 "template/internal/http/handlers/v1"
	gspacehandler "template/internal/http/handlers/v1/gspace"
	"template/internal/http/middlewares"
)

// gspaceRoute нь "Gerege Space" (апп-ын өөрийн SFTP хадгалалт)-ын /v1/gspace/*
// бүлгийг холбоно.
type gspaceRoute struct {
	handler          gspacehandler.Handler
	router           chi.Router
	authMiddleware   func(http.Handler) http.Handler
	writeRateLimiter *middlewares.RateLimiter
}

func NewGSpaceRoute(router chi.Router, gspaceUC gspaceuc.Usecase, authMiddleware func(http.Handler) http.Handler, writeRateLimiter *middlewares.RateLimiter) *gspaceRoute {
	return &gspaceRoute{
		handler:          gspacehandler.NewHandler(gspaceUC),
		router:           router,
		authMiddleware:   authMiddleware,
		writeRateLimiter: writeRateLimiter,
	}
}

func (rt *gspaceRoute) Routes() {
	rt.router.Route("/v1/gspace", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		write := rt.writeRateLimiter.Middleware()

		r.Get("/", v1.Wrap(rt.handler.Overview))
		r.Get("/download", v1.Wrap(rt.handler.Download))
		r.With(write).Post("/upload", v1.Wrap(rt.handler.Upload))
		r.With(write).Delete("/", v1.Wrap(rt.handler.Delete))
	})
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	authuc "template/internal/business/usecases/auth"
	v1 "template/internal/http/handlers/v1"
	eidprofilehandler "template/internal/http/handlers/v1/eidprofile"
)

// eidProfileRoute нь нэвтэрсэн хэрэглэгчийн eID нэмэлт мэдээллийг
// (/users/me/eid/*) auth middleware-ийн дор холбоно. auth usecase-ийг
// ашигладаг (eID client + users-ийн хосолсон).
type eidProfileRoute struct {
	handler        eidprofilehandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewEIDProfileRoute(router chi.Router, authUC authuc.Usecase, authMiddleware func(http.Handler) http.Handler) *eidProfileRoute {
	return &eidProfileRoute{
		handler:        eidprofilehandler.NewHandler(authUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *eidProfileRoute) Routes() {
	rt.router.Route("/v1/users/me/eid", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Get("/organizations", v1.Wrap(rt.handler.Organizations))
	})
}

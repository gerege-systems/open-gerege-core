// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	authuc "template/internal/business/usecases/auth"
	v1 "template/internal/http/handlers/v1"
	eidprofilehandler "template/internal/http/handlers/v1/eidprofile"
	"template/internal/http/middlewares"
)

// eidProfileRoute нь нэвтэрсэн хэрэглэгчийн eID нэмэлт мэдээллийг
// (/users/me/eid/*) auth middleware-ийн дор холбоно. auth usecase-ийг
// ашигладаг (eID client + users-ийн хосолсон).
type eidProfileRoute struct {
	handler          eidprofilehandler.Handler
	router           chi.Router
	authMiddleware   func(http.Handler) http.Handler
	writeRateLimiter *middlewares.RateLimiter
}

func NewEIDProfileRoute(router chi.Router, authUC authuc.Usecase, authMiddleware func(http.Handler) http.Handler, writeRateLimiter *middlewares.RateLimiter) *eidProfileRoute {
	return &eidProfileRoute{
		handler:          eidprofilehandler.NewHandler(authUC),
		router:           router,
		authMiddleware:   authMiddleware,
		writeRateLimiter: writeRateLimiter,
	}
}

func (rt *eidProfileRoute) Routes() {
	rt.router.Route("/v1/users/me/eid", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		// write нь мутаци (байгууллага холбох) POST-д per-IP хязгаар нэмнэ;
		// уншилтын GET-үүд хязгааргүй хэвээр.
		write := rt.writeRateLimiter.Middleware()

		r.Get("/organizations", v1.Wrap(rt.handler.Organizations))
		r.With(write).Post("/organizations", v1.Wrap(rt.handler.AddOrganization))
		r.With(write).Delete("/organizations/{regNo}", v1.Wrap(rt.handler.RemoveOrganization))
		// Байгууллагын гарын үсэг зурагчид (нэвтэрсэн иргэн тухайн байгууллагын төлөөлөгч байх ёстой).
		r.Get("/organizations/{regNo}/signers", v1.Wrap(rt.handler.OrgSigners))
		r.With(write).Post("/organizations/{regNo}/signers", v1.Wrap(rt.handler.AddOrgSigner))
		r.With(write).Post("/organizations/{regNo}/signers/resend", v1.Wrap(rt.handler.ResendOrgSigner))
		r.With(write).Delete("/organizations/{regNo}/signers", v1.Wrap(rt.handler.RemoveOrgSigner))
		r.Get("/summary", v1.Wrap(rt.handler.Summary))
		r.Get("/certificates", v1.Wrap(rt.handler.Certificates))
		r.Get("/devices", v1.Wrap(rt.handler.Devices))
		r.Get("/activity", v1.Wrap(rt.handler.Activity))
	})
}

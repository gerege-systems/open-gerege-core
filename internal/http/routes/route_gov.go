// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	govuc "template/internal/business/usecases/gov"
	v1 "template/internal/http/handlers/v1"
	govhandler "template/internal/http/handlers/v1/gov"
	"template/internal/http/middlewares"
)

// govRoute нь иргэний "Төрийн үйлчилгээ" порталын /gov/* бүлгийг холбоно. Бүгд
// нэвтэрсэн хэрэглэгч шаардана (хувийн өгөгдөл; userID токеноос авагдана).
type govRoute struct {
	handler          govhandler.Handler
	router           chi.Router
	authMiddleware   func(http.Handler) http.Handler
	writeRateLimiter *middlewares.RateLimiter
}

func NewGovRoute(router chi.Router, govUC govuc.Usecase, authMiddleware func(http.Handler) http.Handler, writeRateLimiter *middlewares.RateLimiter) *govRoute {
	return &govRoute{
		handler:          govhandler.NewHandler(govUC),
		router:           router,
		authMiddleware:   authMiddleware,
		writeRateLimiter: writeRateLimiter,
	}
}

func (rt *govRoute) Routes() {
	rt.router.Route("/v1/gov", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		// write нь мутаци (POST) endpoint-уудад л per-IP хязгаар нэмнэ —
		// уншилтын GET-үүд (dashboard-ийн жагсаалтууд) хязгааргүй хэвээр.
		write := rt.writeRateLimiter.Middleware()

		r.Get("/services", v1.Wrap(rt.handler.ListServices))
		r.Get("/overview", v1.Wrap(rt.handler.Overview))

		r.Get("/applications", v1.Wrap(rt.handler.ListApplications))
		r.With(write).Post("/applications", v1.Wrap(rt.handler.Apply))
		r.With(write).Post("/applications/{id}/cancel", v1.Wrap(rt.handler.CancelApplication))

		r.Get("/references", v1.Wrap(rt.handler.ListReferences))
		r.With(write).Post("/references", v1.Wrap(rt.handler.RequestReference))

		r.Get("/notifications", v1.Wrap(rt.handler.ListNotifications))
		r.With(write).Post("/notifications/read-all", v1.Wrap(rt.handler.MarkAllRead))
		r.With(write).Post("/notifications/{id}/read", v1.Wrap(rt.handler.MarkNotificationRead))

		r.Get("/payments", v1.Wrap(rt.handler.ListPayments))
		r.With(write).Post("/payments/{id}/pay", v1.Wrap(rt.handler.PayPayment))

		r.Get("/appointments", v1.Wrap(rt.handler.ListAppointments))
		r.With(write).Post("/appointments", v1.Wrap(rt.handler.BookAppointment))
		r.With(write).Post("/appointments/{id}/cancel", v1.Wrap(rt.handler.CancelAppointment))
	})
}

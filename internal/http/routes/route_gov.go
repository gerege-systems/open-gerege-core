// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	govuc "template/internal/business/usecases/gov"
	v1 "template/internal/http/handlers/v1"
	govhandler "template/internal/http/handlers/v1/gov"
)

// govRoute нь иргэний "Төрийн үйлчилгээ" порталын /gov/* бүлгийг холбоно. Бүгд
// нэвтэрсэн хэрэглэгч шаардана (хувийн өгөгдөл; userID токеноос авагдана).
type govRoute struct {
	handler        govhandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewGovRoute(router chi.Router, govUC govuc.Usecase, authMiddleware func(http.Handler) http.Handler) *govRoute {
	return &govRoute{
		handler:        govhandler.NewHandler(govUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *govRoute) Routes() {
	rt.router.Route("/v1/gov", func(r chi.Router) {
		r.Use(rt.authMiddleware)

		r.Get("/services", v1.Wrap(rt.handler.ListServices))
		r.Get("/overview", v1.Wrap(rt.handler.Overview))

		r.Get("/applications", v1.Wrap(rt.handler.ListApplications))
		r.Post("/applications", v1.Wrap(rt.handler.Apply))
		r.Post("/applications/{id}/cancel", v1.Wrap(rt.handler.CancelApplication))

		r.Get("/references", v1.Wrap(rt.handler.ListReferences))
		r.Post("/references", v1.Wrap(rt.handler.RequestReference))

		r.Get("/notifications", v1.Wrap(rt.handler.ListNotifications))
		r.Post("/notifications/read-all", v1.Wrap(rt.handler.MarkAllRead))
		r.Post("/notifications/{id}/read", v1.Wrap(rt.handler.MarkNotificationRead))

		r.Get("/payments", v1.Wrap(rt.handler.ListPayments))
		r.Post("/payments/{id}/pay", v1.Wrap(rt.handler.PayPayment))

		r.Get("/appointments", v1.Wrap(rt.handler.ListAppointments))
		r.Post("/appointments", v1.Wrap(rt.handler.BookAppointment))
		r.Post("/appointments/{id}/cancel", v1.Wrap(rt.handler.CancelAppointment))
	})
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"template/internal/business/domain"
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
	rbacUC           middlewares.PermissionResolver
	authMiddleware   func(http.Handler) http.Handler
	writeRateLimiter *middlewares.RateLimiter
}

func NewGovRoute(router chi.Router, govUC govuc.Usecase, rbacUC middlewares.PermissionResolver, authMiddleware func(http.Handler) http.Handler, writeRateLimiter *middlewares.RateLimiter) *govRoute {
	return &govRoute{
		handler:          govhandler.NewHandler(govUC),
		router:           router,
		rbacUC:           rbacUC,
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
		r.Get("/life-events", v1.Wrap(rt.handler.ListLifeEvents))
		r.Get("/overview", v1.Wrap(rt.handler.Overview))

		r.Get("/applications", v1.Wrap(rt.handler.ListApplications))
		r.With(write).Post("/applications", v1.Wrap(rt.handler.Apply))
		r.With(write).Post("/applications/{id}/cancel", v1.Wrap(rt.handler.CancelApplication))
		r.Get("/applications/{id}/timeline", v1.Wrap(rt.handler.ApplicationTimeline))
		r.With(write).Post("/applications/{id}/provide-info", v1.Wrap(rt.handler.ProvideInfo))

		// ── Менежерийн дараалал ────────────────────────────────────────────
		// Хоёр давхар хамгаалалт:
		//   1. RequirePermission(gov.review) — эрхгүй бол 403.
		//   2. OfficerRLSContext — DB давхаргад 'officer' үүрэг тавьж, зөвхөн
		//      gov хүснэгтүүдэд хандах эрх өгнө. Эрхийн шалгалт алдаатай байсан
		//      ч RLS нь users/payments/appointments-ыг ХААСАН хэвээр (fail-closed).
		r.Route("/officer", func(o chi.Router) {
			o.Use(middlewares.RequirePermission(rt.rbacUC, domain.PermGovReview))
			o.Use(middlewares.OfficerRLSContext())

			o.Get("/stats", v1.Wrap(rt.handler.QueueStats))
			o.Get("/queue", v1.Wrap(rt.handler.ListQueue))
			o.Get("/queue/{id}", v1.Wrap(rt.handler.QueueItem))
			o.With(write).Post("/queue/{id}/assign", v1.Wrap(rt.handler.Assign))
			o.With(write).Post("/queue/{id}/decide", v1.Wrap(rt.handler.Decide))
			o.With(write).Post("/queue/{id}/complete", v1.Wrap(rt.handler.Complete))
			o.With(write).Post("/queue/{id}/request-info", v1.Wrap(rt.handler.RequestInfo))
		})

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

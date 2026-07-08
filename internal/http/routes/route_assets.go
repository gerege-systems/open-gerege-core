// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	assetsuc "template/internal/business/usecases/assets"
	v1 "template/internal/http/handlers/v1"
	assetshandler "template/internal/http/handlers/v1/assets"
	"template/internal/http/middlewares"
)

// assetsRoute нь гарын үсэг (хувь хүн) ба байгууллагын тамганы дардасын зургийн
// URL-ийг удирдах /v1/users/me/* бүлгийг холбоно.
type assetsRoute struct {
	handler          assetshandler.Handler
	router           chi.Router
	authMiddleware   func(http.Handler) http.Handler
	writeRateLimiter *middlewares.RateLimiter
}

func NewAssetsRoute(router chi.Router, assetsUC assetsuc.Usecase, authMiddleware func(http.Handler) http.Handler, writeRateLimiter *middlewares.RateLimiter) *assetsRoute {
	return &assetsRoute{
		handler:          assetshandler.NewHandler(assetsUC),
		router:           router,
		authMiddleware:   authMiddleware,
		writeRateLimiter: writeRateLimiter,
	}
}

func (rt *assetsRoute) Routes() {
	// ЧУХАЛ: /v1/users/me-д mount хийвэл chi-ийн longest-prefix match нь одоо байгаа
	// GET /v1/users/me (who-am-I) endpoint-ийг шадовлаж 404 болгоно. Тиймээс зөрчилгүй
	// /v1/me namespace-д (нэрлэсэн leaf-үүдтэй) байрлуулна.
	rt.router.Route("/v1/me", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		write := rt.writeRateLimiter.Middleware()

		// Хувь хүний гарын үсэг.
		r.Get("/signature", v1.Wrap(rt.handler.GetSignature))
		r.With(write).Put("/signature", v1.Wrap(rt.handler.SetSignature))
		r.With(write).Delete("/signature", v1.Wrap(rt.handler.DeleteSignature))

		// Латин нэр засах (галиглалт заримдаа буруу).
		r.With(write).Put("/latin-name", v1.Wrap(rt.handler.SetLatinName))
		r.With(write).Put("/org-name-latin/{regNo}", v1.Wrap(rt.handler.SetOrgNameLatin))

		// Байгууллагын тамганы дардас (зөвхөн ADMIN бичнэ).
		r.Get("/orgstamp/{regNo}", v1.Wrap(rt.handler.GetStamp))
		r.With(write).Put("/orgstamp/{regNo}", v1.Wrap(rt.handler.SetStamp))
		r.With(write).Delete("/orgstamp/{regNo}", v1.Wrap(rt.handler.DeleteStamp))
	})
}

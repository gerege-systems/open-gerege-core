// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	registryuc "template/internal/business/usecases/registry"
	v1 "template/internal/http/handlers/v1"
	registryhandler "template/internal/http/handlers/v1/registry"
)

// catalogRoute нь Ring System · R1 — иргэн рүү харсан НИЙТИЙН үйлчилгээний
// каталогийн /catalog/* бүлгийг холбоно.
//
// /registry/*-аас ялгаатай нь энд тусгай эрх ШААРДАХГҮЙ — нэвтэрсэн дурын
// иргэн үзнэ. Оронд нь usecase давхарга ЗӨВХӨН нийтлэгдсэн паспортыг
// эргүүлнэ (PublicCatalog / PublicService); ноорог, архивласан бичлэг энэ
// гадаргуугаар хэзээ ч гарахгүй.
type catalogRoute struct {
	handler        registryhandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewCatalogRoute(router chi.Router, registryUC registryuc.Usecase, authMiddleware func(http.Handler) http.Handler) *catalogRoute {
	return &catalogRoute{
		handler:        registryhandler.NewHandler(registryUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *catalogRoute) Routes() {
	rt.router.Route("/v1/catalog", func(r chi.Router) {
		r.Use(rt.authMiddleware)

		// Зөвхөн уншилт — нийтийн каталогт мутаци байхгүй.
		r.Get("/services", v1.Wrap(rt.handler.Catalog))
		r.Get("/services/{id}", v1.Wrap(rt.handler.PublicService))
		r.Get("/life-events", v1.Wrap(rt.handler.PublicLifeEvents))
	})
}

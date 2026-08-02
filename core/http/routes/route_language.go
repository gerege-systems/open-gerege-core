// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	languageuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/language"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	languagehandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/language"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
)

// languageRoute нь /languages/* бүлгийг холбоно.
//
// Нийтийн (auth-гүй): идэвхтэй хэлний жагсаалт ба dictionary — хуудас бүрийн
// ачаалалт эдгээрийг уншдаг тул нэвтрэлт шаардах нь утгагүй (нэвтрэх хуудас ч
// орчуулагдах ёстой).
//
// Удирдлага нь ЗӨВХӨН super admin: хэл нэмэх/хасах нь бүх хэрэглэгчийн харах
// интерфейсийг өөрчилдөг тул энгийн админд ч нээхгүй (least-privilege) —
// /superadmin гадаргуутай ижил түвшин.
type languageRoute struct {
	handler        languagehandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

func NewLanguageRoute(router chi.Router, languageUC languageuc.Usecase, authMiddleware func(http.Handler) http.Handler) *languageRoute {
	return &languageRoute{
		handler:        languagehandler.NewHandler(languageUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

func (rt *languageRoute) Routes() {
	rt.router.Route("/v1/languages", func(r chi.Router) {
		// Нийтийн — апп ачаалахдаа уншина (auth-гүй).
		r.Get("/enabled", v1.Wrap(rt.handler.ListEnabled))
		r.Get("/{code}/dictionary", v1.Wrap(rt.handler.Dictionary))

		// Удирдлага — нэвтрэлт + super admin.
		r.Group(func(ar chi.Router) {
			ar.Use(rt.authMiddleware)
			ar.Use(middlewares.RequireSuperAdmin())
			ar.Get("/", v1.Wrap(rt.handler.List))
			ar.Post("/", v1.Wrap(rt.handler.Create))
			ar.Patch("/{code}", v1.Wrap(rt.handler.Update))
			ar.Delete("/{code}", v1.Wrap(rt.handler.Delete))
			ar.Put("/{code}/translations", v1.Wrap(rt.handler.PutTranslations))
			ar.Post("/{code}/translate", v1.Wrap(rt.handler.AutoTranslate))
		})
	})
}

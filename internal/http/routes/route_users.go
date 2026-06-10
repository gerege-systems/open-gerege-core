// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"template/internal/business/usecases/users"
	v1 "template/internal/http/handlers/v1"
	usershandler "template/internal/http/handlers/v1/users"
)

// usersRoute нь /users/* бүлгийг холбоно — хэрэглэгчийн өөрийнх нь
// профайл / өгөгдөлд хамаарах endpoint-ууд. Auth урсгалууд нь
// route.auth.go-д байрладаг.
type usersRoute struct {
	handler        usershandler.Handler
	router         chi.Router
	authMiddleware func(http.Handler) http.Handler
}

// NewUsersRoute нь route модулийг бүтээдэг. Auth middleware-г дамжуулдаг
// тул ижил JWT баталгаажуулдаг middleware нь хамгаалагдсан route бүлэг
// бүрд хуваалцагддаг.
func NewUsersRoute(router chi.Router, usersUC users.Usecase, authMiddleware func(http.Handler) http.Handler) *usersRoute {
	return &usersRoute{
		handler:        usershandler.NewHandler(usersUC),
		router:         router,
		authMiddleware: authMiddleware,
	}
}

// Routes нь /v1/users бүлэг болон түүний endpoint-уудыг суулгана.
func (rt *usersRoute) Routes() {
	rt.router.Route("/v1/users", func(r chi.Router) {
		r.Use(rt.authMiddleware)
		r.Get("/me", v1.Wrap(rt.handler.GetUserData))
	})
}

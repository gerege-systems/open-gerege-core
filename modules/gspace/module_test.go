// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gspace

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// fakeHost — модулийн wiring тестийн хамгийн жижиг Host.
type fakeHost struct {
	api      chi.Router
	services map[string]any
}

func (h *fakeHost) APIRouter() chi.Router { return h.api }
func (h *fakeHost) Pool() *pgxpool.Pool   { return nil }
func (h *fakeHost) AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}
func (h *fakeHost) Service(name string) (any, bool) {
	v, ok := h.services[name]
	return v, ok
}

// TestRegisterMountsRoutes — Register нь /v1/gspace бүлгээ Host router дээр
// суулгадаг ба шаардлагатай service дутуу бол тодорхой алдаа буцаадаг.
func TestRegisterMountsRoutes(t *testing.T) {
	h := &fakeHost{api: chi.NewRouter(), services: map[string]any{
		module.ServiceWriteRateLimiter: middlewares.NewRateLimiter(rate.Limit(1), 1),
	}}
	if err := New().Register(context.Background(), h); err != nil {
		t.Fatalf("Register: %v", err)
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/gspace/"},
		{http.MethodPost, "/v1/gspace/upload"},
		{http.MethodGet, "/v1/gspace/download"},
	} {
		if tctx := chi.NewRouteContext(); !h.api.Match(tctx, tc.method, tc.path) {
			t.Errorf("%s %s: маршрут суугаагүй", tc.method, tc.path)
		}
	}

	// Service дутуу → тодорхой алдаа.
	empty := &fakeHost{api: chi.NewRouter(), services: map[string]any{}}
	if err := New().Register(context.Background(), empty); err == nil {
		t.Fatal("write limiter service дутуу байхад алдаа гарах ёстой")
	}
}

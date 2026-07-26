// Маршрутын хэв шинжийн smoke тест — DB/сервер шаардлагагүй.
//
// Юуг хамгаалж байна вэ: chi нь зөрчилдсөн хэв шинжийг БҮРТГЭХ үедээ panic
// хийдэг тул алдаа нь зөвхөн процесс асах мөчид илэрдэг — unit тест, build
// хоёулаа барихгүй. Гар утасны урсгалд нэмэгдсэн статик сегментүүд
// (/auth/status/{sid}, /sign/status/{sid}) нь ижил түвшний wildcard-уудтай
// (/sign/{id}) зэрэгцэн оршдог тул энэ хосолмолыг тусад нь баталгаажуулна.
package routes

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAuthAndSignPatternsRegisterWithoutConflict(t *testing.T) {
	noop := func(w http.ResponseWriter, r *http.Request) {}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("chi нь маршрутын хэв шинжийг бүртгэхэд panic хийв: %v", rec)
		}
	}()

	r := chi.NewRouter()

	r.Route("/auth", func(a chi.Router) {
		a.Post("/eid/start", noop)
		a.Post("/eid/start-id", noop)
		a.Get("/eid/poll", noop)
		a.Post("/google", noop)
		a.Post("/google/link", noop)
		a.Post("/refresh", noop)
		a.Post("/logout", noop)
		a.Post("/initiate", noop)
		a.Get("/status/{sid}", noop)
	})

	r.Route("/sign", func(s chi.Router) {
		s.Post("/init", noop)
		s.Post("/initiate", noop)
		s.Get("/status/{sid}", noop)
		s.Get("/{id}", noop)
		s.Get("/{id}/download", noop)
	})

	// Статик сегмент нь ижил түвшний wildcard-аас ТҮРҮҮЛЖ таарах ёстой —
	// эс тэгвээс /sign/status/... нь /sign/{id}-д залгигдана.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/sign/status/abc"},
		{http.MethodGet, "/sign/xyz"},
		{http.MethodGet, "/sign/xyz/download"},
		{http.MethodGet, "/auth/status/abc"},
		{http.MethodPost, "/auth/initiate"},
	} {
		if tctx := chi.NewRouteContext(); !r.Match(tctx, tc.method, tc.path) {
			t.Errorf("%s %s: маршрут таарсангүй", tc.method, tc.path)
		}
	}
}

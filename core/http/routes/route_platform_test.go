// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	platformuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/platform"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// TestPlatformModulesEndpoint — /v1/platform/modules нь модулиудын нийтийн
// төлөвийг зөв хэлбэрээр буцаадаг ба унтраасан модуль enabled=false харагдана.
func TestPlatformModulesEndpoint(t *testing.T) {
	reg := module.Builtin()
	if err := reg.Disable("gspace"); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		api.Use(module.Gate(reg))
		routes.NewPlatformRoute(api, platformuc.NewUsecase(reg, nil, nil), func(h http.Handler) http.Handler { return h }).Routes()
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/platform/modules", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}

	var body struct {
		Status bool `json:"status"`
		Data   []struct {
			ID      string `json:"id"`
			Kind    string `json:"kind"`
			Enabled bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v (%s)", err, rec.Body.String())
	}
	if !body.Status || len(body.Data) < 20 {
		t.Fatalf("хариу дутуу: status=%v модуль=%d", body.Status, len(body.Data))
	}
	seen := map[string]bool{}
	for _, m := range body.Data {
		seen[m.ID] = true
		switch m.ID {
		case "gspace":
			if m.Enabled {
				t.Error("gspace унтраасан байх ёстой")
			}
		case "auth":
			if !m.Enabled || m.Kind != "core" {
				t.Errorf("auth: enabled=%v kind=%s", m.Enabled, m.Kind)
			}
		}
	}
	if !seen["gov"] || !seen["ai"] || !seen["platform"] {
		t.Fatalf("гол модулиуд жагсаалтад алга: %v", seen)
	}

	// Gate: унтарсан gspace-ийн route 404 (жагсаалтын endpoint өөрөө нээлттэй хэвээр).
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/v1/gspace/", http.NoBody))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("унтарсан модулийн зам: %d, 404 хүлээсэн", rec2.Code)
	}
}

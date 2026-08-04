// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Golden route inventory — modular refactor-ийн Phase 0 хамгаалалтын тор.
//
// Юуг хамгаалж байна вэ: (1) бүртгэгддэг endpoint-уудын БҮРЭН жагсаалт
// testdata/routes_golden.txt-тэй үг үсэггүй тохирохыг — refactor-ийн явцад
// route санамсаргүй алга болох/өөрчлөгдөхийг build дээр барина; (2) endpoint
// бүр kernel/module-ийн builtin манифестийн ЯГ НЭГ модульд харьяалагдахыг —
// "эзэнгүй" route гарахгүй гэсэн модульчлалын үндсэн инвариант.
//
// Route угсралт нь server.go-ийн /api блокийг stub хамаарлуудаар яг
// давтдаг (усecase-ууд nil interface — бүртгэлийн үед дуудагддаггүй тул
// аюулгүй; handler ажиллуулбал panic болно, гэхдээ энэ тест хүсэлт явуулдаггүй).
// Шинэ route нэмбэл: (1) энд угсралтад нэм (server.go-той зэрэгцүүлж),
// (2) `go test ./core/http/routes -run TestGoldenRoutes -update` ажиллуулж
// golden файлыг шинэчилж commit хий, (3) эзэн модулийг kernel/module/builtin.go-д
// зарла.
package routes_test

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"

	platformuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/platform"
	sitehandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/site"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

var update = flag.Bool("update", false, "golden файлыг дахин бичих")

// kernelOwnedPatterns — модульд бус kernel-д харьяалагдах цөөн зам.
var kernelOwnedPatterns = map[string]bool{
	"GET /api/": true, // RootHandler "alive" хариу
}

// buildFullRouter нь server.go-ийн route угсралтыг stub хамаарлуудаар давтана.
func buildFullRouter(t *testing.T) chi.Router {
	t.Helper()

	noopMW := func(h http.Handler) http.Handler { return h }
	rl := func() *middlewares.RateLimiter { return middlewares.NewRateLimiter(rate.Limit(100), 100) }
	reg := module.Builtin()

	r := chi.NewRouter()

	// OIDC нийтийн endpoint-ууд — /api-ийн гадна, үндэс дээр (server.go-той ижил).
	routes.NewOIDCRoute(r, nil, nil, "https://issuer.test").Routes()

	r.Route("/api", func(api chi.Router) {
		api.Get("/", routes.RootHandler)
		routes.NewAuthRoute(api, nil, nil, nil, noopMW, rl(), rl()).Routes()
		routes.NewUsersRoute(api, nil, noopMW, true).Routes()
		routes.NewEIDProfileRoute(api, nil, noopMW, rl()).Routes()
		routes.NewRBACRoute(api, nil, nil, noopMW).Routes()
		routes.NewOrgRoute(api, nil, nil, noopMW).Routes()
		routes.NewGovRoute(api, nil, nil, noopMW, rl()).Routes()
		routes.NewIntegrationsRoute(api, nil, noopMW).Routes()
		routes.NewAssetsRoute(api, nil, noopMW, rl()).Routes()
		routes.NewGSpaceRoute(api, nil, noopMW, rl()).Routes()
		routes.NewGatewayRoute(api, nil, nil, noopMW).Routes()
		routes.NewRelayRoute(api, nil, nil, noopMW).Routes()
		routes.NewRegistryRoute(api, nil, nil, noopMW).Routes()
		routes.NewCatalogRoute(api, nil, noopMW).Routes()
		routes.NewApplicationsRoute(api, nil, nil, noopMW).Routes()
		routes.NewCoreRoute(api, nil, nil, noopMW).Routes()
		routes.NewSSORoute(api, nil).Routes()
		routes.NewAdminRoute(api, nil, nil, noopMW).Routes()
		routes.NewAdminAIRoute(api, nil, nil, noopMW).Routes()
		routes.NewSuperAdminRoute(api, nil, noopMW).Routes()
		routes.NewSuperAdminOnboardRoute(api, nil, rl(), rl()).Routes()
		routes.NewAIRoute(api, nil, noopMW, rl()).Routes()
		routes.NewPublicAIRoute(api, nil, rl(), rl()).Routes()
		routes.NewAuditRoute(api, nil, noopMW).Routes()
		routes.NewSecurityRoute(api, nil, noopMW).Routes()
		routes.NewSiteRoute(api, nil, nil, noopMW, sitehandler.AuthSurface{}).Routes()
		routes.NewThemeRoute(api, nil, nil, noopMW).Routes()
		routes.NewLanguageRoute(api, nil, noopMW).Routes()
		routes.NewSignRoute(api, nil, nil, nil, noopMW).Routes()
		routes.NewEIDProxyRoute(api, nil, nil, noopMW, noopMW).Routes()
		routes.NewEIDAuthRoute(api, nil, nil, noopMW, noopMW).Routes()
		routes.NewProviderRoute(api, nil, noopMW).Routes()
		routes.NewPlatformRoute(api, platformuc.NewUsecase(reg, nil, nil), noopMW).Routes()
	})
	return r
}

// walkRoutes нь "METHOD PATTERN" мөрүүдийг эрэмбэлж буцаана.
func walkRoutes(t *testing.T, r chi.Router) []string {
	t.Helper()
	var lines []string
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		lines = append(lines, fmt.Sprintf("%s %s", method, route))
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	sort.Strings(lines)
	return lines
}

func TestGoldenRoutes(t *testing.T) {
	lines := walkRoutes(t, buildFullRouter(t))
	got := strings.Join(lines, "\n") + "\n"

	golden := filepath.Join("testdata", "routes_golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden шинэчлэгдлээ: %d route", len(lines))
		return
	}
	wantBytes, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden файл алга (%v) — `go test -run TestGoldenRoutes -update`-ээр үүсгэ", err)
	}
	if got != string(wantBytes) {
		gotSet := map[string]bool{}
		for _, l := range lines {
			gotSet[l] = true
		}
		wantSet := map[string]bool{}
		for _, l := range strings.Split(strings.TrimSpace(string(wantBytes)), "\n") {
			wantSet[l] = true
		}
		for l := range wantSet {
			if !gotSet[l] {
				t.Errorf("route АЛГА БОЛСОН: %s", l)
			}
		}
		for l := range gotSet {
			if !wantSet[l] {
				t.Errorf("ШИНЭ route (golden-д алга): %s", l)
			}
		}
		t.Fatal("route inventory golden-оос зөрлөө — санаатай өөрчлөлт бол -update-ээр шинэчил")
	}
}

// TestEveryRouteHasOwnerModule — endpoint бүр яг нэг модульд харьяалагдана.
func TestEveryRouteHasOwnerModule(t *testing.T) {
	reg := module.Builtin()
	for _, line := range walkRoutes(t, buildFullRouter(t)) {
		if kernelOwnedPatterns[line] {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		pattern := parts[1]
		// chi pattern-ийг угтвар тааруулалтад шууд ашиглаж болно:
		// модулийн угтварууд placeholder-оос өмнө төгсдөг статик замууд.
		if id, ok := reg.ModuleForPath(pattern); !ok {
			t.Errorf("ЭЗЭНГҮЙ route: %s — kernel/module/builtin.go-д эзэн модуль зарла", line)
		} else if _, exists := reg.Get(id); !exists {
			t.Errorf("%s: үл мэдэгдэх модуль %q", line, id)
		}
	}
}

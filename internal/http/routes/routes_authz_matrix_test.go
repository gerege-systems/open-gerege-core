//go:build integration

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Role × endpoint эрхийн matrix-ийн integration тест. Жинхэнэ Postgres
// (migration-ууд + RBAC seed), жинхэнэ Redis, жинхэнэ JWT, жинхэнэ
// middleware/route угсралт ашиглан дөрвөн persona (нэвтрээгүй, user,
// manager, admin) бүрээр хамгаалагдсан endpoint бүрийн 401/403/зөвшөөрөл
// зан төлөвийг сервер талын бүтэн давхаргаар баталгаажуулна.
//
// "Зөвшөөрөгдсөн" тохиолдолд handler дотоод шалтгаанаар 404 г.м. буцааж
// болох тул шаардлага нь: хариу 401/403 БИШ байх (эрхийн хаалга нээгдсэн).
// Гол admin/manager унших endpoint-уудад 200-г шууд шаардана.
package routes_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"template/internal/business/usecases/ai"
	"template/internal/business/usecases/audit"
	"template/internal/business/usecases/org"
	"template/internal/business/usecases/rbac"
	"template/internal/business/usecases/security"
	"template/internal/business/usecases/users"
	"template/internal/datasources/caches"
	aipostgres "template/internal/datasources/repositories/postgres/ai"
	auditpostgres "template/internal/datasources/repositories/postgres/audit"
	orgpostgres "template/internal/datasources/repositories/postgres/org"
	rbacpostgres "template/internal/datasources/repositories/postgres/rbac"
	securitypostgres "template/internal/datasources/repositories/postgres/security"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	"template/internal/http/middlewares"
	"template/internal/http/routes"
	"template/internal/test/testenv"
	"template/pkg/gemini"
	"template/pkg/jwt"
)

// personas — matrix-ийн мөрүүд. Токенууд server-тэй ижил claim бүтэцтэй.
const (
	adminUserID   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	managerUserID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	plainUserID   = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

// newAuthzServer нь бүрэн бодит угсралттай (route + middleware + usecase +
// repo) HTTP сервер босгоно. Auth-аас бусад бүх зүйл production wiring-тэй
// ижил; auth нь шууд токен үүсгэлтээр орлуулагдсан (login урсгал биш эрхийн
// хаалгыг туршиж байгаа тул).
func newAuthzServer(t *testing.T) (*httptest.Server, jwt.JWTService) {
	t.Helper()
	pool := testenv.StartPostgres(t)
	redisCache := testenv.StartRedis(t)

	jwtSvc := jwt.NewJWTService("integration-test-secret-0123456789abcdef", "authz-matrix-test", 1)
	authMW := middlewares.NewAuthMiddleware(jwtSvc, redisCache, false)

	ristretto, err := caches.NewRistrettoCache()
	require.NoError(t, err)

	usersUC := users.NewUsecase(userspostgres.NewUserRepository(pool), ristretto, users.Config{BcryptCost: 4})
	rbacUC := rbac.NewUsecase(rbacpostgres.NewRBACRepository(pool))
	orgUC := org.NewUsecase(orgpostgres.NewOrgRepository(pool))
	auditUC := audit.NewUsecase(auditpostgres.NewAuditRepository(pool))
	securityUC := security.NewUsecase(securitypostgres.NewSecurityEventRepository(pool))
	aiRepo := aipostgres.NewAIRepository(pool)
	// Gemini рүү гарахгүй — matrix нь зөвхөн DB-д суурилсан admin prompt
	// endpoint-уудыг ашигладаг.
	aiUC := ai.NewUsecase(gemini.NewClient("", "", ""), gemini.NewClient("", "", ""), aiRepo, ai.DefaultTools(), ai.Config{})

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		routes.NewUsersRoute(api, usersUC, authMW).Routes()
		routes.NewRBACRoute(api, rbacUC, auditUC, authMW).Routes()
		routes.NewOrgRoute(api, orgUC, auditUC, authMW).Routes()
		routes.NewAdminRoute(api, usersUC, rbacUC, aiUC, authMW).Routes()
		routes.NewAuditRoute(api, auditUC, authMW).Routes()
		routes.NewSecurityRoute(api, securityUC, authMW).Routes()
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, jwtSvc
}

// anyAllowed нь "эрхийн хаалга нээгдсэн" гэсэн утгатай sentinel — хариу
// 401/403 биш л бол хангагдана (handler 200/404 аль нь ч байж болно).
const anyAllowed = -1

func TestAuthorizationMatrix(t *testing.T) {
	srv, jwtSvc := newAuthzServer(t)

	adminTok, err := jwtSvc.GenerateToken(adminUserID, true, 1, "admin@test.mn")
	require.NoError(t, err)
	managerTok, err := jwtSvc.GenerateToken(managerUserID, false, 3, "manager@test.mn")
	require.NoError(t, err)
	userTok, err := jwtSvc.GenerateToken(plainUserID, false, 2, "user@test.mn")
	require.NoError(t, err)
	// Хуучин (RoleID claim-гүй) токен — user role руу fallback хийх ёстой.
	legacyTok, err := jwtSvc.GenerateToken(plainUserID, false, 0, "legacy@test.mn")
	require.NoError(t, err)

	tokens := map[string]string{
		"anon": "", "user": userTok, "manager": managerTok, "admin": adminTok, "legacy": legacyTok,
	}

	type expectations map[string]int // persona → хүлээгдэх статус (anyAllowed = 401/403 биш)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   expectations
	}{
		{
			// users.manage: admin + manager тийм, user үгүй.
			name: "admin users list (users.manage)", method: http.MethodGet, path: "/api/v1/admin/users",
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 200, "admin": 200},
		},
		{
			// settings.manage: зөвхөн admin (manager-т олгогдоогүй).
			name: "admin AI prompts (settings.manage)", method: http.MethodGet, path: "/api/v1/admin/ai/prompts",
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 403, "admin": 200},
		},
		{
			// roles.manage: зөвхөн admin.
			name: "rbac roles list (roles.manage)", method: http.MethodGet, path: "/api/v1/rbac/roles",
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 403, "admin": 200},
		},
		{
			name: "rbac permissions list (roles.manage)", method: http.MethodGet, path: "/api/v1/rbac/permissions",
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 403, "admin": 200},
		},
		{
			name: "rbac role create (roles.manage)", method: http.MethodPost, path: "/api/v1/rbac/roles",
			body: `{"name":"Matrix Test Role"}`,
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 403, "admin": anyAllowed},
		},
		{
			// Нэвтэрсэн хэн бүхэн өөрийн эрхээ харна.
			name: "rbac my permissions (authed)", method: http.MethodGet, path: "/api/v1/rbac/me",
			want: expectations{"anon": 401, "user": 200, "legacy": 200, "manager": 200, "admin": 200},
		},
		{
			// RequireAdmin: manager users.manage-тэй ч audit хаалттай.
			name: "audit log list (admin-only)", method: http.MethodGet, path: "/api/v1/audit",
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 403, "admin": 200},
		},
		{
			name: "audit chain verify (admin-only)", method: http.MethodGet, path: "/api/v1/audit/verify",
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 403, "admin": 200},
		},
		{
			name: "security events read (admin-only)", method: http.MethodGet, path: "/api/v1/security/events",
			want: expectations{"anon": 401, "user": 403, "legacy": 403, "manager": 403, "admin": 200},
		},
		{
			// Ingest нь нэвтэрсэн хэн бүхэнд нээлттэй (RASP клиент талын мэдээлэл).
			name: "security events ingest (authed)", method: http.MethodPost, path: "/api/v1/security/events",
			body: `{"events":[]}`,
			want: expectations{"anon": 401, "user": anyAllowed, "manager": anyAllowed, "admin": anyAllowed},
		},
		{
			name: "org list mine (authed)", method: http.MethodGet, path: "/api/v1/org",
			want: expectations{"anon": 401, "user": 200, "manager": 200, "admin": 200},
		},
		{
			// Нэвтэрсэн боловч DB-д байхгүй хэрэглэгч — хаалга нээгдэж handler
			// дотоод шийдвэрээ (404) гаргана; 401/403 байж болохгүй.
			name: "users me (authed)", method: http.MethodGet, path: "/api/v1/users/me",
			want: expectations{"anon": 401, "user": anyAllowed, "admin": anyAllowed},
		},
	}

	client := srv.Client()
	for _, tc := range cases {
		for persona, want := range tc.want {
			t.Run(tc.name+"/"+persona, func(t *testing.T) {
				var bodyReader *strings.Reader
				if tc.body != "" {
					bodyReader = strings.NewReader(tc.body)
				} else {
					bodyReader = strings.NewReader("")
				}
				req, err := http.NewRequest(tc.method, srv.URL+tc.path, bodyReader)
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				if tok := tokens[persona]; tok != "" {
					req.Header.Set("Authorization", "Bearer "+tok)
				}
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				if want == anyAllowed {
					require.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, resp.StatusCode,
						"%s %s [%s]: эрхийн хаалга хаагдсан байна (status=%d)", tc.method, tc.path, persona, resp.StatusCode)
					return
				}
				require.Equal(t, want, resp.StatusCode,
					"%s %s [%s]", tc.method, tc.path, persona)
			})
		}
	}
}

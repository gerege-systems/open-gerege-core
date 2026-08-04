// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package assets нь гарын үсэг/тамгын asset (core) модулийн wiring:
// /v1/me/signature, /v1/me/orgstamp г.м. Зураг нь Google Drive-д, URL нь
// DB-д хадгалагдана.
//
// Usecase-ийг sign модуль хэрэглэдэг тул ServiceAssets нэрээр НИЙТЭЛНЭ —
// тиймээс бүртгэлийн дараалалд sign-аас өмнө байна.
package assets

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	assetsuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/assets"
	usersuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	repointerface "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/interface"
	orgstamppostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/orgstamp"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/eid"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — assets модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "assets" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. Файлын
// дугаар нь ГЛОБАЛ дугаарлалтаас хэвээр үлдсэн — нүүлгэлтийн явцад анхны
// дарааллыг мөшгих боломжтой байлгах тул.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("assets: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь asset repo → usecase → /v1/me route угсралтыг гүйцэтгэж,
// usecase-ээ бусад модульд нийтэлнэ.
func (m *Module) Register(_ context.Context, host module.Host) error {
	usersUC, ok := module.ServiceAs[usersuc.Usecase](host, module.ServiceUsers)
	if !ok {
		return fmt.Errorf("assets: host-д %q service алга", module.ServiceUsers)
	}
	userRepo, ok := module.ServiceAs[repointerface.UserRepository](host, module.ServiceUserRepo)
	if !ok {
		return fmt.Errorf("assets: host-д %q service алга", module.ServiceUserRepo)
	}
	eidClient, ok := module.ServiceAs[eid.Client](host, module.ServiceEID)
	if !ok {
		return fmt.Errorf("assets: host-д %q service алга", module.ServiceEID)
	}
	limiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServiceWriteRateLimiter)
	if !ok {
		return fmt.Errorf("assets: host-д %q service алга", module.ServiceWriteRateLimiter)
	}

	stampRepo := orgstamppostgres.NewOrgStampRepository(host.Pool())
	uc := assetsuc.NewUsecase(usersUC, userRepo, stampRepo, eidClient)

	sp, ok := host.(module.ServiceProvider)
	if !ok {
		return fmt.Errorf("assets: host нь ServiceProvider биш — %q нийтлэх боломжгүй", module.ServiceAssets)
	}
	sp.Provide(module.ServiceAssets, uc)

	routes.NewAssetsRoute(host.APIRouter(), uc, host.AuthMiddleware(), limiter).Routes()
	return nil
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package applications нь нэгдсэн Applications (Gateway consumer + SSO RP
// client) business модулийн wiring. oauth_clients хүснэгтийн repo-г өөрөө
// үүсгэнэ (stateless pool wrapper тул provider модультай зэрэг хэрэглэхэд
// давхар instance аюулгүй).
package applications

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	applicationsuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/applications"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	applicationspostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/applications"
	oauthpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/oauth"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — applications модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "applications" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. 22/36 нь анх
// gateway-console-той нэг файлд байсныг хуваасан: application_services нь
// gateway_services рүү FK-тэй тул энэ модуль gateway-console-ийн ДАРАА
// ажиллана (modules/sources.go).
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("applications: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь /v1/applications route-уудаа суулгана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	rbacUC, ok := module.ServiceAs[rbacuc.Usecase](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("applications: host-д %q service алга", module.ServiceRBAC)
	}

	uc := applicationsuc.NewUsecase(
		applicationspostgres.NewApplicationRepository(host.Pool()),
		oauthpostgres.NewClientRepository(host.Pool()),
	)
	routes.NewApplicationsRoute(host.APIRouter(), uc, rbacUC, host.AuthMiddleware()).Routes()
	return nil
}

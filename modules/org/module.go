// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package org нь байгууллага ба гишүүнчлэлийн (core) модулийн wiring:
// repo → usecase → /v1/org route-ууд. Мутаци бүр audit log-д бичигдэнэ.
package org

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	orguc "github.com/gerege-systems/open-gerege-core/core/business/usecases/org"
	orgpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/org"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — org модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "org" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. Файлын
// дугаар нь ГЛОБАЛ дугаарлалтаас хэвээр үлдсэн — нүүлгэлтийн явцад анхны
// дарааллыг мөшгих боломжтой байлгах тул.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("org: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь байгууллагын repo → usecase → route угсралтыг гүйцэтгэнэ.
func (m *Module) Register(_ context.Context, host module.Host) error {
	auditUC, ok := module.ServiceAs[audituc.Usecase](host, module.ServiceAudit)
	if !ok {
		return fmt.Errorf("org: host-д %q service алга", module.ServiceAudit)
	}

	uc := orguc.NewUsecase(orgpostgres.NewOrgRepository(host.Pool()))
	routes.NewOrgRoute(host.APIRouter(), uc, auditUC, host.AuthMiddleware()).Routes()
	return nil
}

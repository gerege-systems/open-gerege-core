// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gov нь "Төрийн үйлчилгээ" порталын business модулийн wiring:
// каталог, хүсэлт, лавлагаа, мэдэгдэл, төлбөр, цаг захиалга + иргэний
// хүсэлтийн SLA sweep worker.
package gov

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/gerege-systems/open-gerege-core/core/business/usecases/gov"
	govpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/gov"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — gov модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "gov" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. Файлын
// дугаар нь ГЛОБАЛ дугаарлалтаас хэвээр үлдсэн — нүүлгэлтийн явцад анхны
// дарааллыг мөшгих боломжтой байлгах тул.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("gov: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь /v1/gov route-ууд + SLA sweep worker-оо суулгана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	rbacResolver, ok := module.ServiceAs[middlewares.PermissionResolver](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("gov: host-д %q service алга", module.ServiceRBAC)
	}
	limiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServiceWriteRateLimiter)
	if !ok {
		return fmt.Errorf("gov: host-д %q service алга", module.ServiceWriteRateLimiter)
	}

	uc := gov.NewUsecase(govpostgres.NewGovRepository(host.Pool()))
	routes.NewGovRoute(host.APIRouter(), uc, rbacResolver, host.AuthMiddleware(), limiter).Routes()

	// Иргэний хүсэлтийн SLA — хугацаа хэтрэлт тэмдэглэх + чимээгүй зөвшөөрөл.
	// Relay-ээс сийрэг (60с): хүний хугацаа цаг/хоногоор хэмжигддэг тул илүү
	// нягт шалгах нь ачааллаас өөр үр дүн авчрахгүй.
	if wr, ok := host.(module.WorkerRegistrar); ok {
		wr.AddWorker("gov-sla-sweep", 60*time.Second, func(c context.Context) { _ = uc.SLASweep(c) })
	}
	return nil
}

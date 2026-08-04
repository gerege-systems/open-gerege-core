// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gatewayconsole нь API gateway-ийн удирдлагын (services/routes/
// consumers/keys/policies + телеметр) business модулийн wiring. Gateway
// usecase ӨӨРӨӨ kernel-ийн түвшинд амьдардаг (хүсэлтийн лог queue + eID
// proxy түүнээс хамаардаг) тул модуль зөвхөн УДИРДЛАГЫН гадаргууг эзэмшинэ —
// ирээдүйн бодит reverse-proxy enforcement ч энэ модульд орно.
package gatewayconsole

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	gatewayuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/gateway"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — gateway-console модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "gateway-console" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. 22/36 нь
// gateway хүснэгтүүдээс гадна applications-ыг ч үүсгэдэг (нэг файлд хоёр
// модулийн хүснэгт) — SQL-ийг хуваахын оронд файлыг нэрээрээ илүү ойр
// gateway-console эзэмшинэ. Эзэмшлийн энэ ойролцоолол нь зөвхөн ФАЙЛД
// хамаарна: манифестийн route эзэмшил өөрчлөгдөөгүй.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("gateway-console: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь /v1/gateway route-уудаа суулгана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	gatewayUC, ok := module.ServiceAs[gatewayuc.Usecase](host, module.ServiceGateway)
	if !ok {
		return fmt.Errorf("gateway-console: host-д %q service алга", module.ServiceGateway)
	}
	rbacUC, ok := module.ServiceAs[rbacuc.Usecase](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("gateway-console: host-д %q service алга", module.ServiceRBAC)
	}
	routes.NewGatewayRoute(host.APIRouter(), gatewayUC, rbacUC, host.AuthMiddleware()).Routes()
	return nil
}

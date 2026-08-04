// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gatewayconsole нь API gateway-ийн удирдлагын (services/routes/
// consumers/keys/policies + телеметр) business модулийн wiring. Gateway
// usecase ӨӨРӨӨ kernel-ийн түвшинд амьдардаг (хүсэлтийн лог queue + eID
// proxy түүнээс хамаардаг) тул модуль зөвхөн УДИРДЛАГЫН гадаргууг эзэмшинэ —
// ирээдүйн бодит reverse-proxy enforcement ч энэ модульд орно.
package gatewayconsole

import (
	"context"
	"fmt"

	gatewayuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/gateway"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — gateway-console модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "gateway-console" }

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

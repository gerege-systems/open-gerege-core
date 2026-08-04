// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package rbac нь эрхийн удирдлагын (core) модулийн wiring: repo → usecase
// → /v1/rbac route-ууд.
//
// RBAC usecase-ийг олон модуль permission шалгалтад хэрэглэдэг тул модуль
// түүнийг ServiceRBAC нэрээр НИЙТЭЛНЭ (Provide). Тиймээс энэ модуль
// хэрэглэгчдээсээ ӨМНӨ бүртгэгдэх ёстой — дарааллыг platformModules()
// тогтооно.
package rbac

import (
	"context"
	"fmt"

	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	rbacpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/rbac"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — rbac модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "rbac" }

// Register нь RBAC repo → usecase → route угсралтыг гүйцэтгэж, usecase-ээ
// бусад модульд нийтэлнэ.
func (m *Module) Register(_ context.Context, host module.Host) error {
	auditUC, ok := module.ServiceAs[audituc.Usecase](host, module.ServiceAudit)
	if !ok {
		return fmt.Errorf("rbac: host-д %q service алга", module.ServiceAudit)
	}

	uc := rbacuc.NewUsecase(rbacpostgres.NewRBACRepository(host.Pool()))

	sp, ok := host.(module.ServiceProvider)
	if !ok {
		return fmt.Errorf("rbac: host нь ServiceProvider биш — %q нийтлэх боломжгүй", module.ServiceRBAC)
	}
	sp.Provide(module.ServiceRBAC, uc)

	routes.NewRBACRoute(host.APIRouter(), uc, auditUC, host.AuthMiddleware()).Routes()
	return nil
}

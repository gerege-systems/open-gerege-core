// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package registry нь үйлчилгээний нэгдсэн регистрийн (CPSV-AP паспорт,
// нотолгооны каталог, life-events, once-only) business модулийн wiring —
// админ бүртгэл (/v1/registry) + нийтийн каталог (/v1/catalog) хоёул.
package registry

import (
	"context"
	"fmt"

	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/registry"
	registrypostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/registry"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — registry модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "registry" }

// Register нь /v1/registry + /v1/catalog route-уудаа суулгана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	rbacUC, ok := module.ServiceAs[rbacuc.Usecase](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("registry: host-д %q service алга", module.ServiceRBAC)
	}

	// Мастер өгөгдөл тул RLS-гүй; хамгаалалт нь registry.view/manage эрхээр.
	uc := registry.NewUsecase(registrypostgres.NewRegistryRepository(host.Pool()))
	routes.NewRegistryRoute(host.APIRouter(), uc, rbacUC, host.AuthMiddleware()).Routes()
	routes.NewCatalogRoute(host.APIRouter(), uc, host.AuthMiddleware()).Routes()
	return nil
}

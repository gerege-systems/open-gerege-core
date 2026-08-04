// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package corefind нь Gerege Core (core.gerege.mn) USER FIND / ORG FIND
// лавлагааны wrap business модулийн wiring. Модулийн ID нь "core-find".
package corefind

import (
	"context"
	"fmt"

	"github.com/gerege-systems/open-gerege-core/core/business/usecases/core"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — core-find модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "core-find" }

// Register нь /v1/core route-уудыг суулгана. RBAC usecase-ийг host-ын
// хуваалцсан service-ээс авна (permission шалгалтад).
func (m *Module) Register(_ context.Context, host module.Host) error {
	rbacUC, ok := module.ServiceAs[rbacuc.Usecase](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("core-find: host-д %q service алга", module.ServiceRBAC)
	}
	uc := core.NewUsecase(config.AppConfig.CoreAPIBase, config.AppConfig.CoreAPIToken)
	routes.NewCoreRoute(host.APIRouter(), uc, rbacUC, host.AuthMiddleware()).Routes()
	return nil
}

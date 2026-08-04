// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package org нь байгууллага ба гишүүнчлэлийн (core) модулийн wiring:
// repo → usecase → /v1/org route-ууд. Мутаци бүр audit log-д бичигдэнэ.
package org

import (
	"context"
	"fmt"

	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	orguc "github.com/gerege-systems/open-gerege-core/core/business/usecases/org"
	orgpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/org"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — org модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "org" }

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

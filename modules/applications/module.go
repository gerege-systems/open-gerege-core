// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package applications нь нэгдсэн Applications (Gateway consumer + SSO RP
// client) business модулийн wiring. oauth_clients хүснэгтийн repo-г өөрөө
// үүсгэнэ (stateless pool wrapper тул provider модультай зэрэг хэрэглэхэд
// давхар instance аюулгүй).
package applications

import (
	"context"
	"fmt"

	applicationsuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/applications"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	applicationspostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/applications"
	oauthpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/oauth"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — applications модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "applications" }

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

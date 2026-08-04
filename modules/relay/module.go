// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package relay нь платформ-хоорондын хүсэлт дамжуулалт + SLA хяналтын
// business модулийн wiring (route + sweep/demo worker-ууд).
package relay

import (
	"context"
	"fmt"
	"time"

	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	relayuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/relay"
	"github.com/gerege-systems/open-gerege-core/core/config"
	relaypostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/relay"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — relay модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "relay" }

// Register нь /v1/relay route-ууд + SLA sweep (мөн RELAY_DEMO_MODE-д
// simulator) worker-уудаа суулгана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	rbacUC, ok := module.ServiceAs[rbacuc.Usecase](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("relay: host-д %q service алга", module.ServiceRBAC)
	}

	uc := relayuc.NewUsecase(relaypostgres.NewRelayRepository(host.Pool()))
	routes.NewRelayRoute(host.APIRouter(), uc, rbacUC, host.AuthMiddleware()).Routes()

	if wr, ok := host.(module.WorkerRegistrar); ok {
		wr.AddWorker("relay-sla-sweep", 20*time.Second, func(c context.Context) { _ = uc.SLASweep(c) })
		if config.AppConfig.RelayDemoMode {
			// Демо горим: доод platform-уудын хариу дуурайх + шинэ демо хүсэлт үүсгэх.
			wr.AddWorker("relay-demo-step", 10*time.Second, uc.SimulateStep)
			wr.AddWorker("relay-demo-ingest", 25*time.Second, uc.SimulateIngest)
		}
	}
	return nil
}

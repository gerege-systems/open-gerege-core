// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package platform нь модулийн удирдлагын (core) модулийн wiring:
// нийтийн жагсаалт + админы асаах/унтраах гадаргуу (/v1/platform).
//
// Бүртгэл (*module.Registry) болон төлвийн store нь kernel-ийн ӨМЧ — boot
// дээр gate суухаас өмнө сэргээгддэг тул server.go үүсгэж, энд Host
// service-ээр ирнэ. Модуль зөвхөн usecase + route-оо угсарна.
package platform

import (
	"context"
	"fmt"

	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	platformuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/platform"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — platform модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "platform" }

// Register нь модулийн удирдлагын usecase + /v1/platform route-уудыг суулгана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	reg, ok := module.ServiceAs[*module.Registry](host, module.ServiceModuleRegistry)
	if !ok {
		return fmt.Errorf("platform: host-д %q service алга", module.ServiceModuleRegistry)
	}
	store, ok := module.ServiceAs[platformuc.Store](host, module.ServiceModuleStore)
	if !ok {
		return fmt.Errorf("platform: host-д %q service алга", module.ServiceModuleStore)
	}
	auditUC, ok := module.ServiceAs[audituc.Usecase](host, module.ServiceAudit)
	if !ok {
		return fmt.Errorf("platform: host-д %q service алга", module.ServiceAudit)
	}

	uc := platformuc.NewUsecase(reg, store, auditUC)
	routes.NewPlatformRoute(host.APIRouter(), uc, host.AuthMiddleware()).Routes()
	return nil
}

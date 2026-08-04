// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package audit нь audit ба аюулгүй байдлын үйл явдлын (core) модулийн
// wiring: /v1/audit (hash-гинжтэй, зөвхөн нэмэгддэг лог) + /v1/security
// (RASP маягийн үйл явдлын ingest).
//
// Audit usecase-ийг бараг бүх мутацитай модуль хэрэглэдэг тул модуль
// түүнийг ServiceAudit нэрээр НИЙТЭЛНЭ. Энэ нь бүртгэлийн дарааллын хамгийн
// эхэнд байх ёстой модуль — platformModules() түүнийг эхэнд жагсаана.
package audit

import (
	"context"
	"fmt"

	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	securityuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/security"
	auditpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/audit"
	securitypostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/security"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — audit модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "audit" }

// Register нь audit + security repo → usecase → route угсралтыг гүйцэтгэж,
// audit usecase-ээ бусад модульд нийтэлнэ.
func (m *Module) Register(_ context.Context, host module.Host) error {
	pool := host.Pool()
	authMW := host.AuthMiddleware()

	// audit_log нь admin-only тул repository нь хүсэлтийн RLS-аас үл хамааран
	// транзакц дотроо service/admin GUC тогтоодог.
	uc := audituc.NewUsecase(auditpostgres.NewAuditRepository(pool))

	sp, ok := host.(module.ServiceProvider)
	if !ok {
		return fmt.Errorf("audit: host нь ServiceProvider биш — %q нийтлэх боломжгүй", module.ServiceAudit)
	}
	sp.Provide(module.ServiceAudit, uc)

	// Security events — нэвтэрсэн хэрэглэгч бичнэ, admin унших.
	securityUC := securityuc.NewUsecase(securitypostgres.NewSecurityEventRepository(pool))

	routes.NewAuditRoute(host.APIRouter(), uc, authMW).Routes()
	routes.NewSecurityRoute(host.APIRouter(), securityUC, authMW).Routes()
	return nil
}

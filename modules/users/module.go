// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package users нь хэрэглэгч ба eID профайлын (core) модулийн wiring:
// repo → usecase → /v1/users, /v1/admin/users*.
//
// usersUC нь платформын хамгийн олон хэрэглэгчтэй service-үүдийн нэг
// (auth, assets, superadmin, OIDC token issuing) тул ServiceUsers болон
// ServiceUserRepo хоёуланг НИЙТЭЛНЭ — бүртгэлийн дараалалд эрт байна.
//
// /v1/users/me/eid профайлын route нь ЭНД БИШ, auth модульд бүртгэгддэг:
// тэр нь authUC-ээс хамаардаг ба authUC нь эргээд usersUC-ээс хамаардаг тул
// нэг Register дуудалтад багтахгүй мөчлөг үүснэ. Gate нь ЗАМААР ажилладаг
// (бүртгэгчээр биш) тул users модулийг унтраахад тэр route мөн 404 болно —
// манифестийн эзэмшил хэвээр.
package users

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	usersuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/constants"
	"github.com/gerege-systems/open-gerege-core/core/datasources/caches"
	repointerface "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/interface"
	userspostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/users"
	"github.com/gerege-systems/open-gerege-core/core/datasources/rls"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — users модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "users" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. Файлын
// дугаар нь ГЛОБАЛ дугаарлалтаас хэвээр үлдсэн — нүүлгэлтийн явцад анхны
// дарааллыг мөшгих боломжтой байлгах тул.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("users: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь хэрэглэгчийн repo → usecase → route угсралтыг гүйцэтгэж,
// usecase/repo-гоо нийтэлж, SUPERADMIN_EMAIL ахиулалтыг гүйцэтгэнэ.
func (m *Module) Register(ctx context.Context, host module.Host) error {
	rbacUC, ok := module.ServiceAs[rbacuc.Usecase](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("users: host-д %q service алга", module.ServiceRBAC)
	}
	sp, ok := host.(module.ServiceProvider)
	if !ok {
		return fmt.Errorf("users: host нь ServiceProvider биш — %q нийтлэх боломжгүй", module.ServiceUsers)
	}

	// Хэрэглэгчийн кэш нь зөвхөн энэ модулийн өмч (ristretto, process-local).
	ristretto, err := caches.NewRistrettoCache()
	if err != nil {
		return fmt.Errorf("users: ristretto cache: %w", err)
	}

	userRepo := userspostgres.NewUserRepository(host.Pool())
	uc := usersuc.NewUsecase(userRepo, ristretto, usersuc.Config{
		BcryptCost: config.AppConfig.BcryptCost,
	})

	sp.Provide(module.ServiceUsers, uc)
	sp.Provide(module.ServiceUserRepo, userRepo)

	// Bootstrap: SUPERADMIN_EMAIL тохируулсан бол тухайн хэрэглэгчийг super
	// admin болгож ахиулна (best-effort; байхгүй бол warning).
	bootstrapSuperAdmin(ctx, userRepo, config.AppConfig.SuperAdminEmail)

	// SSO eID proxy идэвхтэй эсэх нь профайлын гадаргууг өөрчилдөг. Тухайн
	// нөхцөл нь auth модулийнхтой ижил config-оос тооцоологдоно.
	ssoEidProxyEnabled := config.AppConfig.SSOEidProxyBaseURL != "" && config.AppConfig.IntegrationEncKey != ""

	routes.NewUsersRoute(host.APIRouter(), uc, host.AuthMiddleware(), ssoEidProxyEnabled).Routes()
	routes.NewAdminRoute(host.APIRouter(), uc, rbacUC, host.AuthMiddleware()).Routes()
	return nil
}

// bootstrapSuperAdmin нь SUPERADMIN_EMAIL-ээр заасан хэрэглэгчийг super
// admin болгоно. Best-effort: хэрэглэгч байхгүй бол зөвхөн warning (эхлээд
// бүртгүүлж, дараа нь дахин эхлүүлнэ).
func bootstrapSuperAdmin(ctx context.Context, repo repointerface.UserRepository, email string) {
	email = domain.NormalizeEmail(email)
	if email == "" {
		return
	}
	log := logger.WithFields(logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
	sctx := rls.WithService(ctx)
	existing, err := repo.GetByEmail(sctx, &domain.User{Email: email})
	if err != nil {
		log.Warnf("SUPERADMIN_EMAIL (%s) ахиулалт алгаслаа: хэрэглэгч олдсонгүй эсвэл хайлт амжилтгүй (эхлээд бүртгүүлж, дараа нь дахин эхлүүлнэ үү): %v", email, err)
		return
	}
	if existing.RoleID == domain.RoleSuperAdmin {
		return // аль хэдийн super admin — no-op
	}
	if err := repo.UpdateRole(sctx, existing.ID, domain.RoleSuperAdmin); err != nil {
		log.Warnf("SUPERADMIN_EMAIL (%s) ахиулалт амжилтгүй: %v", email, err)
		return
	}
	log.Infof("SUPERADMIN_EMAIL (%s) super admin болголоо (role_id=%d)", email, domain.RoleSuperAdmin)
}

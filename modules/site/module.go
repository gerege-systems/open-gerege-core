// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package site нь сайтын харагдац · theme · хэлний (core) модулийн wiring:
// /v1/site, /v1/themes, /v1/languages. Гурвуулаа нийтийн config тул
// RLS-гүй plain pool дээр ажиллана; бичих эрх нь route давхаргад
// 'settings.manage' permission-оор хамгаалагдана.
package site

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	languageuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/language"
	rbacuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	siteuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/site"
	themeuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/theme"
	"github.com/gerege-systems/open-gerege-core/core/config"
	languagepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/language"
	sitepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/site"
	themepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/theme"
	sitehandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/site"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/gemini"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — site модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "site" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. Файлын
// дугаар нь ГЛОБАЛ дугаарлалтаас хэвээр үлдсэн — нүүлгэлтийн явцад анхны
// дарааллыг мөшгих боломжтой байлгах тул.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("site: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь site/theme/language repo → usecase → route угсралтыг гүйцэтгэнэ.
func (m *Module) Register(_ context.Context, host module.Host) error {
	rbacUC, ok := module.ServiceAs[rbacuc.Usecase](host, module.ServiceRBAC)
	if !ok {
		return fmt.Errorf("site: host-д %q service алга", module.ServiceRBAC)
	}
	// Хэлний модуль орчуулгад Gemini-г хэрэглэнэ (ai модулиас хамаарахгүй —
	// client нь kernel түвшний хуваалцсан нөөц).
	geminiClient, ok := module.ServiceAs[*gemini.Client](host, module.ServiceGeminiChat)
	if !ok {
		return fmt.Errorf("site: host-д %q service алга", module.ServiceGeminiChat)
	}

	pool := host.Pool()
	authMW := host.AuthMiddleware()

	siteUC := siteuc.NewUsecase(sitepostgres.NewSiteRepository(pool))
	themeUC := themeuc.NewUsecase(themepostgres.NewThemeRepository(pool))
	languageUC := languageuc.NewUsecase(
		languagepostgres.NewLanguageRepository(pool),
		languageuc.NewGeminiTranslator(geminiClient),
	)

	// Нэвтрэх гадаргууны горимыг config-оос нэг удаа уншиж handler руу өгнө
	// (handler давхарга config-оос хамаардаггүй). client горимд л дээд
	// IdP-ийн issuer-ыг мэдэгдэнэ — provider горимд утга учиргүй.
	authSurface := sitehandler.AuthSurface{
		Mode:     config.AppConfig.LoginMode(),
		Provider: config.AppConfig.ProviderConfigured(),
	}
	if authSurface.Mode == config.AuthModeClient {
		authSurface.SSOIssuer = config.AppConfig.SSOIssuer
	}

	routes.NewSiteRoute(host.APIRouter(), siteUC, rbacUC, authMW, authSurface).Routes()
	routes.NewThemeRoute(host.APIRouter(), themeUC, rbacUC, authMW).Routes()
	routes.NewLanguageRoute(host.APIRouter(), languageUC, authMW).Routes()
	return nil
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package integrations нь хэрэглэгчийн гуравдагч этгээдийн интеграцийн
// (Google Drive/Meet, Dropbox) business модулийн wiring.
package integrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/gerege-systems/open-gerege-core/core/business/usecases/integrations"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/constants"
	userintegrationspostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/userintegrations"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — integrations модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "integrations" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. Файлын
// дугаар нь ГЛОБАЛ дугаарлалтаас хэвээр үлдсэн — нүүлгэлтийн явцад анхны
// дарааллыг мөшгих боломжтой байлгах тул.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("integrations: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь шифрлэлттэй токен хадгалалтын repo + usecase-ээ угсарч
// /v1/integrations route-уудыг суулгана. Enc key-ийн алдаа boot-ыг унагаана
// (production-д fail-closed — өнөөдрийн server.go-той ижил зан төлөв).
func (m *Module) Register(_ context.Context, host module.Host) error {
	isProduction := config.AppConfig.Environment == constants.EnvironmentProduction
	repo := userintegrationspostgres.NewUserIntegrationsRepository(host.Pool())
	uc, err := integrations.NewUsecase(repo, config.AppConfig.IntegrationEncKey, isProduction)
	if err != nil {
		return fmt.Errorf("init integrations usecase: %w", err)
	}
	routes.NewIntegrationsRoute(host.APIRouter(), uc, host.AuthMiddleware()).Routes()
	return nil
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package provider нь OIDC provider-ийн login/consent/logout урсгалын
// business модулийн wiring (/v1/provider). OIDC цөм (Service, түлхүүр,
// нийтийн /.well-known /oauth2 endpoint-ууд, /admin оператор гадаргуу) нь
// kernel түвшинд үлддэг — тэдгээр нь root router + boot-ийн fail-closed
// түлхүүр менежменттэй уялдсан; энэ модуль хэрэглэгчийн УРСГАЛЫГ эзэмшинэ.
package provider

import (
	"context"
	"fmt"

	oidcuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/oidc"
	provideruc "github.com/gerege-systems/open-gerege-core/core/business/usecases/provider"
	"github.com/gerege-systems/open-gerege-core/core/config"
	oauthpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/oauth"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — provider модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "provider" }

// Register нь provider usecase-ээ угсарч /v1/provider route-уудаа суулгана.
// oauth_clients repo нь stateless pool wrapper тул өөрийн instance аюулгүй.
func (m *Module) Register(_ context.Context, host module.Host) error {
	oidcSvc, ok := module.ServiceAs[*oidcuc.Service](host, module.ServiceOIDCService)
	if !ok {
		return fmt.Errorf("provider: host-д %q service алга", module.ServiceOIDCService)
	}
	usersUC, ok := module.ServiceAs[provideruc.UserLookup](host, module.ServiceUsers)
	if !ok {
		return fmt.Errorf("provider: host-д %q service алга", module.ServiceUsers)
	}

	uc := provideruc.NewUsecase(
		oidcSvc,
		oauthpostgres.NewClientRepository(host.Pool()),
		usersUC,
		config.AppConfig.SSOFirstPartyClientsList(),
		config.AppConfig.Issuer(),
	)
	routes.NewProviderRoute(host.APIRouter(), uc, host.AuthMiddleware()).Routes()
	return nil
}

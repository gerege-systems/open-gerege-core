// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package eidproxy нь eID service proxy business модулийн wiring: бүртгэгдсэн
// апп (relying party)-ууд хэрэглэгчийнхээ access token-оор платформын eID
// service-үүдийг ДАМЖУУЛАН дуудна (/v1/eid/*, /v1/eid-org/*). Платформ
// өөрийн eidmongolia RP creds-ээр татаж өгнө тул апп-д eID credential
// эзэмших шаардлагагүй.
//
// /v1/eid-auth/* нь мөн энэ модульд харьяалагдана — гэхдээ тэр нь иргэнийг
// ШИНЭЭР танихад (нэвтрэлт эхлүүлэх + poll) зориулагдсан тул иргэний биш,
// АППЫН токеноор ажиллана.
package eidproxy

import (
	"context"
	"fmt"
	"net/http"

	authuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/auth"
	eidauthuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/eidauth"
	gatewayuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/gateway"
	oidcuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/oidc"
	"github.com/gerege-systems/open-gerege-core/core/config"
	oauthpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/oauth"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/eid"
)

// Module — eidproxy модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "eidproxy" }

// Register нь OAuth bearer middleware-уудаа угсарч /v1/eid, /v1/eid-org
// route-уудаа суулгана. Service тус бүрийн зөвшөөрөл нь client-ийн allowed
// scope дахь "svc:<service>"-ээр илэрхийлэгдэнэ (олгогдоогүй апп 403);
// gateway catalog-д идэвхгүй бол route-ууд өөрсдөө хаагдана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	authUC, ok := module.ServiceAs[authuc.Usecase](host, module.ServiceAuth)
	if !ok {
		return fmt.Errorf("eidproxy: host-д %q service алга", module.ServiceAuth)
	}
	gatewayUC, ok := module.ServiceAs[gatewayuc.Usecase](host, module.ServiceGateway)
	if !ok {
		return fmt.Errorf("eidproxy: host-д %q service алга", module.ServiceGateway)
	}
	oidcSvc, ok := module.ServiceAs[*oidcuc.Service](host, module.ServiceOIDCService)
	if !ok {
		return fmt.Errorf("eidproxy: host-д %q service алга", module.ServiceOIDCService)
	}

	// oauth_clients repo — stateless pool wrapper тул өөрийн instance аюулгүй.
	oauthClients := oauthpostgres.NewClientRepository(host.Pool())
	eidProxyMW := middlewares.NewOAuthBearerMiddleware(oidcSvc, oauthClients, "svc:"+routes.EIDProxyServiceName)
	eidOrgProxyMW := middlewares.NewOAuthBearerMiddleware(oidcSvc, oauthClients, "svc:"+routes.EIDOrgProxyServiceName)

	routes.NewEIDProxyRoute(host.APIRouter(), authUC, gatewayUC, eidProxyMW, eidOrgProxyMW).Routes()

	// eID НЭВТРЭЛТИЙН proxy (/v1/eid-auth/*) — иргэн энэ үед хараахан
	// танигдаагүй тул АППЫН токеноор (client_credentials) ажиллана.
	eidClient, ok := module.ServiceAs[eid.Client](host, module.ServiceEID)
	if !ok {
		return fmt.Errorf("eidproxy: host-д %q service алга", module.ServiceEID)
	}
	eidAuthUC := eidauthuc.NewUsecase(eidClient, eidauthuc.Config{
		DisplayText: config.AppConfig.EIDDisplayText,
	})
	eidAuthMW := middlewares.NewOAuthAppMiddleware(oidcSvc, oauthClients, "svc:"+routes.EIDAuthServiceName)
	// poll нь long-poll тул нэвтрэлтийн урсгалын хязгаарлагчийг хуваалцана.
	var pollMW func(http.Handler) http.Handler
	if pollLimiter, lok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServicePollRateLimiter); lok {
		pollMW = pollLimiter.Middleware()
	}
	routes.NewEIDAuthRoute(host.APIRouter(), eidAuthUC, gatewayUC, eidAuthMW, pollMW).Routes()
	return nil
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package auth нь танилтын (core) модулийн wiring: eID · Google · SSO
// нэвтрэлт (/v1/auth, /v1/sso) + иргэний PKI профайл (/v1/users/me/eid).
//
// ЯАГААД /v1/users/me/eid ЭНД: тэр route нь authUC-ээс хамаардаг ба authUC
// нь usersUC-ээс хамаардаг. Хэрэв users модуль түүнийг бүртгэвэл users →
// auth → users гэсэн мөчлөг үүсэх бөгөөд нэг Register дуудалтад шийдэгдэхгүй.
// Gate нь ЗАМААР ажилладаг тул эзэмшил users модульд хэвээр: users-ийг
// унтраахад энэ route мөн 404 болно.
package auth

import (
	"context"
	"fmt"
	"time"

	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	authuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/auth"
	ssouc "github.com/gerege-systems/open-gerege-core/core/business/usecases/sso"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/ssotoken"
	usersuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/datasources/caches"
	platformsettings "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/platformsettings"
	ssotokenpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/ssotoken"
	ssouserpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/ssouser"
	authhandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/auth"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/crypto"
	"github.com/gerege-systems/open-gerege-core/pkg/eid"
	"github.com/gerege-systems/open-gerege-core/pkg/google"
	"github.com/gerege-systems/open-gerege-core/pkg/jwt"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
	"github.com/gerege-systems/open-gerege-core/pkg/oidc"
	"github.com/gerege-systems/open-gerege-core/pkg/ssoeidproxy"
	"github.com/gerege-systems/open-gerege-core/pkg/verify"
	"github.com/gerege-systems/open-gerege-core/pkg/xyp"
)

// Module — auth модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "auth" }

// Register нь танилтын usecase-уудыг угсарч route-уудаа суулгаад, authUC-ээ
// нийтэлнэ (eidproxy, provider модулиуд хэрэглэнэ).
func (m *Module) Register(_ context.Context, host module.Host) error {
	usersUC, ok := module.ServiceAs[usersuc.Usecase](host, module.ServiceUsers)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceUsers)
	}
	auditUC, ok := module.ServiceAs[audituc.Usecase](host, module.ServiceAudit)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceAudit)
	}
	jwtService, ok := module.ServiceAs[jwt.JWTService](host, module.ServiceJWT)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceJWT)
	}
	verifier, ok := module.ServiceAs[*verify.Client](host, module.ServiceVerifier)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceVerifier)
	}
	eidClient, ok := module.ServiceAs[eid.Client](host, module.ServiceEID)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceEID)
	}
	googleClient, ok := module.ServiceAs[*google.Client](host, module.ServiceGoogle)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceGoogle)
	}
	redisCache, ok := module.ServiceAs[caches.RedisCache](host, module.ServiceRedis)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceRedis)
	}
	authLimiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServiceAuthRateLimiter)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceAuthRateLimiter)
	}
	pollLimiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServicePollRateLimiter)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServicePollRateLimiter)
	}
	writeLimiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServiceWriteRateLimiter)
	if !ok {
		return fmt.Errorf("auth: host-д %q service алга", module.ServiceWriteRateLimiter)
	}
	// СОНГОЛТТОЙ: түрийвчгүй платформд nil (алдаа биш).
	walletProvisioner, _ := module.ServiceAs[authhandler.WalletProvisioner](host, module.ServiceWalletProvisioner)

	pool := host.Pool()

	// Gerege Verify / XYP — улсын бүртгэлээс байгууллагын мэдээлэл
	// (eID байгууллага холбох). Зөвхөн энэ модуль хэрэглэнэ.
	xypClient := xyp.NewClient(config.AppConfig.XYPAPIBase, config.AppConfig.XYPClientID, config.AppConfig.XYPClientSecret)

	// Gerege SSO (sso.gerege.mn, OIDC) client — RP нэвтрэлт + eID proxy.
	ssoClient := oidc.NewClient(config.AppConfig.SSOIssuer, config.AppConfig.SSOClientID, config.AppConfig.SSOClientSecret, config.AppConfig.SSORedirectURI, config.AppConfig.SSOScope)

	// SSO eID proxy (сонголттой) — SSO_EID_PROXY_BASE_URL + INTEGRATION_ENC_KEY
	// хоёулаа тохируулсан бол иргэний PKI самбар (summary/certificates/devices/
	// activity) нь шууд eidmongolia-ий оронд sso.gerege.mn/rp/eid-ээр дамжина.
	// Токенуудыг шифрлэн (sso_tokens) хадгалж, хугацаа дуусахад refresh хийнэ.
	// Хоосон бол шууд eidmongolia зам (өөрчлөлтгүй).
	var (
		ssoEidProxy    authuc.SSOEidProxy
		ssoTokens      authuc.SSOTokenService
		ssoTokenStorer ssouc.TokenStorer
	)
	if config.AppConfig.SSOEidProxyBaseURL != "" && config.AppConfig.IntegrationEncKey != "" {
		tokenCipher, cErr := crypto.New(config.AppConfig.IntegrationEncKey)
		if cErr != nil {
			return fmt.Errorf("auth: init sso token cipher: %w", cErr)
		}
		tokenSvc := ssotoken.New(ssotokenpostgres.NewSSOTokenRepository(pool, tokenCipher), ssoClient)
		ssoTokens = tokenSvc
		ssoTokenStorer = tokenSvc
		ssoEidProxy = ssoeidproxy.New(config.AppConfig.SSOEidProxyBaseURL)
		logger.Info("SSO eID proxy enabled — PKI dashboard reads proxied via SSO", logger.Fields{"base": config.AppConfig.SSOEidProxyBaseURL})
	}

	uc := authuc.NewUsecase(usersUC, jwtService, verifier, eidClient, xypClient, googleClient, redisCache, authuc.Config{
		OTPMaxAttempts:    config.AppConfig.OTPMaxAttempts,
		OTPTTL:            time.Duration(config.AppConfig.REDISExpired) * time.Minute,
		PasswordResetTTL:  30 * time.Minute,
		BcryptCost:        config.AppConfig.BcryptCost,
		LoginMaxAttempts:  10,
		LoginLockoutTTL:   15 * time.Minute,
		ForgotMaxAttempts: 3,
		ForgotLockoutTTL:  15 * time.Minute,
		EIDCallbackURL:    config.AppConfig.EIDCallbackURL,
		EIDDisplayText:    config.AppConfig.EIDDisplayText,
		SSOEidProxy:       ssoEidProxy,
		SSOTokens:         ssoTokens,
	})

	sp, ok := host.(module.ServiceProvider)
	if !ok {
		return fmt.Errorf("auth: host нь ServiceProvider биш — %q нийтлэх боломжгүй", module.ServiceAuth)
	}
	sp.Provide(module.ServiceAuth, uc)

	// Gerege SSO RP урсгал — нэвтрэлтийг sso.gerege.mn руу даатгаж, буцаж
	// ирсэн code-ийг токен болгон солин, хэрэглэгчийг sso_sub-ээр upsert хийнэ.
	// platformSettings нь хандалтын горим (public|private) gate-д хэрэгтэй.
	ssoUC := ssouc.NewUsecase(
		ssoClient,
		ssouserpostgres.NewSSOUserRepository(pool),
		jwtService, redisCache,
		config.AppConfig.SSONativeClientID,
		ssoTokenStorer,
		platformsettings.NewRepository(pool),
	)

	api := host.APIRouter()
	authMW := host.AuthMiddleware()
	routes.NewAuthRoute(api, uc, auditUC, walletProvisioner, authMW, authLimiter, pollLimiter).Routes()
	routes.NewSSORoute(api, ssoUC).Routes()
	routes.NewEIDProfileRoute(api, uc, authMW, writeLimiter).Routes()
	return nil
}

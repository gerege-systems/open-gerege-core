// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/ai"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/assets"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/auth"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/gateway"
	languageuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/language"
	oidcuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/oidc"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/org"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/rbac"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/security"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/sign"
	siteuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/site"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/sso"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/ssotoken"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/superadmin"
	onboarding "github.com/gerege-systems/open-gerege-core/core/business/usecases/superadmin_onboarding"
	themeuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/theme"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/constants"
	"github.com/gerege-systems/open-gerege-core/core/datasources/caches"
	"github.com/gerege-systems/open-gerege-core/core/datasources/drivers"
	repointerface "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/interface"
	auditpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/audit"
	gatewaypostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/gateway"
	languagepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/language"
	oauthpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/oauth"
	orgpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/org"
	orgstamppostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/orgstamp"
	platformsettings "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/platformsettings"
	rbacpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/rbac"
	recoverypostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/recovery"
	securitypostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/security"
	sitepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/site"
	ssotokenpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/ssotoken"
	ssouserpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/ssouser"
	superadminaccountpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/superadminaccount"
	superadmininvitepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/superadmininvite"
	themepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/theme"
	userspostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/users"
	"github.com/gerege-systems/open-gerege-core/core/datasources/rls"
	V1Handler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	authhandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/auth"
	sitehandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/site"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/core/provider/adminapi"
	"github.com/gerege-systems/open-gerege-core/core/provider/adminkeys"
	"github.com/gerege-systems/open-gerege-core/core/provider/devapps"
	"github.com/gerege-systems/open-gerege-core/core/provider/signrelay"
	docs "github.com/gerege-systems/open-gerege-core/docs" // swagger тодорхойлолт, swaggo-оор init үед бүртгэгддэг
	"github.com/gerege-systems/open-gerege-core/pkg/crypto"
	"github.com/gerege-systems/open-gerege-core/pkg/eid"
	"github.com/gerege-systems/open-gerege-core/pkg/gemini"
	"github.com/gerege-systems/open-gerege-core/pkg/google"
	"github.com/gerege-systems/open-gerege-core/pkg/jwt"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
	"github.com/gerege-systems/open-gerege-core/pkg/observability"
	"github.com/gerege-systems/open-gerege-core/pkg/oidc"
	"github.com/gerege-systems/open-gerege-core/pkg/ssoeidproxy"
	"github.com/gerege-systems/open-gerege-core/pkg/verify"
	"github.com/gerege-systems/open-gerege-core/pkg/xyp"

	platformmodulespostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/platformmodules"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	aimod "github.com/gerege-systems/open-gerege-core/modules/ai"
	applicationsmod "github.com/gerege-systems/open-gerege-core/modules/applications"
	corefindmod "github.com/gerege-systems/open-gerege-core/modules/corefind"
	eidproxymod "github.com/gerege-systems/open-gerege-core/modules/eidproxy"
	gatewayconsolemod "github.com/gerege-systems/open-gerege-core/modules/gatewayconsole"
	govmod "github.com/gerege-systems/open-gerege-core/modules/gov"
	gspacemod "github.com/gerege-systems/open-gerege-core/modules/gspace"
	integrationsmod "github.com/gerege-systems/open-gerege-core/modules/integrations"
	platformmod "github.com/gerege-systems/open-gerege-core/modules/platform"
	providermod "github.com/gerege-systems/open-gerege-core/modules/provider"
	registrymod "github.com/gerege-systems/open-gerege-core/modules/registry"
	relaymod "github.com/gerege-systems/open-gerege-core/modules/relay"
	signmod "github.com/gerege-systems/open-gerege-core/modules/sign"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ServiceName нь telemetry (tracing/metrics)-д ашиглагдах үйлчилгээний нэр.
// Апп өөрийн нэрээр дарж бичиж болно — NewApp дуудахаас өмнө тавина.
var ServiceName = "gerege-platform"

// WalletProvisioner нь гар утасны нэвтрэлт амжилттай болоход иргэний
// түрийвчийг нээх/олох СОНГОЛТТОЙ дэгээ. Түрийвчтэй апп (жишээ нь
// wallet-gerege-mn) үүнийг NewApp дуудахаас ӨМНӨ тавина:
//
//	server.WalletProvisioner = walletAdapter{uc}
//
// nil үлдвэл (суурь платформ) нэвтрэлт хэвийн ажиллах ч хариунд IBAN ирэхгүй.
var WalletProvisioner authhandler.WalletProvisioner

// bootstrapOnce нь Bootstrap-ыг зөвхөн нэг удаа гүйцэтгэнэ — нимгэн апп нь
// core-ийн main-ыг давхарлан дуудсан ч аюулгүй.
var (
	bootstrapOnce sync.Once
	bootstrapErr  error
)

// Bootstrap нь тохиргоо болон logger-ийг ачаална.
//
// NewApp үүнийг ӨӨРӨӨ дууддаг тул нимгэн апп мартах боломжгүй. (Өмнө нь
// энэ нь cmd/api/main.go-ийн init()-д байсан бөгөөд суурийг модуль болгож
// гаргахад аппууд үүнийг санамсаргүй орхивол config зөвхөн тэг утгатай
// үлдэж, API эхлэхгүй байв.)
func Bootstrap() error {
	bootstrapOnce.Do(func() {
		if err := config.InitializeAppConfig(); err != nil {
			bootstrapErr = err
			return
		}
		// Орчноос гарган авсан тохиргоогоор logger-ийг дахин эхлүүлнэ
		// (production = JSON; dev = console).
		_ = logger.InitDefault(loggerConfig(), logger.InstanceZap)
		logger.Info("configuration loaded", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
	})
	return bootstrapErr
}

// loggerConfig нь орчны тохиргооноос logger-ийн тохиргоог гаргана.
func loggerConfig() logger.Config {
	cfg := logger.Config{
		Level:         logger.LevelInfo,
		EnableConsole: true,
		AppName:       ServiceName,
	}
	if config.AppConfig.Environment == constants.EnvironmentProduction {
		cfg.ConsoleJSONFormat = true
	} else if config.AppConfig.Debug {
		cfg.Level = logger.LevelDebug
	}
	return cfg
}

// moduleHost — kernel/module.Host-ийн server талын хэрэгжилт. Модулиудад
// router, pool, auth middleware болон хуваалцсан service-үүдийг олгоно.
type moduleHost struct {
	api      chi.Router
	pool     *pgxpool.Pool
	authMW   func(http.Handler) http.Handler
	services map[string]any
	workers  []moduleWorker
	shutdown []func()
}

// moduleWorker — модулийн бүртгүүлсэн background worker.
type moduleWorker struct {
	name  string
	every time.Duration
	fn    func(context.Context)
}

func (h *moduleHost) APIRouter() chi.Router { return h.api }
func (h *moduleHost) Pool() *pgxpool.Pool   { return h.pool }
func (h *moduleHost) AuthMiddleware() func(http.Handler) http.Handler {
	return h.authMW
}
func (h *moduleHost) Service(name string) (any, bool) {
	v, ok := h.services[name]
	return v, ok
}

// Provide — модуль usecase-ээ нийтэлнэ (module.ServiceProvider).
func (h *moduleHost) Provide(name string, svc any) { h.services[name] = svc }

// AddWorker — модуль background worker бүртгэнэ (module.WorkerRegistrar).
func (h *moduleHost) AddWorker(name string, every time.Duration, fn func(context.Context)) {
	h.workers = append(h.workers, moduleWorker{name: name, every: every, fn: fn})
}

// OnShutdown — модуль shutdown цэвэрлэгээ бүртгэнэ (module.ShutdownRegistrar).
func (h *moduleHost) OnShutdown(fn func()) { h.shutdown = append(h.shutdown, fn) }

// platformModules — kernel гэрээгээр өөрсдийгөө угсардаг (Phase 1-д
// нүүлгэсэн) модулиудын жагсаалт. Дараагийн алхмуудад үлдсэн модулиуд
// нэг нэгээрээ энэ жагсаалт руу нүүнэ; эцэст нь жагсаалт generated болно.
func platformModules() []module.Module {
	return []module.Module{
		aimod.New(),
		applicationsmod.New(),
		corefindmod.New(),
		eidproxymod.New(),
		gatewayconsolemod.New(),
		govmod.New(),
		gspacemod.New(),
		integrationsmod.New(),
		platformmod.New(),
		providermod.New(),
		registrymod.New(),
		relaymod.New(),
		signmod.New(),
	}
}

type App struct {
	server              *http.Server
	router              chi.Router
	authMiddleware      func(http.Handler) http.Handler
	usersUC             users.Usecase
	pool                *pgxpool.Pool
	redisCache          caches.RedisCache
	tracerShutdown      observability.Shutdown
	authRateLimiter     *middlewares.RateLimiter
	pollRateLimiter     *middlewares.RateLimiter
	govWriteRateLimiter *middlewares.RateLimiter
	modules             *module.Registry // модулийн бүртгэл + идэвхийн төлөв
	// Модулиудын Host-оор бүртгүүлсэн гаралтууд:
	moduleServices map[string]any // нийтэлсэн usecase-ууд (sign г.м.)
	moduleWorkers  []moduleWorker // background worker-ууд (Run-д асна)
	moduleShutdown []func()       // graceful shutdown-ийн цэвэрлэгээ
}

func NewApp() (*App, error) {
	// Тохиргоо/logger-ийг эхлээд ачаална — нимгэн апп үүнийг мартвал
	// config тэг утгатай үлдэж, DB URL/порт хоосон болно.
	if err := Bootstrap(); err != nil {
		return nil, fmt.Errorf("bootstrap config: %w", err)
	}

	ctx := context.Background()

	// Модулийн бүртгэл — builtin манифестууд + MODULES_DISABLED env.
	// Core модуль эсвэл бүртгэлгүй ID унтраах гэвэл boot-ыг шууд унагаана:
	// тохиргооны алдааг чимээгүй үл тоох нь далд эвдрэл болдог.
	modReg := module.Builtin()
	if err := modReg.ApplyDisabledList(config.AppConfig.ModulesDisabled); err != nil {
		return nil, fmt.Errorf("MODULES_DISABLED: %w", err)
	}
	for _, s := range modReg.List() {
		if !s.Enabled {
			logger.Info("модуль унтраалттай", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryServer,
				"module":                 s.Manifest.ID,
			})
		}
	}

	// Tracer-ийг эхэлд тохируулна — ингэснээр дараагийн тохиргооноос
	// ялгарах span-ууд зөв provider руу очно.
	shutdownTracer, err := observability.SetupTracing(ctx, observability.TracingConfig{
		ServiceName: ServiceName,
		Environment: config.AppConfig.Environment,
		Exporter:    config.AppConfig.OTelExporter,
		SampleRatio: config.AppConfig.OTelSampleRatio,
	})
	if err != nil {
		return nil, fmt.Errorf("setup tracing: %w", err)
	}

	// өгөгдлийн сан (pgxpool)
	pool, err := drivers.SetupPgxPostgres(ctx)
	if err != nil {
		return nil, err
	}
	// pool-ийн бодит статистикийг /metrics-ээр гаргана.
	observability.RegisterDBStatsProvider(func() observability.DBPoolStats {
		s := pool.Stat()
		return observability.DBPoolStats{
			OpenConnections: int(s.TotalConns()),
			InUse:           int(s.AcquiredConns()),
			WaitCount:       s.EmptyAcquireCount(),
		}
	})

	// jwt сервис
	jwtService := jwt.NewJWTServiceWithRefresh(
		config.AppConfig.JWTSecret,
		config.AppConfig.JWTIssuer,
		config.AppConfig.JWTExpired,
		config.AppConfig.JWTRefreshExpired,
	)

	// кэш
	redisCache := caches.NewRedisCache(config.AppConfig.REDISHost, 0, config.AppConfig.REDISPassword, time.Duration(config.AppConfig.REDISExpired))
	ristrettoCache, err := caches.NewRistrettoCache()
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}

	// router + глобал middleware. Дараалал чухал: эхэлд tracing — ингэснээр
	// RequestIDMiddleware түүнийг logger context руу холбохоос өмнө span
	// context (trace_id) тогтоогддог.
	r := chi.NewRouter()
	r.Use(middlewares.TracingMiddleware(ServiceName))
	r.Use(middlewares.RequestIDMiddleware())
	// RequestID-ийн дараа — ингэснээр panic-recovery хариунд request_id
	// орж, доош урсгалын бүх middleware+handler-ийн panic баригдана.
	r.Use(middlewares.RecovererMiddleware())
	r.Use(middlewares.MetricsMiddleware())
	r.Use(middlewares.SecurityHeadersMiddleware())
	r.Use(middlewares.CORSMiddleware())
	// Глобал net нь upload-ийн дээд хязгаар (26 MiB) — файл байршуулдаг sign
	// route-ууд үүнийг шаарддаг. Эцгийн middleware нь дэд route-ийг зөвхөн
	// чангалж чаддаг тул энд 1 MiB тавибал sign upload эцэгтээ 413 болно.
	// Ердийн JSON route-уудыг DecodeBody-ийн 1 MiB cap + auth-ийн 4 KiB
	// route-cap хамгаална.
	r.Use(middlewares.BodySizeLimitMiddleware(middlewares.UploadBodyMaxBytes))
	r.Use(middlewares.AccessLogMiddleware())
	r.Use(middlewares.TimeoutMiddleware(middlewares.DefaultRequestTimeout))

	authMiddleware := middlewares.NewAuthMiddleware(jwtService, redisCache, false)

	// Дэд бүтцийн endpoint-ууд (/api бүлгээс гадуур). /health, /ready нь
	// load balancer / orchestrator-т хэрэгтэй тул нээлттэй хэвээр; харин
	// /metrics, /swagger нь операторын мэдрэмжтэй endpoint тул production-д
	// ObservabilityGate-аар (bearer token + 404) хаагдана.
	healthHandler := V1Handler.NewHealthHandler(pool, redisCache.Client())
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)
	isProduction := config.AppConfig.Environment == constants.EnvironmentProduction
	obsGate := middlewares.ObservabilityGate(isProduction, config.AppConfig.ObservabilityToken)
	r.With(obsGate).Handle("/metrics", promhttp.Handler())
	r.With(obsGate).Get("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	})

	// Хязгаарлагдсан контекстуудыг угсарна.
	userRepo := userspostgres.NewUserRepository(pool)
	usersUC := users.NewUsecase(userRepo, ristrettoCache, users.Config{
		BcryptCost: config.AppConfig.BcryptCost,
	})
	// Bootstrap: SUPERADMIN_EMAIL тохируулсан бол тухайн хэрэглэгчийг super admin
	// болгож ахиулна (best-effort; байхгүй бол warning).
	bootstrapSuperAdmin(ctx, userRepo, config.AppConfig.SuperAdminEmail)
	// GeregeCloud Verify API — OTP send/check. (Нууц үг/OTP route-ууд eID-ийн
	// төлөө хасагдсан ч usecase нь verifier-ийг шаардсан хэвээр; цэвэр угсралт.)
	verifier := verify.NewClient(config.AppConfig.VerifyAPIBase, config.AppConfig.VerifyAPIKey, config.AppConfig.VerifyChannel)
	// eID identity provider (RP) — "Login with eID"-ийн цорын ганц нэвтрэх арга.
	eidClient := eid.NewClient(config.AppConfig.EIDBaseURL, config.AppConfig.EIDRPUUID, config.AppConfig.EIDRPName, config.AppConfig.EIDRPSecret, config.AppConfig.EIDCertLevel)
	// Google OAuth — Google account-ийг eID хэрэглэгчид холбох нэвтрэлт.
	googleClient := google.NewClient(config.AppConfig.GoogleClientID, config.AppConfig.GoogleClientSecret)
	// Gerege Verify / XYP — улсын бүртгэлээс байгууллагын мэдээлэл (eID байгууллага холбох).
	xypClient := xyp.NewClient(config.AppConfig.XYPAPIBase, config.AppConfig.XYPClientID, config.AppConfig.XYPClientSecret)

	// Gerege SSO (sso.gerege.mn, OIDC) client — RP нэвтрэлт (ssoUC доор) болон
	// eID proxy-д хуваалцана.
	ssoClient := oidc.NewClient(config.AppConfig.SSOIssuer, config.AppConfig.SSOClientID, config.AppConfig.SSOClientSecret, config.AppConfig.SSORedirectURI, config.AppConfig.SSOScope)

	// SSO eID proxy (сонголттой) — SSO_EID_PROXY_BASE_URL + INTEGRATION_ENC_KEY
	// хоёулаа тохируулсан бол иргэний PKI самбар (summary/certificates/devices/
	// activity) нь шууд eidmongolia-ий оронд sso.gerege.mn/rp/eid-ээр дамжина.
	// Токенуудыг шифрлэн (sso_tokens) хадгалж, хугацаа дуусахад refresh хийнэ.
	// Хоосон бол шууд eidmongolia зам (өөрчлөлтгүй).
	var (
		ssoEidProxy    auth.SSOEidProxy
		ssoTokens      auth.SSOTokenService
		ssoTokenStorer sso.TokenStorer
	)
	if config.AppConfig.SSOEidProxyBaseURL != "" && config.AppConfig.IntegrationEncKey != "" {
		tokenCipher, cErr := crypto.New(config.AppConfig.IntegrationEncKey)
		if cErr != nil {
			return nil, fmt.Errorf("init sso token cipher: %w", cErr)
		}
		tokenSvc := ssotoken.New(ssotokenpostgres.NewSSOTokenRepository(pool, tokenCipher), ssoClient)
		ssoTokens = tokenSvc
		ssoTokenStorer = tokenSvc
		ssoEidProxy = ssoeidproxy.New(config.AppConfig.SSOEidProxyBaseURL)
		logger.Info("SSO eID proxy enabled — PKI dashboard reads proxied via SSO", logger.Fields{"base": config.AppConfig.SSOEidProxyBaseURL})
	}

	authUC := auth.NewUsecase(usersUC, jwtService, verifier, eidClient, xypClient, googleClient, redisCache, auth.Config{
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

	// RBAC — динамик role/permission удирдлага + enforcement.
	rbacRepo := rbacpostgres.NewRBACRepository(pool)
	rbacUC := rbac.NewUsecase(rbacRepo)

	// Organizations — байгууллага + гишүүнчлэл (RLS-тэй; бичих эрх usecase-д).
	orgRepo := orgpostgres.NewOrgRepository(pool)
	orgUC := org.NewUsecase(orgRepo)

	// Gov портал (gov) — modules/gov дотор угсарна (route + SLA sweep worker).

	// API Gateway — services/routes/consumers/api keys/policies + телеметр.
	// Usecase нь kernel түвшинд үлдэнэ: хүсэлтийн лог queue + eID proxy
	// түүнээс хамаардаг; удирдлагын route нь modules/gatewayconsole-д.
	gatewayRepo := gatewaypostgres.NewGatewayRepository(pool)
	gatewayUC := gateway.NewUsecase(gatewayRepo)

	// Relay (дамжуулалт + SLA) — modules/relay; регистр — modules/registry.

	// Gerege Core лавлагаа (core-find) — modules/corefind дотор угсарна.

	// Gerege SSO (sso.gerege.mn, OIDC) — гадаад SSO provider-т нэвтрэх RP урсгал.
	// Энэ апп нь sso.gerege.mn-ий relying party: нэвтрэлтийг тийш даатгаж, буцаж
	// ирсэн code-ийг токен болгож солин, хэрэглэгчийг sso_sub-ээр upsert хийнэ.
	// ssoClient дээр (eID proxy-тай хамт) угсарсан. ssoTokenStorer нь SSO eID
	// proxy идэвхтэй үед нэвтрэлтийн дараа токенуудыг хадгална (nil бол алгасна).
	ssoRepo := ssouserpostgres.NewSSOUserRepository(pool)
	// Платформын хандалтын горим (public|private) — SSO нэвтрэлтийн gate болон
	// superadmin тохиргоонд хэрэглэнэ.
	platformSettingsRepo := platformsettings.NewRepository(pool)
	ssoUC := sso.NewUsecase(ssoClient, ssoRepo, jwtService, redisCache, config.AppConfig.SSONativeClientID, ssoTokenStorer, platformSettingsRepo)

	// Гуравдагч интеграци (integrations) ба Gerege Space (gspace) — өөрсдийн
	// modules/<id>/module.go дотор угсарна (Phase 1 modular wiring).
	// Гарын үсэг (хувь хүн) + байгууллагын тамга (ADMIN) — зураг Google Drive-д, URL DB-д.
	orgStampRepo := orgstamppostgres.NewOrgStampRepository(pool)
	assetsUC := assets.NewUsecase(usersUC, userRepo, orgStampRepo, eidClient)

	// Audit — persisted hash-chained, append-only audit log (admin-only унших API).
	// audit_log нь admin-only тул repository нь хүсэлтийн RLS-аас үл хамааран
	// транзакц дотроо service/admin GUC тогтоодог.
	auditRepo := auditpostgres.NewAuditRepository(pool)
	auditUC := audit.NewUsecase(auditRepo)

	// Модулийн lifecycle — DB-д хадгалагдсан унтраалттай төлвийг сэргээж
	// (зөөлөн: stale мөр warning), admin toggle-ийн usecase-ийг угсарна.
	platformModulesStore := platformmodulespostgres.NewRepository(pool)
	if disabledIDs, err := platformModulesStore.ListDisabled(ctx); err != nil {
		// Migration хараахан гүйгээгүй орчинд boot-ыг унагаахгүй — default
		// (бүгд идэвхтэй) төлвөөр үргэлжилнэ.
		logger.Warn("platform_modules уншигдсангүй — default төлвөөр үргэлжилнэ", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryServer,
			"error":                  err.Error(),
		})
	} else {
		for _, rerr := range modReg.RestoreDisabled(disabledIDs) {
			logger.Warn("хадгалагдсан модулийн төлөв сэргээгдсэнгүй", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryServer,
				"error":                  rerr.Error(),
			})
		}
	}

	// Super admin — админ хэрэглэгчдийг удирдах (үүсгэх/эрх олгох/хасах) +
	// super admin урилга (allow-list). users давхаргаар (кэш-зөв мутациуд)
	// ажиллаж, мутаци бүрийг audit log-д бичнэ.
	superadminInviteRepo := superadmininvitepostgres.NewSuperadminInviteRepository(pool)
	superadminUC := superadmin.NewUsecase(usersUC, auditUC, superadminInviteRepo, platformSettingsRepo)

	// Super admin бүртгэлийн шидтэн (урилга → Google → eID → и-мэйл OTP →
	// TOTP) + MFA-тай super admin нэвтрэлтийн 2 дахь шат. TOTP secret-ийг
	// storage-д AES-GCM-ээр шифрлэх түлхүүр хэрэгтэй. INTEGRATION_ENC_KEY
	// тохируулсан бол түүнийг ашиглана; тохируулаагүй бол JWT_SECRET-ээс
	// domain-separated тогтвортой түлхүүр гаргаж авна (репод ил биш,
	// restart-д тогтвортой) — ингэснээр superadmin MFA-г нэмэлт env
	// тохируулахгүйгээр асаана. crypto.New утгыг SHA-256-аар 32 байт болгодог
	// тул урт ямар ч байсан ажиллана. АНХААР: энэ түлхүүр (эсвэл JWT_SECRET)-ийг
	// нэгэнт superadmin MFA идэвхжсэн хойно солиход өмнөх TOTP secret задрахаа
	// болино — тиймээс тогтвортой байлгана.
	totpEncKey := config.AppConfig.IntegrationEncKey
	if totpEncKey == "" {
		totpEncKey = config.AppConfig.JWTSecret + "|superadmin-mfa-v1"
		logger.Warn("superadmin MFA: INTEGRATION_ENC_KEY not set — deriving TOTP encryption key from JWT_SECRET (set INTEGRATION_ENC_KEY for a dedicated key)", logger.Fields{})
	}
	var onboardingUC onboarding.Usecase
	{
		recoveryRepo := recoverypostgres.NewRecoveryCodeRepository(pool)
		superadminAcctRepo := superadminaccountpostgres.NewSuperadminAccountRepository(pool)
		uc, ucErr := onboarding.NewUsecase(
			googleClient, eidClient, verifier,
			userRepo, recoveryRepo, superadminAcctRepo, superadminInviteRepo,
			jwtService, redisCache, totpEncKey,
			onboarding.Config{
				Issuer:         config.AppConfig.JWTIssuer,
				PendingTTL:     30 * time.Minute,
				OTPTTL:         time.Duration(config.AppConfig.REDISExpired) * time.Minute,
				OTPMaxAttempts: config.AppConfig.OTPMaxAttempts,
				MFAMaxAttempts: 5,
				EIDDisplayText: config.AppConfig.EIDDisplayText,
			},
		)
		if ucErr != nil {
			return nil, fmt.Errorf("init superadmin onboarding usecase: %w", ucErr)
		}
		onboardingUC = uc
	}

	// Security events — RASP-style ingest (нэвтэрсэн хэрэглэгч бичнэ, admin унших).
	securityRepo := securitypostgres.NewSecurityEventRepository(pool)
	securityUC := security.NewUsecase(securityRepo)

	// Site appearance — сайтын нийтийн харагдацын default (landing уншина,
	// admin 'settings.manage'-ээр өөрчилнө). Нийтийн config тул RLS-гүй plain pool.
	siteRepo := sitepostgres.NewSiteRepository(pool)
	siteUC := siteuc.NewUsecase(siteRepo)

	// Landing themes — нэрлэсэн бүрэн загварууд (харагдац + текст/цэс). Идэвхтэйг
	// нэвтрээгүй зочны landing уншина; админ CRUD/идэвхжүүлнэ. Нийтийн config, RLS-гүй.
	themeRepo := themepostgres.NewThemeRepository(pool)
	themeUC := themeuc.NewUsecase(themeRepo)

	// Gemini client-үүд — kernel түвшинд: AI модулиас гадна хэлний модуль
	// (орчуулга) хэрэглэдэг. AI pipeline-ийн үлдсэн угсралт modules/ai-д.
	geminiClient := gemini.NewClient(config.AppConfig.GeminiAPIBase, config.AppConfig.GeminiAPIKey, config.AppConfig.GeminiModel).
		WithEmbedModel(config.AppConfig.GeminiEmbedModel)
	geminiTTSClient := gemini.NewClient(config.AppConfig.GeminiAPIBase, config.AppConfig.GeminiAPIKey, config.AppConfig.GeminiTTSModel)

	// Интерфейсийн хэл — super admin хэл нэмж/хасч, орчуулгыг гараар эсвэл
	// Gemini-ээр бөглөнө. Түлхүүрийн жагсаалт нь аппынх (frontend-д
	// багцлагдсан); платформ зөвхөн утгыг хадгална. Нийтийн config, RLS-гүй.
	languageRepo := languagepostgres.NewLanguageRepository(pool)
	languageUC := languageuc.NewUsecase(languageRepo, languageuc.NewGeminiTranslator(geminiClient))

	// Гарын үсэг (sign) — modules/sign дотор угсарна (ServiceSign нийтэлнэ).

	// TRUSTED_PROXIES хоосон бол clientIP() нь X-Forwarded-For-д итгэхгүй тул
	// урвуу proxy-гийн ард (энэ template-ийн топологи: nginx → web BFF → api,
	// api нь нийтийн порт-гүй) БҮХ хүсэлт нэг proxy peer IP дор орж, per-IP
	// rate-limit ба audit-ийн клиент-IP таних нь ажиллахаа болино. Boot үед
	// сануулна (fail-closed биш — шууд интернетэд ил api-д proxy байхгүй байж
	// болно). BFF нь клиент IP-г XFF-ээр дамжуулдаг (frontend lib/api.ts).
	if len(config.AppConfig.TrustedProxiesList()) == 0 {
		logger.Warn("TRUSTED_PROXIES хоосон — клиент IP нь proxy peer рүү унана; урвуу proxy-гийн ард per-IP rate-limit ба audit клиент-IP таних ажиллахгүй. proxy/docker сүлжээгээ TRUSTED_PROXIES-д заана уу (docs/DEPLOYMENT.md).",
			logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
	}

	// Нэргүй /auth гадаргуун дээр IP тус бүрт минутанд 5 хүсэлт зөвшөөрнө.
	authRateLimiter := middlewares.NewRateLimiter(rate.Limit(5.0/60.0), 5)
	// /eid/poll нь unauthenticated бөгөөд IdP-г 25с хүртэл long-poll хийж
	// холболт барьдаг. 5/мин-ийн чанга хязгаарт орвол long-poll өөрөө 429
	// болно. Иймд тусдаа СУЛ limiter — IP тус бүрт ~60/мин (burst 30): frontend
	// ~2.5с тутам poll хийхэд (~24/мин) хангалттай зайтай, гэхдээ нэг IP-гээс
	// хязгааргүй concurrent long-poll эхлүүлэх slow-DoS-д таазтай болгоно.
	pollRateLimiter := middlewares.NewRateLimiter(rate.Limit(1.0), 30)
	// /gov-ийн МУТАЦИ endpoint-ууд (хүсэлт/лавлагаа/цаг үүсгэх г.м.) — нэвтэрсэн
	// хэрэглэгч тус бүрт мөр үүсгэхийг хязгаарлана (өөрийн RLS-мөрд storage-abuse).
	// Уншилтад хамаарахгүй; ~30/мин (burst 15) нь энгийн хэрэглээнд элбэг зайтай.
	govWriteRateLimiter := middlewares.NewRateLimiter(rate.Limit(30.0/60.0), 15)

	// OIDC provider — өөрийн login/consent/logout цөм. Өмнө нь Ory Hydra
	// эзэмшдэг байсан challenge/client бүртгэлийг одоо усгэсэн usecases/oidc +
	// oauth_clients хүснэгт эзэмшинэ (Hydra-аас хамаарахаа больсон).
	oauthClients := oauthpostgres.NewClientRepository(pool)
	oidcSvc := oidcuc.NewService(oauthClients, oauthpostgres.NewFlowRepository(pool), config.AppConfig.Issuer())
	// Login/consent/logout урсгал (providerUC) — modules/provider дотор угсарна.

	// Applications (Gateway consumer + SSO RP) — modules/applications дотор угсарна.

	// Өөрийн OIDC provider-ийн гарын үсгийн түлхүүр. Эхний ажиллагаанд RSA
	// түлхүүр үүсгэж, INTEGRATION_ENC_KEY-ээр шифрлэн хадгална. Түлхүүр бэлэн
	// биш бол id_token гаргах боломжгүй тул boot зогсоно (fail-closed).
	oidcKeys, err := oidcuc.NewKeyManager(oauthpostgres.NewKeyRepository(pool), config.AppConfig.IntegrationEncKey)
	if err != nil {
		return nil, fmt.Errorf("oidc signing keys: %w", err)
	}
	if err := oidcKeys.EnsureKey(ctx); err != nil {
		return nil, fmt.Errorf("oidc: ensure signing key: %w", err)
	}
	// Түлхүүр болон иргэний бүртгэл бэлэн болсны дараа token гаргах чадварыг
	// залгана (id_token-ий гарын үсэг + claims).
	oidcSvc.WithTokenIssuing(oidcKeys, usersUC)

	// Гуравдагч талын RP-ийн gateway хүсэлтийг (/rp/sign, /api/v1/provider) API
	// Gateway-ийн лог руу async бичих middleware (detached ctx тул хоцролтгүй;
	// best-effort). DAN-ий өөрийн first-party API трафикийг лог-лохгүй —
	// шүүлтүүр middleware дотор (isRPGatewayPath).
	//
	// Хүсэлт бүрт хязгааргүй goroutine салгахын оронд буфертэй queue + цөөн
	// тогтмол worker ашиглана: DB удаашрах/ханах үед goroutine хуримтлагдахгүй,
	// queue дүүрвэл лог-ийг чимээгүй хаяна (best-effort). Бичилт бүр богино
	// timeout-той тул ханасан DB нэг ч worker-ийг мөнхөд түгжихгүй.
	gwLogQueue := make(chan gateway.RequestLogInput, 512)
	for i := 0; i < 4; i++ {
		go func() {
			for in := range gwLogQueue {
				writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				gatewayUC.RecordRequest(writeCtx, in)
				cancel()
			}
		}()
	}
	gwLogMW := middlewares.GatewayRequestLogMiddleware(func(method, path, ip string, status, latencyMS int) {
		select {
		case gwLogQueue <- gateway.RequestLogInput{
			Method: method, Path: path, ClientIP: ip, Status: status, LatencyMS: latencyMS,
		}:
		default:
			// Queue дүүрсэн — энэ нэг лог мөрийг хаяна (edge трафикийг блоклохгүй).
		}
	})

	// API Route-ууд
	// Өөрийн OIDC provider-ийн НИЙТИЙН endpoint-ууд. /api бүлгээс ГАДУУР, үндэс
	// дээр — замыг нь OIDC стандарт (/.well-known/*) болон nginx-ийн одоо байгаа
	// дүрмүүд (/oauth2/*, /userinfo) тогтоосон.
	routes.NewOIDCRoute(r, oidcKeys, oidcSvc, config.AppConfig.Issuer()).Routes()

	var (
		moduleRegErr   error
		moduleServices map[string]any
		moduleWorkers  []moduleWorker
		moduleShutdown []func()
	)
	r.Route("/api", func(api chi.Router) {
		api.Use(gwLogMW)
		// Модулийн gate — идэвхгүй модулийн бүх route 404. Телеметрийн ДАРАА
		// байрлана: хаагдсан хүсэлт ч gateway логт харагдана.
		api.Use(module.Gate(modReg))

		// ── Модулиудын өөрийн wiring (Phase 1 modular framework) ──────────
		// modules/<id>/module.go өөрсдөө repo → usecase → route угсралтаа
		// хийнэ; server.go тэдний дотоод бүтцийг мэдэхгүй. Хуваалцсан
		// хамаарлууд Host service locator-оор очно; модулиуд өөрсдөө ч
		// service нийтэлдэг (ai, sign) тул core route-уудаас ӨМНӨ бүртгэнэ.
		host := &moduleHost{
			api:    api,
			pool:   pool,
			authMW: authMiddleware,
			services: map[string]any{
				module.ServiceRBAC:             rbacUC,
				module.ServiceAudit:            auditUC,
				module.ServiceUsers:            usersUC,
				module.ServiceWriteRateLimiter: govWriteRateLimiter,
				module.ServiceRedis:            redisCache,
				module.ServiceAssets:           assetsUC,
				module.ServiceGateway:          gatewayUC,
				module.ServiceGeminiChat:       geminiClient,
				module.ServiceGeminiTTS:        geminiTTSClient,
				module.ServiceAuth:             authUC,
				module.ServiceOIDCService:      oidcSvc,
				module.ServiceModuleRegistry:   modReg,
				module.ServiceModuleStore:      platformModulesStore,
			},
		}
		for _, mod := range platformModules() {
			if err := mod.Register(ctx, host); err != nil {
				moduleRegErr = fmt.Errorf("module %s: %w", mod.ID(), err)
				return
			}
		}
		// ai модулийн нийтэлсэн usecase — core admin route (/admin/ai/prompts)
		// хэрэглэнэ.
		aiUC, ok := module.ServiceAs[ai.Usecase](host, module.ServiceAI)
		if !ok {
			moduleRegErr = fmt.Errorf("module ai: %q service нийтлэгдсэнгүй", module.ServiceAI)
			return
		}
		moduleServices = host.services
		moduleWorkers = host.workers
		moduleShutdown = host.shutdown

		api.Get("/", routes.RootHandler)
		routes.NewAuthRoute(api, authUC, auditUC, WalletProvisioner, authMiddleware, authRateLimiter, pollRateLimiter).Routes()
		routes.NewUsersRoute(api, usersUC, authMiddleware, ssoEidProxy != nil).Routes()
		routes.NewEIDProfileRoute(api, authUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewRBACRoute(api, rbacUC, auditUC, authMiddleware).Routes()
		routes.NewOrgRoute(api, orgUC, auditUC, authMiddleware).Routes()
		routes.NewAssetsRoute(api, assetsUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewSSORoute(api, ssoUC).Routes()
		routes.NewAdminRoute(api, usersUC, rbacUC, aiUC, authMiddleware).Routes()
		routes.NewSuperAdminRoute(api, superadminUC, authMiddleware).Routes()
		// Super admin бүртгэл + MFA — нэвтрээгүй гадаргуу (rate limit + service RLS).
		// Зөвхөн INTEGRATION_ENC_KEY тохируулагдсан үед идэвхжинэ (эс бөгөөс inert).
		if onboardingUC != nil {
			routes.NewSuperAdminOnboardRoute(api, onboardingUC, authRateLimiter, pollRateLimiter).Routes()
		}
		routes.NewAuditRoute(api, auditUC, authMiddleware).Routes()
		routes.NewSecurityRoute(api, securityUC, authMiddleware).Routes()
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
		routes.NewSiteRoute(api, siteUC, rbacUC, authMiddleware, authSurface).Routes()
		routes.NewThemeRoute(api, themeUC, rbacUC, authMiddleware).Routes()
		routes.NewLanguageRoute(api, languageUC, authMiddleware).Routes()
		// eID service proxy (/v1/eid*) — modules/eidproxy; OIDC provider-ийн
		// login/consent/logout (/v1/provider) — modules/provider дотор угсарна.
	})
	if moduleRegErr != nil {
		return nil, moduleRegErr
	}

	// OIDC provider — /admin оператор гадаргуу (RP OAuth2 client бүртгэл/удирдлага
	// + admin API key). sso.gerege.mn нь Ory Hydra-г урдаа тавьж SSO болно. Зөвхөн
	// Hydra тохируулагдсан (ProviderConfigured) үед идэвхжинэ; эс бөгөөс inert.
	if config.AppConfig.ProviderConfigured() {
		devAppsStore := devapps.New(pool)
		adminKeyStore := adminkeys.New(pool, config.AppConfig.SSOAdminAPIKeysList())
		// chi.Mount нь plain http.Handler-ийн r.URL.Path-аас prefix-ыг хасдаггүй
		// тул StripPrefix-ээр хасна — ингэснээр доторх ServeMux нь /api/v1/...
		// pattern-тэй таарна.
		r.Mount("/admin", http.StripPrefix("/admin", adminapi.New(oauthClients, devAppsStore, adminKeyStore).Router()))
		logger.Info("OIDC provider admin surface mounted at /admin", logger.Fields{
			"issuer": config.AppConfig.Issuer(),
		})
	}

	// Sign relay — 3 дагч RP (template.gerege.mn гэх мэт) dan-аар ДАМЖИН eID гарын
	// үсэг зурах reverse-proxy (/rp/sign/*). dan-ий eidmongolia RP creds шаардана.
	if config.AppConfig.SignRelayToken != "" && config.AppConfig.EIDRPSecret != "" {
		if relay, rerr := signrelay.New(config.AppConfig.EIDBaseURL, config.AppConfig.EIDRPSecret, config.AppConfig.SignRelayToken); rerr != nil {
			logger.Warn("sign relay init failed", logger.Fields{"error": rerr.Error()})
		} else {
			// RP-ийн gateway хүсэлт тул лог middleware-ээр ороож бичнэ.
			r.Handle("/rp/sign/*", gwLogMW(relay))
			r.Handle("/rp/sign", gwLogMW(relay))
			logger.Info("sign relay mounted at /rp/sign (RP eID signing via dan)", logger.Fields{})
		}
	}

	// Серверийн түвшний timeout-ууд (slowloris / удаан client-ийн эсрэг):
	//   - ReadTimeout нь header+body уншилтыг бүхэлд нь хязгаарлана;
	//   - WriteTimeout нь handler + хариу бичилтийг хамардаг тул request-
	//     түвшний ХАМГИЙН УРТ timeout-аас (AIRequestTimeout, 50s) урт байх
	//     ёстой — эс тэгвээс удаан AI хариу бичих үед холболт тасарна;
	//   - IdleTimeout нь сул keep-alive холболтыг чөлөөлнө;
	//   - MaxHeaderBytes нь body-н хязгаараас гадуурх том header-ийн
	//     дайралтыг хаана (JWT+cookie 16 KiB-д амархан багтана).
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.AppConfig.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      middlewares.AIRequestTimeout + 20*time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	return &App{
		server:              srv,
		router:              r,
		authMiddleware:      authMiddleware,
		usersUC:             usersUC,
		pool:                pool,
		redisCache:          redisCache,
		tracerShutdown:      shutdownTracer,
		authRateLimiter:     authRateLimiter,
		pollRateLimiter:     pollRateLimiter,
		govWriteRateLimiter: govWriteRateLimiter,
		modules:             modReg,
		moduleServices:      moduleServices,
		moduleWorkers:       moduleWorkers,
		moduleShutdown:      moduleShutdown,
	}, nil
}

// startBackgroundWorkers нь модулиудын Host-оор бүртгүүлсэн background
// worker-уудыг (relay/gov SLA sweep, demo simulator г.м.) эхлүүлнэ.
// Worker бүр тусдаа goroutine — нэг нь гацахад нөгөө нь зогсохгүй; ctx
// cancel болоход бүгд зогсоно.
func (a *App) startBackgroundWorkers(ctx context.Context) {
	tick := func(every time.Duration, fn func(context.Context)) {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// Алхмын context-ыг worker context-оос гаргана — shutdown үед
				// явж буй алхам ч цуцлагдана.
				stepCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				fn(stepCtx)
				cancel()
			}
		}
	}
	for _, w := range a.moduleWorkers {
		go tick(w.every, w.fn)
	}
}

// Router нь суурийн chi router-ыг буцаана. Апп нь Run() дуудахаас ӨМНӨ
// өөрийн маршрутаа энд бүртгэнэ — жишээ нь:
//
//	app.Router().Route("/api/ring", ring.Routes(app.Pool()))
//
// Суурийн маршрутууд аль хэдийн бүртгэгдсэн тул зам давхцуулахгүй байхыг анхаар.
func (a *App) Router() chi.Router { return a.router }

// Pool нь апп өөрийн repository-гоо байгуулахад хэрэглэх DB pool-ыг буцаана.
func (a *App) Pool() *pgxpool.Pool { return a.pool }

// Modules нь модулийн бүртгэлийг буцаана — нимгэн апп өөрийн модулиа
// нэмж бүртгэх/идэвхийг нь шалгахад хэрэглэнэ.
func (a *App) Modules() *module.Registry { return a.modules }

// AuthMiddleware нь суурийн JWT танилтын middleware-ийг буцаана — апп өөрийн
// маршрутаа ижил session-оор хамгаалахад хэрэглэнэ:
//
//	app.Router().Route("/api/v1/wallet", wallet.Routes(app.AuthMiddleware()))
func (a *App) AuthMiddleware() func(http.Handler) http.Handler { return a.authMiddleware }

// Users нь суурийн хэрэглэгчийн usecase-ыг буцаана — апп өөрийн модульдаа
// иргэний нэр/РД-г шийдэхэд хэрэглэнэ.
func (a *App) Users() users.Usecase { return a.usersUC }

// Sign нь sign модулийн нийтэлсэн гарын үсгийн usecase-ыг буцаана — апп нь
// VerifiedDigest-ээр гүйлгээгээ иргэний eID гарын үсэгт уяхад хэрэглэнэ.
// Sign модуль бүртгэгдээгүй бол nil.
func (a *App) Sign() sign.Usecase {
	if uc, ok := a.moduleServices[module.ServiceSign].(sign.Usecase); ok {
		return uc
	}
	return nil
}

func (a *App) Run() (err error) {
	srvLog := logger.WithFields(logger.Fields{constants.LoggerCategory: constants.LoggerCategoryServer})

	// SLA хяналтын background worker-ууд — shutdown үед workerCtx cancel болж зогсоно.
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	a.startBackgroundWorkers(workerCtx)

	go func() {
		srvLog.Infof("success to listen and serve on %s", a.server.Addr)
		if listenErr := a.server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			srvLog.Fatalf("Failed to listen and serve: %+v", listenErr)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	srvLog.Info("shutdown server ...")
	stopWorkers()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Шинэ холболт хүлээж авахаа болиод, явагдаж буй хүсэлтүүдийг гүйцээнэ.
	if shutdownErr := a.server.Shutdown(ctx); shutdownErr != nil {
		return fmt.Errorf("error when shutdown server: %v", shutdownErr)
	}

	// Rate limiter-уудын cleanup goroutine-уудыг зогсооно.
	if a.authRateLimiter != nil {
		a.authRateLimiter.Stop()
	}
	if a.pollRateLimiter != nil {
		a.pollRateLimiter.Stop()
	}
	if a.govWriteRateLimiter != nil {
		a.govWriteRateLimiter.Stop()
	}
	// Модулиудын бүртгүүлсэн цэвэрлэгээ (rate limiter Stop г.м.).
	for _, fn := range a.moduleShutdown {
		fn()
	}

	// өгөгдлийн сангийн pool-г хаах
	a.pool.Close()

	// redis холболтыг хаах
	if rErr := a.redisCache.Close(); rErr != nil {
		srvLog.Errorf("error closing redis: %v", rErr)
	}

	// batch exporter-ийн span-уудыг flush хийнэ.
	if a.tracerShutdown != nil {
		if tErr := a.tracerShutdown(ctx); tErr != nil {
			srvLog.Errorf("tracer shutdown incomplete: %v", tErr)
		}
	}

	srvLog.Info("server exiting")
	return
}

// bootstrapSuperAdmin нь SUPERADMIN_EMAIL тохируулсан бол тухайн и-мэйлтэй
// хэрэглэгчийг super admin (RoleSuperAdmin) болгож ахиулна. Service RLS context
// дор ажиллана (users_service бодлого бүх мөрд хандана). Best-effort: хэрэглэгч
// байхгүй/аль хэдийн super admin/алдаа гарвал boot-ийг эвдэлгүй warning бичнэ.
// migration ажиллаагүй (roles(4) байхгүй) орчинд ч boot зогсохгүй.
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

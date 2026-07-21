// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	docs "template/docs" // swagger тодорхойлолт, swaggo-оор init үед бүртгэгддэг
	"template/internal/business/domain"
	"template/internal/business/usecases/ai"
	applicationsuc "template/internal/business/usecases/applications"
	"template/internal/business/usecases/assets"
	"template/internal/business/usecases/audit"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/core"
	"template/internal/business/usecases/gateway"
	"template/internal/business/usecases/gov"
	"template/internal/business/usecases/gspace"
	"template/internal/business/usecases/integrations"
	oidcuc "template/internal/business/usecases/oidc"
	"template/internal/business/usecases/org"
	provideruc "template/internal/business/usecases/provider"
	"template/internal/business/usecases/rbac"
	"template/internal/business/usecases/registry"
	relayuc "template/internal/business/usecases/relay"
	"template/internal/business/usecases/security"
	"template/internal/business/usecases/sign"
	siteuc "template/internal/business/usecases/site"
	"template/internal/business/usecases/sso"
	"template/internal/business/usecases/ssotoken"
	"template/internal/business/usecases/superadmin"
	onboarding "template/internal/business/usecases/superadmin_onboarding"
	themeuc "template/internal/business/usecases/theme"
	"template/internal/business/usecases/users"
	"template/internal/config"
	"template/internal/constants"
	"template/internal/datasources/caches"
	"template/internal/datasources/drivers"
	repointerface "template/internal/datasources/repositories/interface"
	aipostgres "template/internal/datasources/repositories/postgres/ai"
	applicationspostgres "template/internal/datasources/repositories/postgres/applications"
	auditpostgres "template/internal/datasources/repositories/postgres/audit"
	gatewaypostgres "template/internal/datasources/repositories/postgres/gateway"
	govpostgres "template/internal/datasources/repositories/postgres/gov"
	oauthpostgres "template/internal/datasources/repositories/postgres/oauth"
	orgpostgres "template/internal/datasources/repositories/postgres/org"
	orgstamppostgres "template/internal/datasources/repositories/postgres/orgstamp"
	platformsettings "template/internal/datasources/repositories/postgres/platformsettings"
	rbacpostgres "template/internal/datasources/repositories/postgres/rbac"
	recoverypostgres "template/internal/datasources/repositories/postgres/recovery"
	registrypostgres "template/internal/datasources/repositories/postgres/registry"
	relaypostgres "template/internal/datasources/repositories/postgres/relay"
	securitypostgres "template/internal/datasources/repositories/postgres/security"
	sitepostgres "template/internal/datasources/repositories/postgres/site"
	ssotokenpostgres "template/internal/datasources/repositories/postgres/ssotoken"
	ssouserpostgres "template/internal/datasources/repositories/postgres/ssouser"
	superadminaccountpostgres "template/internal/datasources/repositories/postgres/superadminaccount"
	superadmininvitepostgres "template/internal/datasources/repositories/postgres/superadmininvite"
	themepostgres "template/internal/datasources/repositories/postgres/theme"
	userintegrationspostgres "template/internal/datasources/repositories/postgres/userintegrations"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	"template/internal/datasources/rls"
	V1Handler "template/internal/http/handlers/v1"
	"template/internal/http/middlewares"
	"template/internal/http/routes"
	"template/internal/provider/adminapi"
	"template/internal/provider/adminkeys"
	"template/internal/provider/devapps"
	"template/internal/provider/signrelay"
	"template/pkg/crypto"
	"template/pkg/eid"
	"template/pkg/gemini"
	"template/pkg/google"
	gspaceclient "template/pkg/gspace"
	"template/pkg/jwt"
	"template/pkg/logger"
	"template/pkg/observability"
	"template/pkg/oidc"
	"template/pkg/ssoeidproxy"
	"template/pkg/verify"
	"template/pkg/xyp"

	"github.com/jackc/pgx/v5/pgxpool"
)

const serviceName = "gerege-template"

type App struct {
	server              *http.Server
	pool                *pgxpool.Pool
	redisCache          caches.RedisCache
	tracerShutdown      observability.Shutdown
	authRateLimiter     *middlewares.RateLimiter
	aiRateLimiter       *middlewares.RateLimiter
	pollRateLimiter     *middlewares.RateLimiter
	govWriteRateLimiter *middlewares.RateLimiter
	relayUC             relayuc.Usecase // SLA worker + demo simulator (background)
	relayDemo           bool
	govUC               gov.Usecase // gov SLA sweep (background)
}

func NewApp() (*App, error) {
	ctx := context.Background()

	// Tracer-ийг эхэлд тохируулна — ингэснээр дараагийн тохиргооноос
	// ялгарах span-ууд зөв provider руу очно.
	shutdownTracer, err := observability.SetupTracing(ctx, observability.TracingConfig{
		ServiceName: serviceName,
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
	r.Use(middlewares.TracingMiddleware(serviceName))
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

	// Government SSO (sso.dgov.mn, OIDC) client — RP нэвтрэлт (ssoUC доор) болон
	// eID proxy-д хуваалцана.
	ssoClient := oidc.NewClient(config.AppConfig.SSOIssuer, config.AppConfig.SSOClientID, config.AppConfig.SSOClientSecret, config.AppConfig.SSORedirectURI, config.AppConfig.SSOScope)

	// SSO eID proxy (сонголттой) — SSO_EID_PROXY_BASE_URL + INTEGRATION_ENC_KEY
	// хоёулаа тохируулсан бол иргэний PKI самбар (summary/certificates/devices/
	// activity) нь шууд eidmongolia-ий оронд sso.dgov.mn/rp/eid-ээр дамжина.
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

	// Gov — иргэний "Төрийн үйлчилгээ" портал (per-user өгөгдөл RLS-тэй; каталог
	// нийтийн).
	govRepo := govpostgres.NewGovRepository(pool)
	govUC := gov.NewUsecase(govRepo)

	// API Gateway — services/routes/consumers/api keys/policies + телеметр.
	gatewayRepo := gatewaypostgres.NewGatewayRepository(pool)
	gatewayUC := gateway.NewUsecase(gatewayRepo)

	// Platform-хоорондын хүсэлт дамжуулах + SLA хяналт (relay).
	relayUC := relayuc.NewUsecase(relaypostgres.NewRelayRepository(pool))

	// Үйлчилгээний нэгдсэн регистр (CPSV-AP паспорт + нотолгооны каталог) —
	// мастер өгөгдөл тул RLS-гүй; хамгаалалт нь registry.view/manage эрхээр.
	registryUC := registry.NewUsecase(registrypostgres.NewRegistryRepository(pool))

	// Gerege Core (core.gerege.mn) — USER FIND / ORG FIND хайлтын wrap.
	coreUC := core.NewUsecase(config.AppConfig.CoreAPIBase, config.AppConfig.CoreAPIToken)

	// Government SSO (sso.dgov.mn, OIDC) — гадаад SSO provider-т нэвтрэх RP урсгал.
	// Энэ апп нь sso.dgov.mn-ий relying party: нэвтрэлтийг тийш даатгаж, буцаж
	// ирсэн code-ийг токен болгож солин, хэрэглэгчийг sso_sub-ээр upsert хийнэ.
	// ssoClient дээр (eID proxy-тай хамт) угсарсан. ssoTokenStorer нь SSO eID
	// proxy идэвхтэй үед нэвтрэлтийн дараа токенуудыг хадгална (nil бол алгасна).
	ssoRepo := ssouserpostgres.NewSSOUserRepository(pool)
	// Платформын хандалтын горим (public|private) — SSO нэвтрэлтийн gate болон
	// superadmin тохиргоонд хэрэглэнэ.
	platformSettingsRepo := platformsettings.NewRepository(pool)
	ssoUC := sso.NewUsecase(ssoClient, ssoRepo, jwtService, redisCache, config.AppConfig.SSONativeClientID, ssoTokenStorer, platformSettingsRepo)

	// Хэрэглэгчийн гуравдагч этгээдийн интеграци (Google Drive/Meet, Dropbox) —
	// OAuth токеныг шифрлэн хадгална (RLS-тэй per-user хүснэгт).
	userIntegrationsRepo := userintegrationspostgres.NewUserIntegrationsRepository(pool)
	// Гарын үсэг (хувь хүн) + байгууллагын тамга (ADMIN) — зураг Google Drive-д, URL DB-д.
	orgStampRepo := orgstamppostgres.NewOrgStampRepository(pool)
	assetsUC := assets.NewUsecase(usersUC, userRepo, orgStampRepo, eidClient)
	integrationsUC, err := integrations.NewUsecase(userIntegrationsRepo, config.AppConfig.IntegrationEncKey, isProduction)
	if err != nil {
		return nil, fmt.Errorf("init integrations usecase: %w", err)
	}

	// Gerege Space — апп-ын өөрийн SFTP хадгалалт (per-user 2MB, OAuth-гүй, шууд
	// холбогдсон). Тохиргоо (GSPACE_*) хоосон бол Configured()=false болж
	// endpoint-ууд 500 буцаана; UI нь "тохируулаагүй" төлөвийг зохицуулна.
	gspaceClient := gspaceclient.NewClient(gspaceclient.Config{
		Host:     config.AppConfig.GSpaceHost,
		Port:     config.AppConfig.GSpacePort,
		User:     config.AppConfig.GSpaceUser,
		Password: config.AppConfig.GSpacePassword,
		BasePath: config.AppConfig.GSpaceBasePath,
		HostKey:  config.AppConfig.GSpaceHostKey,
		// Production-д host key заавал (MITM-аас хамгаална); development-д
		// тохируулаагүй бол шалгахгүйгээр зөвшөөрнө.
		AllowInsecureHostKey: !isProduction,
	})
	gspaceUC := gspace.NewUsecase(gspaceClient, config.AppConfig.GSpaceQuota)

	// Audit — persisted hash-chained, append-only audit log (admin-only унших API).
	// audit_log нь admin-only тул repository нь хүсэлтийн RLS-аас үл хамааран
	// транзакц дотроо service/admin GUC тогтоодог.
	auditRepo := auditpostgres.NewAuditRepository(pool)
	auditUC := audit.NewUsecase(auditRepo)

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

	// AI pipeline — Gemini REST client + function-calling tools. TTS нь
	// audio гаргадаг тусдаа model тул өөр client-ээр явна. Repo нь DB-ээс
	// тохируулдаг prompt давхаргууд + search_knowledge tool-ийн мэдлэгийн сан.
	geminiClient := gemini.NewClient(config.AppConfig.GeminiAPIBase, config.AppConfig.GeminiAPIKey, config.AppConfig.GeminiModel)
	geminiTTSClient := gemini.NewClient(config.AppConfig.GeminiAPIBase, config.AppConfig.GeminiAPIKey, config.AppConfig.GeminiTTSModel)
	aiRepo := aipostgres.NewAIRepository(pool)
	aiTools := append(ai.DefaultTools(), ai.KnowledgeSearchTool(aiRepo))
	aiUC := ai.NewUsecase(geminiClient, geminiTTSClient, aiRepo, aiTools, ai.Config{
		Voice:       config.AppConfig.GeminiVoice,
		ScopePrompt: config.AppConfig.AIScopePrompt,
	})

	// PDF гарын үсэг (PAdES) — eidmongolia /v3-ээр. Серверийн байнгын
	// Document-Signer гэрчилгээ + түлхүүрийг файлаас (SIGN_SIGNER_*) уншина;
	// хоосон бол production-д fail-closed, development-д dev self-signed.
	signerCertPEM, signerKeyPEM, err := loadSignerMaterial()
	if err != nil {
		return nil, fmt.Errorf("load document-signer material: %w", err)
	}
	signUC, err := sign.NewUsecase(redisCache, sign.Config{
		// EIDBaseURL нь "/v3"-ийг агуулдаг (default https://eidmongolia.mn/v3);
		// sign usecase өөрөө "/v3/signature/..." нэмдэг тул суурийг "/v3"-гүй
		// болгож, /v3/v3 давхардлаас сэргийлнэ.
		V3BaseURL:     signV3Base(config.AppConfig.EIDBaseURL),
		RPUUID:        config.AppConfig.EIDRPUUID,
		RPName:        config.AppConfig.EIDRPName,
		APISecret:     config.AppConfig.EIDRPSecret,
		SignerCertPEM: signerCertPEM,
		SignerKeyPEM:  signerKeyPEM,
		IsProduction:  config.AppConfig.Environment == constants.EnvironmentProduction,
	})
	if err != nil {
		return nil, fmt.Errorf("init sign usecase: %w", err)
	}

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
	// Gemini дуудлага үнэтэй — /ai-д IP тус бүрт минутанд 20 хүсэлт. Burst-ийг
	// 10 болгов: live орчуулга ~8-10 chunk/мин илгээдэг тул эхний тэсрэлт 5-д
	// багтахгүй, хууль ёсны stream 429 болж болзошгүй байв.
	aiRateLimiter := middlewares.NewRateLimiter(rate.Limit(20.0/60.0), 10)
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
	providerUC := provideruc.NewUsecase(oidcSvc, oauthClients, usersUC,
		config.AppConfig.SSOFirstPartyClientsList(), config.AppConfig.Issuer())

	// Нэгдсэн Applications (Gateway consumer + SSO RP). Client бүртгэл нь одоо
	// өөрийн DB-д (oauth_clients) амьдардаг тул Hydra-аас хамаарахаа больсон.
	applicationsUC := applicationsuc.NewUsecase(
		applicationspostgres.NewApplicationRepository(pool),
		oauthClients,
	)

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

	r.Route("/api", func(api chi.Router) {
		api.Use(gwLogMW)

		api.Get("/", routes.RootHandler)
		routes.NewAuthRoute(api, authUC, auditUC, authMiddleware, authRateLimiter, pollRateLimiter).Routes()
		routes.NewUsersRoute(api, usersUC, authMiddleware, ssoEidProxy != nil).Routes()
		routes.NewEIDProfileRoute(api, authUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewRBACRoute(api, rbacUC, auditUC, authMiddleware).Routes()
		routes.NewOrgRoute(api, orgUC, auditUC, authMiddleware).Routes()
		routes.NewGovRoute(api, govUC, rbacUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewIntegrationsRoute(api, integrationsUC, authMiddleware).Routes()
		routes.NewAssetsRoute(api, assetsUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewGSpaceRoute(api, gspaceUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewGatewayRoute(api, gatewayUC, rbacUC, authMiddleware).Routes()
		// Хүсэлт дамжуулах + SLA хяналт (JWT + relay эрх). SLA sweep + demo
		// simulator/generator нь App.Run-д background-аар ажиллана.
		routes.NewRelayRoute(api, relayUC, rbacUC, authMiddleware).Routes()
		// Үйлчилгээний нэгдсэн регистр — админ бүртгэл (registry.view/manage) +
		// нийтийн каталог (нэвтэрсэн дурын иргэн).
		routes.NewRegistryRoute(api, registryUC, rbacUC, authMiddleware).Routes()
		routes.NewCatalogRoute(api, registryUC, authMiddleware).Routes()
		routes.NewApplicationsRoute(api, applicationsUC, rbacUC, authMiddleware).Routes()
		routes.NewCoreRoute(api, coreUC, rbacUC, authMiddleware).Routes()
		routes.NewSSORoute(api, ssoUC).Routes()
		routes.NewAdminRoute(api, usersUC, rbacUC, aiUC, authMiddleware).Routes()
		routes.NewSuperAdminRoute(api, superadminUC, authMiddleware).Routes()
		// Super admin бүртгэл + MFA — нэвтрээгүй гадаргуу (rate limit + service RLS).
		// Зөвхөн INTEGRATION_ENC_KEY тохируулагдсан үед идэвхжинэ (эс бөгөөс inert).
		if onboardingUC != nil {
			routes.NewSuperAdminOnboardRoute(api, onboardingUC, authRateLimiter, pollRateLimiter).Routes()
		}
		routes.NewAIRoute(api, aiUC, authMiddleware, aiRateLimiter).Routes()
		routes.NewAuditRoute(api, auditUC, authMiddleware).Routes()
		routes.NewSecurityRoute(api, securityUC, authMiddleware).Routes()
		routes.NewSiteRoute(api, siteUC, rbacUC, authMiddleware).Routes()
		routes.NewThemeRoute(api, themeUC, rbacUC, authMiddleware).Routes()
		routes.NewSignRoute(api, signUC, usersUC, assetsUC, authMiddleware).Routes()
		// OIDC provider login/consent/logout.
		routes.NewProviderRoute(api, providerUC, authMiddleware).Routes()
	})

	// OIDC provider — /admin оператор гадаргуу (RP OAuth2 client бүртгэл/удирдлага
	// + admin API key). sso.dgov.mn нь Ory Hydra-г урдаа тавьж SSO болно. Зөвхөн
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

	// Sign relay — 3 дагч RP (template.dgov.mn гэх мэт) dan-аар ДАМЖИН eID гарын
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
	//     түвшний timeout (TimeoutMiddleware, 30s)-аас урт байх ёстой;
	//   - IdleTimeout нь сул keep-alive холболтыг чөлөөлнө;
	//   - MaxHeaderBytes нь body-н хязгаараас гадуурх том header-ийн
	//     дайралтыг хаана (JWT+cookie 16 KiB-д амархан багтана).
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.AppConfig.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * middlewares.DefaultRequestTimeout,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	return &App{
		server:              srv,
		pool:                pool,
		redisCache:          redisCache,
		tracerShutdown:      shutdownTracer,
		authRateLimiter:     authRateLimiter,
		aiRateLimiter:       aiRateLimiter,
		pollRateLimiter:     pollRateLimiter,
		govWriteRateLimiter: govWriteRateLimiter,
		relayUC:             relayUC,
		relayDemo:           config.AppConfig.RelayDemoMode,
		govUC:               govUC,
	}, nil
}

// startBackgroundWorkers нь SLA хяналтын background goroutine-уудыг эхлүүлнэ:
//   - relay sweep (reminder/overdue/breach/escalate) 20с тутам
//   - gov sweep (иргэний хүсэлтийн хугацаа хэтрэлт + чимээгүй зөвшөөрөл) 60с тутам
//   - relay demo simulator (RELAY_DEMO_MODE) 10/25с тутам
//
// Модуль тус бүр ТУСДАА шалгагдана — нэг нь холбогдоогүй байхад нөгөө нь
// зогсох ёсгүй. ctx cancel болоход бүгд зогсоно.
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
	if a.relayUC != nil {
		go tick(20*time.Second, func(c context.Context) { _ = a.relayUC.SLASweep(c) })
		if a.relayDemo {
			go tick(10*time.Second, a.relayUC.SimulateStep)   // доод platform-уудын хариу дуурайх
			go tick(25*time.Second, a.relayUC.SimulateIngest) // шинэ демо хүсэлт үүсгэх
		}
	}
	// Иргэний хүсэлтийн SLA — хугацаа хэтрэлт тэмдэглэх + чимээгүй зөвшөөрөл
	// хэрэгжүүлэх. Relay-ээс сийрэг (60с): хүний хугацаа цаг/хоногоор хэмжигддэг
	// тул илүү нягт шалгах нь ачааллаас өөр үр дүн авчрахгүй.
	if a.govUC != nil {
		go tick(60*time.Second, func(c context.Context) { _ = a.govUC.SLASweep(c) })
	}
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
	if a.aiRateLimiter != nil {
		a.aiRateLimiter.Stop()
	}
	if a.pollRateLimiter != nil {
		a.pollRateLimiter.Stop()
	}
	if a.govWriteRateLimiter != nil {
		a.govWriteRateLimiter.Stop()
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

// signV3Base нь sign usecase-д зориулж eID суурь URL-ийг бэлдэнэ. My config-ийн
// EIDBaseURL нь "/v3"-ийг агуулдаг (default https://eidmongolia.mn/v3); sign
// usecase өөрөө "/v3/signature/..." нэмдэг тул эндээс trailing "/v3"-ийг хасаж
// /v3/v3 давхардлаас сэргийлнэ.
func signV3Base(eidBaseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(eidBaseURL), "/")
	base = strings.TrimSuffix(base, "/v3")
	if base == "" {
		return "https://eidmongolia.mn"
	}
	return base
}

// loadSignerMaterial нь серверийн байнгын Document-Signer гэрчилгээ + түлхүүрийн
// PEM-ийг config-ийн файл замаас (SIGN_SIGNER_CERT_FILE / SIGN_SIGNER_KEY_FILE)
// уншина. Хоёулаа хоосон бол nil буцаана — sign.NewUsecase production-д
// fail-closed, development-д dev self-signed руу шилжинэ. Зөвхөн нэг нь өгөгдвөл
// алдаа (буруу хагас тохиргооноос сэргийлнэ).
func loadSignerMaterial() (certPEM, keyPEM []byte, err error) {
	certFile := strings.TrimSpace(config.AppConfig.SignSignerCertFile)
	keyFile := strings.TrimSpace(config.AppConfig.SignSignerKeyFile)
	if certFile == "" && keyFile == "" {
		return nil, nil, nil
	}
	if certFile == "" || keyFile == "" {
		return nil, nil, fmt.Errorf("SIGN_SIGNER_CERT_FILE ба SIGN_SIGNER_KEY_FILE хоёуланг хамт тохируул")
	}
	// #nosec G304 — зам нь оператор SIGN_SIGNER_CERT_FILE env-ээр өгдөг боот
	// тохиргоо; хүсэлтийн/хэрэглэгчийн оролтоос биш (taint биш).
	certPEM, err = os.ReadFile(certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read signer cert: %w", err)
	}
	// #nosec G304 — оператор SIGN_SIGNER_KEY_FILE env-ээр өгсөн зам.
	keyPEM, err = os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read signer key: %w", err)
	}
	return certPEM, keyPEM, nil
}

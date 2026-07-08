// Gerege Template Version 27.0
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
	"template/internal/business/usecases/assets"
	"template/internal/business/usecases/audit"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/core"
	"template/internal/business/usecases/gateway"
	"template/internal/business/usecases/gov"
	"template/internal/business/usecases/gspace"
	"template/internal/business/usecases/integrations"
	"template/internal/business/usecases/org"
	"template/internal/business/usecases/rbac"
	"template/internal/business/usecases/security"
	"template/internal/business/usecases/sign"
	"template/internal/business/usecases/sso"
	"template/internal/business/usecases/superadmin"
	"template/internal/business/usecases/users"
	"template/internal/config"
	"template/internal/constants"
	"template/internal/datasources/caches"
	"template/internal/datasources/drivers"
	repointerface "template/internal/datasources/repositories/interface"
	aipostgres "template/internal/datasources/repositories/postgres/ai"
	auditpostgres "template/internal/datasources/repositories/postgres/audit"
	gatewaypostgres "template/internal/datasources/repositories/postgres/gateway"
	govpostgres "template/internal/datasources/repositories/postgres/gov"
	orgpostgres "template/internal/datasources/repositories/postgres/org"
	orgstamppostgres "template/internal/datasources/repositories/postgres/orgstamp"
	rbacpostgres "template/internal/datasources/repositories/postgres/rbac"
	securitypostgres "template/internal/datasources/repositories/postgres/security"
	ssouserpostgres "template/internal/datasources/repositories/postgres/ssouser"
	userintegrationspostgres "template/internal/datasources/repositories/postgres/userintegrations"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	"template/internal/datasources/rls"
	V1Handler "template/internal/http/handlers/v1"
	"template/internal/http/middlewares"
	"template/internal/http/routes"
	"template/pkg/eid"
	"template/pkg/gemini"
	"template/pkg/google"
	gspaceclient "template/pkg/gspace"
	"template/pkg/jwt"
	"template/pkg/logger"
	"template/pkg/observability"
	"template/pkg/oidc"
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
	r.Use(middlewares.BodySizeLimitMiddleware(middlewares.DefaultBodyMaxBytes))
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

	// Gerege Core (core.gerege.mn) — USER FIND / ORG FIND хайлтын wrap.
	coreUC := core.NewUsecase(config.AppConfig.CoreAPIBase, config.AppConfig.CoreAPIToken)

	// Gerege SSO (sso.gerege.mn, OIDC) — eID-ийн зэрэгцээ 2 дахь нэвтрэлт.
	ssoClient := oidc.NewClient(config.AppConfig.SSOIssuer, config.AppConfig.SSOClientID, config.AppConfig.SSOClientSecret, config.AppConfig.SSORedirectURI, config.AppConfig.SSOScope)
	ssoRepo := ssouserpostgres.NewSSOUserRepository(pool)
	ssoUC := sso.NewUsecase(ssoClient, ssoRepo, jwtService, redisCache, config.AppConfig.SSONativeClientID)

	// Хэрэглэгчийн гуравдагч этгээдийн интеграци (Google Drive/Meet, Dropbox) —
	// OAuth токеныг шифрлэн хадгална (RLS-тэй per-user хүснэгт).
	userIntegrationsRepo := userintegrationspostgres.NewUserIntegrationsRepository(pool)
	// Гарын үсэг (хувь хүн) + байгууллагын тамга (ADMIN) — зураг Google Drive-д, URL DB-д.
	orgStampRepo := orgstamppostgres.NewOrgStampRepository(pool)
	assetsUC := assets.NewUsecase(usersUC, userRepo, orgStampRepo, eidClient)
	integrationsUC, err := integrations.NewUsecase(userIntegrationsRepo, config.AppConfig.IntegrationEncKey)
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
	})
	gspaceUC := gspace.NewUsecase(gspaceClient, config.AppConfig.GSpaceQuota)

	// Audit — persisted hash-chained, append-only audit log (admin-only унших API).
	// audit_log нь admin-only тул repository нь хүсэлтийн RLS-аас үл хамааран
	// транзакц дотроо service/admin GUC тогтоодог.
	auditRepo := auditpostgres.NewAuditRepository(pool)
	auditUC := audit.NewUsecase(auditRepo)

	// Super admin — админ хэрэглэгчдийг удирдах (үүсгэх/эрх олгох/хасах). users
	// давхаргаар (кэш-зөв мутациуд) ажиллаж, мутаци бүрийг audit log-д бичнэ.
	superadminUC := superadmin.NewUsecase(usersUC, auditUC)

	// Security events — RASP-style ingest (нэвтэрсэн хэрэглэгч бичнэ, admin унших).
	securityRepo := securitypostgres.NewSecurityEventRepository(pool)
	securityUC := security.NewUsecase(securityRepo)

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

	// API Route-ууд
	r.Route("/api", func(api chi.Router) {
		api.Get("/", routes.RootHandler)
		routes.NewAuthRoute(api, authUC, auditUC, authMiddleware, authRateLimiter, pollRateLimiter).Routes()
		routes.NewUsersRoute(api, usersUC, authMiddleware).Routes()
		routes.NewEIDProfileRoute(api, authUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewRBACRoute(api, rbacUC, auditUC, authMiddleware).Routes()
		routes.NewOrgRoute(api, orgUC, auditUC, authMiddleware).Routes()
		routes.NewGovRoute(api, govUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewIntegrationsRoute(api, integrationsUC, authMiddleware).Routes()
		routes.NewAssetsRoute(api, assetsUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewGSpaceRoute(api, gspaceUC, authMiddleware, govWriteRateLimiter).Routes()
		routes.NewGatewayRoute(api, gatewayUC, rbacUC, authMiddleware).Routes()
		routes.NewCoreRoute(api, coreUC, authMiddleware).Routes()
		routes.NewSSORoute(api, ssoUC).Routes()
		routes.NewAdminRoute(api, usersUC, rbacUC, aiUC, authMiddleware).Routes()
		routes.NewSuperAdminRoute(api, superadminUC, authMiddleware).Routes()
		routes.NewAIRoute(api, aiUC, authMiddleware, aiRateLimiter).Routes()
		routes.NewAuditRoute(api, auditUC, authMiddleware).Routes()
		routes.NewSecurityRoute(api, securityUC, authMiddleware).Routes()
		routes.NewSignRoute(api, signUC, usersUC, assetsUC, authMiddleware).Routes()
	})

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
	}, nil
}

func (a *App) Run() (err error) {
	srvLog := logger.WithFields(logger.Fields{constants.LoggerCategory: constants.LoggerCategoryServer})

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

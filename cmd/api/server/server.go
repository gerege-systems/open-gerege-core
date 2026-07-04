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
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	docs "template/docs" // swagger тодорхойлолт, swaggo-оор init үед бүртгэгддэг
	"template/internal/business/usecases/ai"
	"template/internal/business/usecases/audit"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/org"
	"template/internal/business/usecases/rbac"
	"template/internal/business/usecases/security"
	"template/internal/business/usecases/users"
	"template/internal/config"
	"template/internal/constants"
	"template/internal/datasources/caches"
	"template/internal/datasources/drivers"
	aipostgres "template/internal/datasources/repositories/postgres/ai"
	auditpostgres "template/internal/datasources/repositories/postgres/audit"
	orgpostgres "template/internal/datasources/repositories/postgres/org"
	rbacpostgres "template/internal/datasources/repositories/postgres/rbac"
	securitypostgres "template/internal/datasources/repositories/postgres/security"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	V1Handler "template/internal/http/handlers/v1"
	"template/internal/http/middlewares"
	"template/internal/http/routes"
	"template/pkg/eid"
	"template/pkg/gemini"
	"template/pkg/jwt"
	"template/pkg/logger"
	"template/pkg/observability"
	"template/pkg/verify"

	"github.com/jackc/pgx/v5/pgxpool"
)

const serviceName = "gerege-template"

type App struct {
	server          *http.Server
	pool            *pgxpool.Pool
	redisCache      caches.RedisCache
	tracerShutdown  observability.Shutdown
	authRateLimiter *middlewares.RateLimiter
	aiRateLimiter   *middlewares.RateLimiter
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
	// GeregeCloud Verify API — OTP send/check. (Нууц үг/OTP route-ууд eID-ийн
	// төлөө хасагдсан ч usecase нь verifier-ийг шаардсан хэвээр; цэвэр угсралт.)
	verifier := verify.NewClient(config.AppConfig.VerifyAPIBase, config.AppConfig.VerifyAPIKey, config.AppConfig.VerifyChannel)
	// eID identity provider (RP) — "Login with eID"-ийн цорын ганц нэвтрэх арга.
	eidClient := eid.NewClient(config.AppConfig.EIDBaseURL, config.AppConfig.EIDRPUUID, config.AppConfig.EIDRPName, config.AppConfig.EIDRPSecret, config.AppConfig.EIDCertLevel)
	authUC := auth.NewUsecase(usersUC, jwtService, verifier, eidClient, redisCache, auth.Config{
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

	// Audit — persisted hash-chained, append-only audit log (admin-only унших API).
	// audit_log нь admin-only тул repository нь хүсэлтийн RLS-аас үл хамааран
	// транзакц дотроо service/admin GUC тогтоодог.
	auditRepo := auditpostgres.NewAuditRepository(pool)
	auditUC := audit.NewUsecase(auditRepo)

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

	// Нэргүй /auth гадаргуун дээр IP тус бүрт минутанд 5 хүсэлт зөвшөөрнө.
	authRateLimiter := middlewares.NewRateLimiter(rate.Limit(5.0/60.0), 5)
	// Gemini дуудлага үнэтэй — /ai-д IP тус бүрт минутанд 20 хүсэлт, burst 5.
	// Live орчуулга ~6-8 секунд тутамд chunk илгээдэг (~8-10/мин) тул үүнд
	// багтахуйц, гэхдээ abuse-ээс хамгаалсан түвшин.
	aiRateLimiter := middlewares.NewRateLimiter(rate.Limit(20.0/60.0), 5)

	// API Route-ууд
	r.Route("/api", func(api chi.Router) {
		api.Get("/", routes.RootHandler)
		routes.NewAuthRoute(api, authUC, auditUC, authMiddleware, authRateLimiter).Routes()
		routes.NewUsersRoute(api, usersUC, authMiddleware).Routes()
		routes.NewRBACRoute(api, rbacUC, auditUC, authMiddleware).Routes()
		routes.NewOrgRoute(api, orgUC, auditUC, authMiddleware).Routes()
		routes.NewAdminRoute(api, usersUC, rbacUC, aiUC, authMiddleware).Routes()
		routes.NewAIRoute(api, aiUC, authMiddleware, aiRateLimiter).Routes()
		routes.NewAuditRoute(api, auditUC, authMiddleware).Routes()
		routes.NewSecurityRoute(api, securityUC, authMiddleware).Routes()
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
		server:          srv,
		pool:            pool,
		redisCache:      redisCache,
		tracerShutdown:  shutdownTracer,
		authRateLimiter: authRateLimiter,
		aiRateLimiter:   aiRateLimiter,
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

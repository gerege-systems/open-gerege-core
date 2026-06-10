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
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/internal/config"
	"template/internal/constants"
	"template/internal/datasources/caches"
	"template/internal/datasources/drivers"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	V1Handler "template/internal/http/handlers/v1"
	"template/internal/http/middlewares"
	"template/internal/http/routes"
	"template/pkg/jwt"
	"template/pkg/logger"
	"template/pkg/mailer"
	"template/pkg/observability"

	"github.com/jackc/pgx/v5/pgxpool"
)

const serviceName = "go-rest-boilerplate"

type App struct {
	server          *http.Server
	pool            *pgxpool.Pool
	redisCache      caches.RedisCache
	asyncMailer     *mailer.AsyncOTPMailer
	tracerShutdown  observability.Shutdown
	authRateLimiter *middlewares.RateLimiter
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

	// mailer — синхрон SMTP илгээгчийг асинхрон дараалалд боож, OTP
	// илгээх хоцролтыг HTTP хүсэлтийн замаас гаргана.
	syncMailer := mailer.NewOTPMailer(config.AppConfig.OTPEmail, config.AppConfig.OTPPassword)
	asyncMailer := mailer.NewAsyncOTPMailer(
		syncMailer,
		config.AppConfig.MailerWorkers,
		config.AppConfig.MailerQueueSize,
		config.AppConfig.MailerRetries,
		time.Second,
	)

	// router + глобал middleware. Дараалал чухал: эхэлд tracing — ингэснээр
	// RequestIDMiddleware түүнийг logger context руу холбохоос өмнө span
	// context (trace_id) тогтоогддог.
	r := chi.NewRouter()
	r.Use(middlewares.TracingMiddleware(serviceName))
	r.Use(middlewares.RequestIDMiddleware())
	r.Use(middlewares.MetricsMiddleware())
	r.Use(middlewares.SecurityHeadersMiddleware())
	r.Use(middlewares.CORSMiddleware())
	r.Use(middlewares.BodySizeLimitMiddleware(middlewares.DefaultBodyMaxBytes))
	r.Use(middlewares.AccessLogMiddleware())
	r.Use(middlewares.TimeoutMiddleware(middlewares.DefaultRequestTimeout))

	authMiddleware := middlewares.NewAuthMiddleware(jwtService, redisCache, false)

	// Дэд бүтцийн endpoint-ууд (/api бүлгээс гадуур)
	healthHandler := V1Handler.NewHealthHandler(pool, redisCache.Client())
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	})

	// Хязгаарлагдсан контекстуудыг угсарна.
	userRepo := userspostgres.NewUserRepository(pool)
	usersUC := users.NewUsecase(userRepo, ristrettoCache, users.Config{
		BcryptCost: config.AppConfig.BcryptCost,
	})
	authUC := auth.NewUsecase(usersUC, jwtService, asyncMailer, redisCache, auth.Config{
		OTPMaxAttempts:    config.AppConfig.OTPMaxAttempts,
		OTPTTL:            time.Duration(config.AppConfig.REDISExpired) * time.Minute,
		PasswordResetTTL:  30 * time.Minute,
		BcryptCost:        config.AppConfig.BcryptCost,
		LoginMaxAttempts:  10,
		LoginLockoutTTL:   15 * time.Minute,
		ForgotMaxAttempts: 3,
		ForgotLockoutTTL:  15 * time.Minute,
	})

	// Нэргүй /auth гадаргуун дээр IP тус бүрт минутанд 5 хүсэлт зөвшөөрнө.
	authRateLimiter := middlewares.NewRateLimiter(rate.Limit(5.0/60.0), 5)

	// API Route-ууд
	r.Route("/api", func(api chi.Router) {
		api.Get("/", routes.RootHandler)
		routes.NewAuthRoute(api, authUC, authMiddleware, authRateLimiter).Routes()
		routes.NewUsersRoute(api, usersUC, authMiddleware).Routes()
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.AppConfig.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &App{
		server:          srv,
		pool:            pool,
		redisCache:      redisCache,
		asyncMailer:     asyncMailer,
		tracerShutdown:  shutdownTracer,
		authRateLimiter: authRateLimiter,
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

	// Rate limiter-ийн cleanup goroutine-ийг зогсооно.
	if a.authRateLimiter != nil {
		a.authRateLimiter.Stop()
	}

	// асинхрон mailer-ийн дарааллыг гүйцээнэ.
	if a.asyncMailer != nil {
		if mErr := a.asyncMailer.Shutdown(ctx); mErr != nil {
			srvLog.Errorf("mailer shutdown incomplete: %v", mErr)
		}
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

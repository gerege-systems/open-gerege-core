// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package superadmin нь super admin (core) модулийн wiring: админ
// удирдлага (/v1/superadmin) + бүртгэлийн шидтэн ба MFA
// (/v1/auth/superadmin — нэвтрээгүй гадаргуу, rate limit-тэй).
package superadmin

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	superadminuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/superadmin"
	onboarding "github.com/gerege-systems/open-gerege-core/core/business/usecases/superadmin_onboarding"
	usersuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/datasources/caches"
	repointerface "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/interface"
	platformsettings "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/platformsettings"
	recoverypostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/recovery"
	superadminaccountpostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/superadminaccount"
	superadmininvitepostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/superadmininvite"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/eid"
	"github.com/gerege-systems/open-gerege-core/pkg/google"
	"github.com/gerege-systems/open-gerege-core/pkg/jwt"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
	"github.com/gerege-systems/open-gerege-core/pkg/oidc"
	"github.com/gerege-systems/open-gerege-core/pkg/verify"
)

//go:embed migrations
var migrationsDir embed.FS

// Module — superadmin модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "superadmin" }

// Migrations нь модулийн өөрийн SQL migration-уудыг буцаана. Файлын
// дугаар нь ГЛОБАЛ дугаарлалтаас хэвээр үлдсэн — нүүлгэлтийн явцад анхны
// дарааллыг мөшгих боломжтой байлгах тул.
func (*Module) Migrations() migrate.MigrationFS {
	sub, err := fs.Sub(migrationsDir, "migrations")
	if err != nil {
		panic("superadmin: migrations FS: " + err.Error())
	}
	return sub
}

// Register нь super admin удирдлага + бүртгэлийн шидтэний угсралтыг хийнэ.
func (m *Module) Register(_ context.Context, host module.Host) error {
	usersUC, ok := module.ServiceAs[usersuc.Usecase](host, module.ServiceUsers)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceUsers)
	}
	auditUC, ok := module.ServiceAs[audituc.Usecase](host, module.ServiceAudit)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceAudit)
	}
	userRepo, ok := module.ServiceAs[repointerface.UserRepository](host, module.ServiceUserRepo)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceUserRepo)
	}
	jwtService, ok := module.ServiceAs[jwt.JWTService](host, module.ServiceJWT)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceJWT)
	}
	redisCache, ok := module.ServiceAs[caches.RedisCache](host, module.ServiceRedis)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceRedis)
	}
	googleClient, ok := module.ServiceAs[*google.Client](host, module.ServiceGoogle)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceGoogle)
	}
	eidClient, ok := module.ServiceAs[eid.Client](host, module.ServiceEID)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceEID)
	}
	verifier, ok := module.ServiceAs[*verify.Client](host, module.ServiceVerifier)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceVerifier)
	}
	authLimiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServiceAuthRateLimiter)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServiceAuthRateLimiter)
	}
	pollLimiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServicePollRateLimiter)
	if !ok {
		return fmt.Errorf("superadmin: host-д %q service алга", module.ServicePollRateLimiter)
	}

	pool := host.Pool()
	inviteRepo := superadmininvitepostgres.NewSuperadminInviteRepository(pool)

	// Админ хэрэглэгчдийг удирдах (үүсгэх/эрх олгох/хасах) + урилга
	// (allow-list). users давхаргаар (кэш-зөв мутациуд) ажиллаж, мутаци
	// бүрийг audit log-д бичнэ.
	uc := superadminuc.NewUsecase(usersUC, auditUC, inviteRepo, platformsettings.NewRepository(pool))

	// Бүртгэлийн шидтэн (урилга → Google → eID → и-мэйл OTP → TOTP) + MFA-тай
	// super admin нэвтрэлтийн 2 дахь шат. TOTP secret-ийг storage-д AES-GCM-ээр
	// шифрлэх түлхүүр хэрэгтэй. INTEGRATION_ENC_KEY тохируулсан бол түүнийг
	// ашиглана; эс бөгөөс JWT_SECRET-ээс domain-separated тогтвортой түлхүүр
	// гаргаж авна (репод ил биш, restart-д тогтвортой) — ингэснээр superadmin
	// MFA-г нэмэлт env тохируулахгүйгээр асаана. crypto.New утгыг SHA-256-аар
	// 32 байт болгодог тул урт ямар ч байсан ажиллана. АНХААР: энэ түлхүүрийг
	// (эсвэл JWT_SECRET-ийг) superadmin MFA идэвхжсэн хойно солиход өмнөх TOTP
	// secret задрахаа болино — тиймээс тогтвортой байлгана.
	totpEncKey := config.AppConfig.IntegrationEncKey
	if totpEncKey == "" {
		totpEncKey = config.AppConfig.JWTSecret + "|superadmin-mfa-v1"
		logger.Warn("superadmin MFA: INTEGRATION_ENC_KEY not set — deriving TOTP encryption key from JWT_SECRET (set INTEGRATION_ENC_KEY for a dedicated key)", logger.Fields{})
	}
	onboardingUC, err := onboarding.NewUsecase(
		googleClient,
		// Төвийн SSO — Google-ийн оронд эхлэх хувилбарын IdP. Платформ
		// бүрийн SSO redirect URI аль хэдийн бүртгэгдсэн байдаг тул шинэ
		// платформ нэмэхэд Google Console-д гар ажиллагаа шаардахгүй.
		oidc.NewClient(
			config.AppConfig.SSOIssuer, config.AppConfig.SSOClientID,
			config.AppConfig.SSOClientSecret, config.AppConfig.SSORedirectURI,
			config.AppConfig.SSOScope,
		),
		eidClient, verifier,
		userRepo,
		recoverypostgres.NewRecoveryCodeRepository(pool),
		superadminaccountpostgres.NewSuperadminAccountRepository(pool),
		inviteRepo,
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
	if err != nil {
		return fmt.Errorf("superadmin: init onboarding usecase: %w", err)
	}

	api := host.APIRouter()
	routes.NewSuperAdminRoute(api, uc, host.AuthMiddleware()).Routes()
	routes.NewSuperAdminOnboardRoute(api, onboardingUC, authLimiter, pollLimiter).Routes()
	return nil
}

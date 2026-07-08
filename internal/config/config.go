// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
	"template/internal/constants"
)

var AppConfig Config

type Config struct {
	Port        int    `mapstructure:"PORT"`
	Environment string `mapstructure:"ENVIRONMENT"`
	Debug       bool   `mapstructure:"DEBUG"`

	DBPostgreDriver string `mapstructure:"DB_POSTGRE_DRIVER"`
	DBPostgreDsn    string `mapstructure:"DB_POSTGRE_DSN"`
	DBPostgreURL    string `mapstructure:"DB_POSTGRE_URL"`

	DBMaxOpenConns    int `mapstructure:"DB_MAX_OPEN_CONNS"`
	DBMaxIdleConns    int `mapstructure:"DB_MAX_IDLE_CONNS"`
	DBConnMaxLifeMins int `mapstructure:"DB_CONN_MAX_LIFE_MINS"`

	JWTSecret         string `mapstructure:"JWT_SECRET"`
	JWTExpired        int    `mapstructure:"JWT_EXPIRED"`
	JWTIssuer         string `mapstructure:"JWT_ISSUER"`
	JWTRefreshExpired int    `mapstructure:"JWT_REFRESH_EXPIRED"` // хоног

	OTPMaxAttempts int `mapstructure:"OTP_MAX_ATTEMPTS"`

	// GeregeCloud Verify API (verify.gecloud.mn) — бүх email/SMS OTP-г (бүртгэл
	// баталгаажуулах, нууц үг сэргээх) энэ үйлчилгээгээр илгээж/шалгана. SMTP
	// огт ашиглахгүй. VERIFY_API_KEY production-д заавал шаардлагатай.
	VerifyAPIBase string `mapstructure:"VERIFY_API_BASE"`
	VerifyAPIKey  string `mapstructure:"VERIFY_API_KEY"`
	VerifyChannel string `mapstructure:"VERIFY_CHANNEL"`

	// Gerege Verify / XYP (xyp.gerege.mn) — улсын бүртгэлээс байгууллагын мэдээлэл
	// авах лавлагаа API. HTTP Basic Auth (client_id:client_secret). Креденшлгүй бол
	// eID байгууллага холбох функц идэвхгүй болно (boot-ыг эвдэхгүй; сонголттой).
	XYPAPIBase      string `mapstructure:"XYP_API_BASE"`
	XYPClientID     string `mapstructure:"XYP_CLIENT_ID"`
	XYPClientSecret string `mapstructure:"XYP_CLIENT_SECRET"`

	// Gerege Space — апп-ын өөрийн SFTP хадгалалт. Хэрэглэгч бүр квоттой (default
	// 2MB). Host/User/Password нууц (env-д). Тохируулаагүй бол функц идэвхгүй.
	GSpaceHost     string `mapstructure:"GSPACE_HOST"`
	GSpacePort     int    `mapstructure:"GSPACE_PORT"`
	GSpaceUser     string `mapstructure:"GSPACE_USER"`
	GSpacePassword string `mapstructure:"GSPACE_PASSWORD"`
	GSpaceBasePath string `mapstructure:"GSPACE_BASE_PATH"`
	GSpaceQuota    int64  `mapstructure:"GSPACE_QUOTA_BYTES"`

	// eID identity provider (RP contract) — энэ template нь Relying Party.
	// "Login with eID" нь цорын ганц нэвтрэх арга тул эдгээр нь сонголттой
	// биш ч boot-ийг эвдэхгүйн тулд бүгд зохистой default-той (production-д
	// шинэ required-var шалгалт нэмэхгүй — IdP-ийн нийтийн endpoint өгөгдмөл).
	// EIDCallbackURL нь IdP-ийн allowlist-д бүртгэгдсэн URL байх ёстой.
	EIDBaseURL     string `mapstructure:"EID_BASE_URL"`
	EIDRPUUID      string `mapstructure:"EID_RP_UUID"`
	EIDRPName      string `mapstructure:"EID_RP_NAME"`
	EIDRPSecret    string `mapstructure:"EID_RP_SECRET"`
	EIDCertLevel   string `mapstructure:"EID_CERT_LEVEL"`
	EIDCallbackURL string `mapstructure:"EID_CALLBACK_URL"`
	EIDDisplayText string `mapstructure:"EID_DISPLAY_TEXT"`

	// PDF гарын үсгийн (PAdES) серверийн БАЙНГЫН Document-Signer гэрчилгээ +
	// ECDSA түлхүүрийн PEM файлын зам. Production-д ЗААВАЛ (fail-closed):
	// эфемер self-signed key нь reproducible/verifiable/revocable бус.
	// development-д хоосон бол sign usecase dev self-signed руу шилжинэ.
	SignSignerCertFile string `mapstructure:"SIGN_SIGNER_CERT_FILE"`
	SignSignerKeyFile  string `mapstructure:"SIGN_SIGNER_KEY_FILE"`

	// Google OAuth — Google account-ийг eID хэрэглэгчид холбох нэвтрэлт.
	// Client secret нь код exchange-д зөвхөн server талд ашиглагдана.
	GoogleClientID     string `mapstructure:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `mapstructure:"GOOGLE_CLIENT_SECRET"`

	BcryptCost int `mapstructure:"BCRYPT_COST"`

	// Gemini AI pipeline (/api/v1/ai/*) — REST-ээр шууд дуудна (SDK-гүй).
	// GEMINI_API_KEY хоосон бол AI endpoint 500 буцаана (template нь
	// AI-гүйгээр boot хийгдэх боломжтой хэвээр). Model/base/voice сонголттой;
	// TTS (text-to-speech) нь тусдаа TTS-чадвартай model хэрэглэдэг.
	GeminiAPIKey   string `mapstructure:"GEMINI_API_KEY"`
	GeminiModel    string `mapstructure:"GEMINI_MODEL"`
	GeminiTTSModel string `mapstructure:"GEMINI_TTS_MODEL"`
	GeminiVoice    string `mapstructure:"GEMINI_VOICE"`
	GeminiAPIBase  string `mapstructure:"GEMINI_API_BASE"`
	// AIScopePrompt нь AI туслахын хамрах хүрээний env fallback — DB-ийн
	// 'scope' prompt давхарга хоосон/уншигдахгүй үед хэрэглэгдэнэ.
	AIScopePrompt string `mapstructure:"AI_SCOPE_PROMPT"`

	// OTel — OTelExporter хоосон бол tracing идэвхгүй болдог (noop
	// provider). Dev орчинд span-уудыг хэвлэхийн тулд "stdout" гэж тохируул,
	// эсвэл production-д OTEL_EXPORTER_OTLP_ENDPOINT-ийг collector / Jaeger /
	// Tempo / Honeycomb / Datadog endpoint руу заасан "otlp" гэж тохируул.
	OTelExporter    string  `mapstructure:"OTEL_EXPORTER"`
	OTelSampleRatio float64 `mapstructure:"OTEL_SAMPLE_RATIO"`

	REDISHost     string `mapstructure:"REDIS_HOST"`
	REDISPassword string `mapstructure:"REDIS_PASS"`
	REDISExpired  int    `mapstructure:"REDIS_EXPIRED"`

	// ObservabilityToken нь production-д /metrics ба /swagger/doc.json
	// операторын endpoint-уудыг хамгаалах bearer token. Хоосон бол эдгээр
	// endpoint production-д 404 буцаана (бүрэн хаалттай). development-д
	// token хамаагүй — үргэлж нээлттэй (ObservabilityGate-г үз).
	ObservabilityToken string `mapstructure:"OBSERVABILITY_TOKEN"`

	AllowedOrigins string `mapstructure:"ALLOWED_ORIGINS"`

	// TrustedProxies нь итгэмжит урвуу proxy-гийн IP/CIDR жагсаалт
	// (таслалаар тусгаарласан, жишээ "10.0.0.0/8,127.0.0.1"). Зөвхөн
	// эдгээрээс ирсэн холболтын X-Forwarded-For-д итгэнэ — эс бөгөөс
	// клиент IP-г RemoteAddr-аас (peer) шууд авна. Хоосон (өгөгдмөл) =
	// XFF-д огт итгэхгүй (rate-limit/audit spoofing-ийн эсрэг fail-safe).
	TrustedProxies string `mapstructure:"TRUSTED_PROXIES"`

	// IntegrationEncKey нь хэрэглэгчийн гуравдагч этгээдийн (Google Drive/Meet,
	// Dropbox) OAuth токеныг storage-д хадгалахын өмнө AES-256-GCM-ээр шифрлэх
	// нууц түлхүүр. SHA-256-аар 32 байт болгон гаргадаг тул дурын урттай байж
	// болно. Хоосон бол сул default — production-д заавал тохируулна.
	IntegrationEncKey string `mapstructure:"INTEGRATION_ENC_KEY"`

	// Gerege Core (core.gerege.mn) — user/organization find. CoreAPIToken нь
	// core.gerege.mn-д хандах урт настай service bearer (server-тал л ашиглана).
	CoreAPIBase  string `mapstructure:"CORE_API_BASE"`
	CoreAPIToken string `mapstructure:"CORE_API_TOKEN"`

	// SuperAdminEmail нь bootstrap: тохируулсан бол boot үед энэ и-мэйлтэй
	// хэрэглэгчийг (байгаа тохиолдолд) super admin (RoleSuperAdmin) болгож
	// ахиулна. Хоосон бол алгасна — super admin-г зөвхөн DB/энэ env-ээр л
	// томилно (API-аар үүсгэдэггүй). Хэрэглэгч эхлээд бүртгүүлсэн байх ёстой.
	SuperAdminEmail string `mapstructure:"SUPERADMIN_EMAIL"`

	// Gerege SSO (sso.gerege.mn, OIDC) — eID-ийн зэрэгцээ 2 дахь нэвтрэлт.
	// ClientID/Secret хоосон бол SSO урсгал inert (Landing дээр товч харагдахгүй).
	// RedirectURI нь SSO client-д бүртгэгдсэн callback (жишээ
	// https://template.gerege.mn/sso/callback) байх ёстой.
	SSOIssuer       string `mapstructure:"SSO_ISSUER"`
	SSOClientID     string `mapstructure:"SSO_CLIENT_ID"`
	SSOClientSecret string `mapstructure:"SSO_CLIENT_SECRET"`
	SSORedirectURI  string `mapstructure:"SSO_REDIRECT_URI"`
	SSOScope        string `mapstructure:"SSO_SCOPE"`
	// SSONativeClientID нь mobile (PKCE, public) урсгалын Hydra client_id —
	// iOS/Android ASWebAuthenticationSession-ийн code-ийг public client-ээр
	// солиход хэрэглэгдэнэ (хоосон бол default template-gerege-mn-ios).
	SSONativeClientID string `mapstructure:"SSO_NATIVE_CLIENT_ID"`
}

// TrustedProxiesList нь TRUSTED_PROXIES-г таслалаар салгаж slice болгоно.
func (c *Config) TrustedProxiesList() []string {
	if c.TrustedProxies == "" {
		return nil
	}
	parts := strings.Split(c.TrustedProxies, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// AllowedOriginsList нь CORS origin-уудыг slice болгож буцаана. Зөвхөн хоосон БА орчин production биш үед ["*"] утгыг анхдагчаар авна.
func (c *Config) AllowedOriginsList() []string {
	if c.AllowedOrigins == "" {
		if c.Environment == constants.EnvironmentProduction {
			return nil
		}
		return []string{"*"}
	}
	parts := strings.Split(c.AllowedOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func InitializeAppConfig() error {
	viper.SetConfigName(".env") // .env файлаас шууд унших боломжийг олгоно
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("internal/config")
	viper.AddConfigPath("/")
	viper.AllowEmptyEnv(true)
	viper.AutomaticEnv()
	// .env файлд байхгүй байж болзошгүй, зөвхөн орчноос ирдэг сонголттой
	// хувьсагчдыг ил BindEnv хийнэ — эс бөгөөс viper.Unmarshal нь AutomaticEnv-
	// ийн утгыг struct руу буулгахгүй (key нь config файл/default-оос
	// бүртгэгдээгүй бол).
	_ = viper.BindEnv("SUPERADMIN_EMAIL")
	// XYP (байгууллагын лавлагаа) креденшл — 12-factor орчинд зөвхөн environment-ээс
	// ирж болзошгүй тул ил bind хийнэ (нууц; .env.example-д хоосон).
	_ = viper.BindEnv("XYP_API_BASE")
	_ = viper.BindEnv("XYP_CLIENT_ID")
	_ = viper.BindEnv("XYP_CLIENT_SECRET")
	// Gerege Space SFTP — нууц; 12-factor орчинд зөвхөн environment-ээс.
	_ = viper.BindEnv("GSPACE_HOST")
	_ = viper.BindEnv("GSPACE_PORT")
	_ = viper.BindEnv("GSPACE_USER")
	_ = viper.BindEnv("GSPACE_PASSWORD")
	_ = viper.BindEnv("GSPACE_BASE_PATH")
	_ = viper.BindEnv("GSPACE_QUOTA_BYTES")
	// .env файл байхгүй байх нь алдаа БИШ — контейнер / 12-factor орчинд
	// тохиргоог зөвхөн environment-ээс уншина. Зөвхөн жинхэнэ задлан унших
	// (parse) алдааг л буцаана.
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return constants.ErrLoadConfig
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		return constants.ErrParseConfig
	}

	applyDefaults()

	// шалгалт
	if AppConfig.Port == 0 || AppConfig.Environment == "" || AppConfig.JWTSecret == "" || AppConfig.JWTExpired == 0 || AppConfig.JWTIssuer == "" || AppConfig.REDISHost == "" || AppConfig.REDISPassword == "" || AppConfig.REDISExpired == 0 || AppConfig.DBPostgreDriver == "" {
		return constants.ErrEmptyVar
	}

	if AppConfig.Port < 1 || AppConfig.Port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535, got %d", AppConfig.Port)
	}
	// ACCESS токены амьдрах хугацаа. Дээд хязгаарыг 24ц болгож бариулав:
	// access токеныг Redis доголдвол revoke хийхэд саатал гарч болзошгүй тул
	// (logout/нууц үг солилтын cutoff шалгалт) урт TTL нь revocation-ийн цонхыг
	// уртасгана. Урт сессийг refresh токен (JWT_REFRESH_EXPIRED, хоногоор)
	// зохицуулна — access богино байх ёстой.
	if AppConfig.JWTExpired < 1 || AppConfig.JWTExpired > 24 {
		return fmt.Errorf("JWT_EXPIRED must be between 1 and 24 hours, got %d", AppConfig.JWTExpired)
	}
	if AppConfig.JWTRefreshExpired < 1 || AppConfig.JWTRefreshExpired > 365 {
		return fmt.Errorf("JWT_REFRESH_EXPIRED must be between 1 and 365 days, got %d", AppConfig.JWTRefreshExpired)
	}
	if len(AppConfig.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d) — HS256 requires 256-bit entropy", len(AppConfig.JWTSecret))
	}
	if AppConfig.REDISExpired < 1 {
		return fmt.Errorf("REDIS_EXPIRED must be at least 1 minute, got %d", AppConfig.REDISExpired)
	}
	if AppConfig.DBMaxOpenConns < 1 || AppConfig.DBMaxIdleConns < 0 || AppConfig.DBMaxIdleConns > AppConfig.DBMaxOpenConns {
		return fmt.Errorf("invalid DB pool config: open=%d idle=%d", AppConfig.DBMaxOpenConns, AppConfig.DBMaxIdleConns)
	}
	if AppConfig.OTPMaxAttempts < 1 {
		return fmt.Errorf("OTP_MAX_ATTEMPTS must be >= 1, got %d", AppConfig.OTPMaxAttempts)
	}
	if AppConfig.BcryptCost < 10 || AppConfig.BcryptCost > 31 {
		return fmt.Errorf("BCRYPT_COST must be between 10 and 31, got %d", AppConfig.BcryptCost)
	}

	switch AppConfig.Environment {
	case constants.EnvironmentDevelopment:
		if AppConfig.DBPostgreDsn == "" {
			return constants.ErrEmptyVar
		}
	case constants.EnvironmentProduction:
		if AppConfig.DBPostgreURL == "" {
			return constants.ErrEmptyVar
		}
		if _, err := url.Parse(AppConfig.DBPostgreURL); err != nil {
			return fmt.Errorf("DB_POSTGRE_URL is not a valid URL: %w", err)
		}
		// secure_system_guide §3.5: production-д DB холболт заавал
		// баталгаажсан TLS-тэй байх ёстой. sslmode=verify-full нь server
		// сертификатыг CA + hostname-ээр шалгаж MITM-аас хамгаална;
		// disable/require/allow/prefer нь сертификатыг шалгахгүй тул
		// production-д хориглоно. (Дотоод сүлжээнд verify-ca-г зөвшөөрнө.)
		if mode := sslModeOf(AppConfig.DBPostgreURL); mode != "verify-full" && mode != "verify-ca" {
			return fmt.Errorf("production DB_POSTGRE_URL must use sslmode=verify-full (got %q) — secure_system_guide §3.5", mode)
		}
		if AppConfig.AllowedOrigins == "" {
			return fmt.Errorf("ALLOWED_ORIGINS must be set in production (comma-separated origins)")
		}
		// Бүх email/SMS OTP нь GeregeCloud Verify-ээр явдаг тул production-д
		// VERIFY_API_KEY заавал шаардлагатай (эс бөгөөс OTP илгээх боломжгүй).
		if AppConfig.VerifyAPIKey == "" {
			return fmt.Errorf("VERIFY_API_KEY must be set in production (GeregeCloud Verify OTP)")
		}
	default:
		return fmt.Errorf("ENVIRONMENT must be 'development' or 'production', got %q", AppConfig.Environment)
	}

	return nil
}

// sslModeOf нь Postgres холболтын мөрөөс sslmode утгыг гаргана —
// URL хэлбэр (postgres://...?sslmode=verify-full) болон keyword/DSN
// хэлбэр (host=... sslmode=verify-full) хоёуланг дэмжинэ. sslmode
// байхгүй бол "" буцаана (libpq нь баталгаажуулдаггүй "prefer"-ийг
// өгөгдмөлөөр авах тул production guard үүнийг найдваргүйд тооцно).
func sslModeOf(conn string) string {
	if u, err := url.Parse(conn); err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		return strings.ToLower(strings.TrimSpace(u.Query().Get("sslmode")))
	}
	for _, field := range strings.Fields(conn) {
		if k, v, ok := strings.Cut(field, "="); ok && strings.EqualFold(strings.TrimSpace(k), "sslmode") {
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	return ""
}

// applyDefaults нь сонголттой config утгуудад зохистой анхдагч утгуудыг олгоно.
func applyDefaults() {
	if AppConfig.DBMaxOpenConns == 0 {
		AppConfig.DBMaxOpenConns = 25
	}
	if AppConfig.DBMaxIdleConns == 0 {
		AppConfig.DBMaxIdleConns = 5
	}
	if AppConfig.DBConnMaxLifeMins == 0 {
		AppConfig.DBConnMaxLifeMins = 15
	}
	if AppConfig.OTPMaxAttempts == 0 {
		AppConfig.OTPMaxAttempts = 5
	}
	if AppConfig.BcryptCost == 0 {
		// 12 ≈ 2026 оны үеийн CPU дээр 100–200 мс. bcrypt.DefaultCost нь
		// түүхэн шалтгаанаар одоо ч 10 хэвээр байгаа; үүнийг нэмэгдүүлэв, гэхдээ
		// буруу тохиргоо сервер тээглэхээс сэргийлж bcrypt-ийн өөрийн дээд
		// хэмжээ (31) хүртэл хязгаарлав.
		AppConfig.BcryptCost = 12
	}
	if AppConfig.JWTRefreshExpired == 0 {
		AppConfig.JWTRefreshExpired = 7
	}
	if AppConfig.GeminiTTSModel == "" {
		AppConfig.GeminiTTSModel = "gemini-2.5-flash-preview-tts"
	}
	// eID RP-ийн өгөгдмөл утгууд. IdP-ийн нийтийн endpoint болон бүртгэгдсэн
	// callback URL тул орчин болгонд найдвартай ажиллана; тохиргоогоор дарж
	// бичиж болно.
	if AppConfig.EIDBaseURL == "" {
		AppConfig.EIDBaseURL = "https://eidmongolia.mn/v3"
	}
	if AppConfig.EIDRPName == "" {
		AppConfig.EIDRPName = "template-web"
	}
	if AppConfig.EIDCertLevel == "" {
		// Нэвтрэлтэд ADVANCED — хамгийн нийцтэй (ADVANCED/QUALIFIED/QSCD бүгдийг
		// хүлээн авна). Гарын үсэгт QUALIFIED/QSCD шаардлагатай бол override хийнэ.
		AppConfig.EIDCertLevel = "ADVANCED"
	}
	if AppConfig.EIDCallbackURL == "" {
		AppConfig.EIDCallbackURL = "https://template.gerege.mn/login/verify"
	}
	if AppConfig.EIDDisplayText == "" {
		AppConfig.EIDDisplayText = "template.gerege.mn"
	}
	if AppConfig.CoreAPIBase == "" {
		AppConfig.CoreAPIBase = "https://core.gerege.mn"
	}
	if AppConfig.XYPAPIBase == "" {
		AppConfig.XYPAPIBase = "https://xyp.gerege.mn"
	}
	if AppConfig.GSpacePort == 0 {
		AppConfig.GSpacePort = 22
	}
	if AppConfig.GSpaceBasePath == "" {
		AppConfig.GSpaceBasePath = "gerege-space"
	}
	if AppConfig.GSpaceQuota == 0 {
		AppConfig.GSpaceQuota = 2 << 20 // 2 MB
	}
	if AppConfig.SSOIssuer == "" {
		AppConfig.SSOIssuer = "https://sso.gerege.mn"
	}
	if AppConfig.SSOScope == "" {
		AppConfig.SSOScope = "openid profile email"
	}
	if AppConfig.SSONativeClientID == "" {
		AppConfig.SSONativeClientID = "template-gerege-mn-ios"
	}
	// OTel-ийн sample ratio нь зөвхөн exporter тохируулагдсан БА оператор
	// ratio-г тодорхой зааж өгөөгүй үед 1.0 утгыг анхдагчаар авна. Exporter
	// байхгүй үед ratio нь хамаагүй (noop provider).
	if AppConfig.OTelSampleRatio == 0 && AppConfig.OTelExporter != "" {
		AppConfig.OTelSampleRatio = 1.0
	}
}

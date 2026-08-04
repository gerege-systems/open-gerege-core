// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package module

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Host нь модульд kernel-ийн олгодог ажиллах орчин. Модуль Register-ийн
// үедээ үүгээр дамжуулан router, DB pool, auth middleware болон хуваалцсан
// service-үүдийг авч ӨӨРИЙН repo → usecase → route wiring-ээ хийнэ —
// server.go модулийн дотоод бүтцийг мэдэхээ больж, модуль server.go-оос
// зөвхөн энэ гэрээгээр хамаарна.
//
// Kernel нь business package-уудыг import хийдэггүй тул модуль-дамнасан
// usecase (rbac, audit г.м.)-ыг Service locator-оор олгоно; модуль boot
// үедээ төрлөө шалгаж (type assertion), таарахгүй бол алдаа буцаана —
// чимээгүй nil-ээр үргэлжлэхгүй. Дараагийн шатанд энэ хэсэг кодоор
// үүсгэгддэг (generated) төрөлжсөн accessor-уудаар солигдоно.
type Host interface {
	// APIRouter — /api дэд модны router; модулиуд /v1/<id> бүлгээ энд суулгана.
	APIRouter() chi.Router
	// Pool — Postgres pool; модуль өөрийн repository-гоо үүсгэхэд хэрэглэнэ.
	Pool() *pgxpool.Pool
	// AuthMiddleware — суурийн JWT танилтын middleware.
	AuthMiddleware() func(http.Handler) http.Handler
	// Service нь нэрээр бүртгэгдсэн хуваалцсан хамаарлыг буцаана
	// (Service* тогтмолуудыг үз). Байхгүй бол ok=false.
	Service(name string) (any, bool)
}

// Хуваалцсан service-үүдийн нэрс. Утгын төрлийг тайлбарт нь заасан —
// модуль assert хийхдээ үүнийг мөрдөнө.
const (
	// ServiceRBAC — rbacuc.Usecase (permission шалгалт).
	ServiceRBAC = "rbac"
	// ServiceAudit — audituc.Usecase (audit бичлэг).
	ServiceAudit = "audit"
	// ServiceUsers — users.Usecase (хэрэглэгчийн lookup).
	ServiceUsers = "users"
	// ServiceWriteRateLimiter — *middlewares.RateLimiter (бичих үйлдлийн
	// нийтлэг хязгаарлагч, server.go-ийн govWriteRateLimiter).
	ServiceWriteRateLimiter = "limiter.write"
	// ServiceRedis — caches.RedisCache.
	ServiceRedis = "redis"
	// ServiceAssets — assetsuc.Usecase (гарын үсэг/тамгын asset).
	ServiceAssets = "assets"
	// ServiceGateway — gatewayuc.Usecase (gateway каталог + телеметр).
	ServiceGateway = "gateway"
	// ServiceGeminiChat — *gemini.Client (чат/embedding model).
	ServiceGeminiChat = "gemini.chat"
	// ServiceGeminiTTS — *gemini.Client (TTS model).
	ServiceGeminiTTS = "gemini.tts"
	// ServiceAuth — authuc.Usecase (танилтын usecase; eID профайл/proxy хэрэглэнэ).
	ServiceAuth = "auth"
	// ServiceOIDCService — *oidcuc.Service (өөрийн OIDC provider-ийн цөм;
	// bearer introspection-д хэрэглэнэ).
	ServiceOIDCService = "oidc.service"
	// ServiceModuleRegistry — *Registry (модулийн бүртгэл + идэвхийн төлөв).
	// platform модуль үүгээр асаах/унтраах үйлдлээ гүйцэтгэнэ. Kernel-ийн
	// төлөв тул зөвхөн platform модульд утга учиртай.
	ServiceModuleRegistry = "module.registry"
	// ServiceModuleStore — platformuc.Store (модулийн төлвийн persistence).
	// Boot дээрх RestoreDisabled нь gate суухаас ӨМНӨ ажиллах ёстой тул
	// store-ыг server.go үүсгэж, admin API-д зориулж энд дамжуулна.
	ServiceModuleStore = "module.store"

	// Модулиудын НИЙТЭЛДЭГ service-үүд (Provide-оор):
	// ServiceAI — aiuc.Usecase (ai модуль нийтэлнэ; admin route хэрэглэнэ).
	ServiceAI = "ai"
	// ServiceSign — signuc.Usecase (sign модуль нийтэлнэ; App.Sign() хэрэглэнэ).
	ServiceSign = "sign"
)

// ServiceProvider — модуль өөрийн usecase-ээ БУЦААЖ нийтлэх боломжтой Host.
// (Жишээ: ai модуль aiUC-гээ нийтэлж, core admin route түүнийг хэрэглэнэ.)
// Boot-ийн нэг goroutine дотор л дуудагдана.
type ServiceProvider interface {
	Provide(name string, svc any)
}

// WorkerRegistrar — модуль background worker бүртгэх боломжтой Host.
// Worker-ууд App.Run-д асаж, shutdown-д context-оор зогсоно; алхам бүр
// 20с timeout-той (нэг гацсан алхам worker-ийг мөнхөд түгжихгүй).
type WorkerRegistrar interface {
	AddWorker(name string, every time.Duration, fn func(context.Context))
}

// ShutdownRegistrar — модуль graceful shutdown-д цэвэрлэгээ бүртгэх Host
// (жишээ: rate limiter-ийн Stop).
type ShutdownRegistrar interface {
	OnShutdown(fn func())
}

// Module нь суулгаж болох модулийн ажиллагааны гэрээ. ID нь Builtin()
// манифестийн ID-тэй тохирно; Register нь route/worker-оо HOST дээр
// суулгана. Register нь модуль ИДЭВХГҮЙ байсан ч дуудагдана (route-ууд
// gate-ээр хаагддаг тул restart-гүйгээр дахин асаах боломжтой байлгана);
// зөвхөн боot-ыг зогсоох ноцтой тохиргооны алдаанд error буцаана.
type Module interface {
	ID() string
	Register(ctx context.Context, host Host) error
}

// ServiceAs нь Service-ийг төрөлжүүлж авах туслах: олдоогүй эсвэл төрөл
// таарахгүй бол тодорхой алдааны мэдээлэлтэй false буцаана.
func ServiceAs[T any](h Host, name string) (T, bool) {
	var zero T
	v, ok := h.Service(name)
	if !ok {
		return zero, false
	}
	t, ok := v.(T)
	if !ok {
		return zero, false
	}
	return t, true
}

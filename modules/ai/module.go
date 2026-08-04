// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ai нь Gemini AI pipeline business модулийн wiring: нэвтэрсэн чат
// (voice/STT/TTS/орчуулга) + нүүр хуудасны нээлттэй чат + мэдлэгийн сангийн
// embedding warm-up. aiUC-гээ ServiceAI нэрээр нийтэлдэг — core admin route
// (/admin/ai/prompts) түүгээр хүрнэ.
//
// Gemini client-үүд kernel талаас (service) ирдэг: хэлний модуль (core site)
// ч мөн орчуулгад ашигладаг тул client нь модулийн өмч биш.
package ai

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	aiuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/ai"
	"github.com/gerege-systems/open-gerege-core/core/config"
	aipostgres "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/postgres/ai"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/gemini"
)

// Module — ai модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "ai" }

// Register нь repo/tools/usecase-уудаа угсарч /v1/ai + /v1/public/ai
// route-уудаа суулгаад, aiUC-гээ нийтэлж, embedding warm-up-аа асаана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	geminiClient, ok := module.ServiceAs[*gemini.Client](host, module.ServiceGeminiChat)
	if !ok {
		return fmt.Errorf("ai: host-д %q service алга", module.ServiceGeminiChat)
	}
	geminiTTSClient, ok := module.ServiceAs[*gemini.Client](host, module.ServiceGeminiTTS)
	if !ok {
		return fmt.Errorf("ai: host-д %q service алга", module.ServiceGeminiTTS)
	}

	aiRepo := aipostgres.NewAIRepository(host.Pool())
	// search_knowledge нь эхлээд семантик (вектор) хайлт хийж, боломжгүй үед
	// түлхүүр үгийн хайлт руу унана — embedder нь chat client-тэй ижил
	// Gemini client (embedding model нь тусдаа).
	aiTools := append(aiuc.DefaultTools(), aiuc.KnowledgeSearchTool(aiRepo, geminiClient))
	// Нүүр хуудасны НЭЭЛТТЭЙ чат (нэвтрэлтгүй) нь ТУСДАА tool багцтай: зөвхөн
	// нийтэд аюулгүй мэдлэгийн сангийн хайлт. Хэрэглэгчийн өгөгдөлд хүрдэг
	// tool нэмэгдвэл нэвтэрсэн чатад л очно — нэргүй зочинд ХЭЗЭЭ Ч биш.
	publicAITools := []aiuc.ToolDef{aiuc.KnowledgeSearchTool(aiRepo, geminiClient)}

	cfg := aiuc.Config{
		Voice:       config.AppConfig.GeminiVoice,
		ScopePrompt: config.AppConfig.AIScopePrompt,
		Embedder:    geminiClient,
	}
	uc := aiuc.NewUsecase(geminiClient, geminiTTSClient, aiRepo, aiTools, cfg)
	// Нээлттэй чатын usecase — ижил model/prompt давхарга, хязгаарлагдмал tool
	// багц. Embedding backfill-ыг зөвхөн uc хийнэ (нэг корпус).
	publicUC := aiuc.NewUsecase(geminiClient, geminiTTSClient, aiRepo, publicAITools, cfg)

	// Rate limiter-ууд (өмнөх server.go-ийн утгууд хэвээр):
	// /ai/* ~20 req/min burst 10 (live орчуулга ~8 chunk/min багтана);
	// нээлттэй чат чангавтар 6/min burst 3; нээлттэй TTS 20/min burst 8
	// (чатын хариу заримдаа хэсэгчлэн уншигддаг).
	aiLimiter := middlewares.NewRateLimiter(rate.Limit(20.0/60.0), 10)
	publicChatLimiter := middlewares.NewRateLimiter(rate.Limit(6.0/60.0), 3)
	publicTTSLimiter := middlewares.NewRateLimiter(rate.Limit(20.0/60.0), 8)
	if sr, ok := host.(module.ShutdownRegistrar); ok {
		sr.OnShutdown(aiLimiter.Stop)
		sr.OnShutdown(publicChatLimiter.Stop)
		sr.OnShutdown(publicTTSLimiter.Stop)
	}

	routes.NewAIRoute(host.APIRouter(), uc, host.AuthMiddleware(), aiLimiter).Routes()
	routes.NewPublicAIRoute(host.APIRouter(), publicUC, publicChatLimiter, publicTTSLimiter).Routes()

	if sp, ok := host.(module.ServiceProvider); ok {
		sp.Provide(module.ServiceAI, uc)
	}

	// Мэдлэгийн сангийн embedding-ийг арын дэвсгэрт гүйцээнэ: migration-аар
	// шинэ/өөрчлөгдсөн бичлэг орж ирвэл эхний ачаалалтад вектор нь
	// тооцоологдоно. Boot-ыг блоклохгүй; алдаа гарвал зөвхөн логдож, хайлт
	// ILIKE-аар ажилласаар байна.
	go func() {
		warmCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		uc.WarmKnowledgeEmbeddings(warmCtx)
	}()
	return nil
}

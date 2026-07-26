// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"template/internal/apperror"
	"template/internal/constants"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/gemini"
	"template/pkg/logger"
)

// fallbackReplies нь Gemini бүх оролдлогын дараа ч амжилтгүй үед хэрэглэгчид
// очих мессеж — хүсэлтийг 5xx болгож унагахын оронд degraded хариу өгнө.
// Хэрэглэгчийн хэлээр (UI-ийн хэл) буцаана; тодорхойгүй бол DefaultLang.
var fallbackReplies = map[string]string{
	"mn": "Уучлаарай, AI үйлчилгээ түр ачаалалтай байна. Та хэсэг хугацааны дараа дахин оролдоно уу.",
	"en": "Sorry, the AI service is temporarily busy. Please try again in a moment.",
	"zh": "抱歉，AI 服务暂时繁忙，请稍后再试。",
	"ru": "Извините, сервис AI временно перегружен. Пожалуйста, попробуйте ещё раз чуть позже.",
}

// fallbackReply нь хэлэнд тохирсон degraded мессежийг буцаана.
func fallbackReply(lang string) string {
	return fallbackReplies[normalizeLang(lang)]
}

const (
	// chatTemperature / chatTopP — чатын sampling. 1.0 нь Gemini-гийн өгөгдмөл
	// бөгөөд ижил асуултад үг сонголтыг хэлбэлзүүлдэг; topP 0.95 нь хэт ховор
	// (утга алдагдуулж болзошгүй) үгсийг хасна.
	chatTemperature = 1.0
	chatTopP        = 0.95

	defaultMaxSteps = 4
	maxHistoryTurns = 20
	// defaultVoice нь Gemini TTS-ийн prebuilt дуу хоолой.
	defaultVoice = "Kore"
)

// Config нь pipeline-ийн тохируулга.
type Config struct {
	// MaxSteps нь function-calling давталтын дээд тоо — model үүнээс олон
	// удаа дараалан tool дуудвал хамгийн сүүлийн текстээр (эсвэл fallback)
	// тасална. 0 бол өгөгдмөл (4).
	MaxSteps int
	// Voice нь TTS-ийн өгөгдмөл prebuilt дуу хоолой. Хоосон бол "Kore".
	Voice string
	// ScopePrompt нь хамрах хүрээний env fallback (AI_SCOPE_PROMPT) —
	// DB-ийн 'scope' давхарга хоосон/уншигдахгүй үед хэрэглэгдэнэ.
	ScopePrompt string
	// Embedder нь мэдлэгийн сангийн вектор хайлт болон backfill-д
	// хэрэглэгдэх Gemini embedding client. nil бол семантик хайлт унтарч,
	// ILIKE (түлхүүр үг) хайлт ажиллана.
	Embedder gemini.Embedder
	// Rand нь [0,n) завсрын санамсаргүй тоо буцаана — хариултын найруулгын
	// хувилбар сонгоход. nil бол math/rand-ийн глобал эх сурвалж; тестэд
	// тогтмол утга өгч детерминистик болгоно.
	Rand func(n int) int
}

type usecase struct {
	client gemini.Generator
	// ttsClient нь TTS-чадвартай model руу заасан тусдаа client — chat
	// model audio гаргадаггүй тул хоёр өөр model хэрэглэнэ.
	ttsClient gemini.Generator
	// repo нь тохируулдаг prompt давхаргууд + мэдлэгийн сангийн gateway.
	// nil байж болно (тест) — тэр үед env/default prompt-ууд хэрэглэгдэнэ.
	repo repointerface.AIRepository
	// embedder нь мэдлэгийн сангийн семантик хайлт болон backfill-д
	// хэрэглэгдэнэ. nil (эсвэл түлхүүргүй) бол хайлт ILIKE рүү унана.
	embedder    gemini.Embedder
	tools       map[string]ToolDef
	decls       []gemini.FunctionDeclaration
	cfg         Config
	promptCache promptCache
}

// NewUsecase нь AI pipeline usecase үүсгэнэ. tools нь model-д зарлагдах ба
// backend дээр гүйцэтгэгдэх функцууд (DefaultTools()-оос эхэлж болно);
// ttsClient нь Speak/Translate-ийн дуут гаралтад, repo нь prompt давхарга +
// мэдлэгийн санд хэрэглэгдэнэ.
func NewUsecase(client, ttsClient gemini.Generator, repo repointerface.AIRepository, tools []ToolDef, cfg Config) Usecase {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = defaultMaxSteps
	}
	if cfg.Voice == "" {
		cfg.Voice = defaultVoice
	}
	if cfg.Rand == nil {
		cfg.Rand = rand.IntN
	}
	byName := make(map[string]ToolDef, len(tools))
	decls := make([]gemini.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		byName[t.Declaration.Name] = t
		decls = append(decls, t.Declaration)
	}
	return &usecase{
		client:    client,
		ttsClient: ttsClient,
		repo:      repo,
		embedder:  cfg.Embedder,
		tools:     byName,
		decls:     decls,
		cfg:       cfg,
	}
}

func (uc *usecase) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	contents := buildContents(req)

	var geminiReq gemini.Request
	// Найруулгын хувилбарыг хүсэлт бүрд сонгоно — ижил асуулт үг үсгээрээ
	// давтагдахгүй байх нэг талын хамгаалалт (нөгөө нь GenerationConfig).
	geminiReq.SystemInstruction = &gemini.Content{
		Parts: []gemini.Part{{Text: uc.systemInstruction(ctx, req.Lang, uc.styleHint(), req.Anonymous)}},
	}
	geminiReq.Contents = contents
	// Sampling — олон янз байдлыг өгнө. Агуулга биш найруулга л хэлбэлзэнэ:
	// баримтыг мэдлэгийн сан болон tool-ийн үр дүн тогтоож өгдөг.
	temperature, topP := chatTemperature, chatTopP
	geminiReq.GenerationConfig = &gemini.GenerationConfig{
		Temperature: &temperature,
		TopP:        &topP,
	}
	if len(uc.decls) > 0 {
		geminiReq.Tools = []gemini.Tool{{FunctionDeclarations: uc.decls}}
	}

	var steps []Step
	for step := 0; step < uc.cfg.MaxSteps; step++ {
		resp, err := uc.client.GenerateContent(ctx, geminiReq)
		if err != nil {
			// Тохиргооны алдаа (түлхүүргүй) нь операторын асуудал — 500
			// болгож log-д бодит шалтгааныг үлдээнэ.
			if errors.Is(err, gemini.ErrNotConfigured) {
				return RunResult{}, apperror.InternalCause(err)
			}
			// Түр зуурын алдаа: retry/backoff client дотор аль хэдийн
			// хийгдсэн — одоо fallback мессежээр намжаана.
			logger.ErrorWithContext(ctx, "ai pipeline: gemini failed, falling back", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryAI,
				"error":                  err.Error(),
				"step":                   step,
			})
			return RunResult{Reply: fallbackReply(req.Lang), Steps: steps, Degraded: true}, nil
		}

		calls := resp.FunctionCalls()
		if len(calls) == 0 {
			reply := resp.Text()
			if reply == "" {
				return RunResult{Reply: fallbackReply(req.Lang), Steps: steps, Degraded: true}, nil
			}
			return RunResult{Reply: reply, Steps: steps}, nil
		}

		// Model-ийн ээлжийг (function дуудлагуудтай нь) conversation-д
		// нэмж, tool бүрийг гүйцэтгээд үр дүнг user ээлжээр буцаана.
		geminiReq.Contents = append(geminiReq.Contents, resp.ModelContent())
		responseParts := make([]gemini.Part, 0, len(calls))
		for _, call := range calls {
			result := uc.executeTool(ctx, call)
			steps = append(steps, Step{Tool: call.Name, Args: call.Args, Result: result})
			responseParts = append(responseParts, gemini.Part{
				FunctionResponse: &gemini.FunctionResponse{Name: call.Name, Response: result},
			})
		}
		geminiReq.Contents = append(geminiReq.Contents, gemini.Content{Role: "user", Parts: responseParts})
	}

	// MaxSteps хүрсэн — model дараалан tool дуудсаар тасрав.
	logger.WarnWithContext(ctx, "ai pipeline: max steps reached without final answer", logger.Fields{
		constants.LoggerCategory: constants.LoggerCategoryAI,
		"max_steps":              uc.cfg.MaxSteps,
	})
	return RunResult{Reply: fallbackReply(req.Lang), Steps: steps, Degraded: true}, nil
}

// executeTool нь нэг function дуудлагыг гүйцэтгэнэ. Алдааг model руу
// {"error": ...} хэлбэрээр буцаадаг — ингэснээр model хэрэглэгчид
// ойлгомжтой тайлбар өгч чадна (pipeline тасрахгүй).
func (uc *usecase) executeTool(ctx context.Context, call gemini.FunctionCall) map[string]any {
	tool, ok := uc.tools[call.Name]
	if !ok {
		return map[string]any{"error": fmt.Sprintf("unknown tool %q", call.Name)}
	}
	result, err := tool.Execute(ctx, call.Args)
	if err != nil {
		logger.ErrorWithContext(ctx, "ai pipeline: tool execution failed", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryAI,
			"tool":                   call.Name,
			"error":                  err.Error(),
		})
		return map[string]any{"error": "tool execution failed"}
	}
	if result == nil {
		result = map[string]any{}
	}
	return result
}

// buildContents нь history + шинэ prompt (текст ба/эсвэл audio)-оос Gemini
// contents угсарна. History-г сүүлийн maxHistoryTurns ээлжээр тайрна
// (token хэмнэлт).
func buildContents(req RunRequest) []gemini.Content {
	history := req.History
	if len(history) > maxHistoryTurns {
		history = history[len(history)-maxHistoryTurns:]
	}
	contents := make([]gemini.Content, 0, len(history)+1)
	for _, t := range history {
		role := t.Role
		if role != "model" {
			role = "user"
		}
		contents = append(contents, gemini.Content{Role: role, Parts: []gemini.Part{{Text: t.Text}}})
	}

	var parts []gemini.Part
	if req.Prompt != "" {
		parts = append(parts, gemini.Part{Text: req.Prompt})
	}
	if req.Audio != nil {
		parts = append(parts, gemini.Part{
			InlineData: &gemini.Blob{MimeType: req.Audio.Mime, Data: req.Audio.Data},
		})
	}
	contents = append(contents, gemini.Content{Role: "user", Parts: parts})
	return contents
}

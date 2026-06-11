// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/internal/constants"
	"template/pkg/gemini"
	"template/pkg/logger"
)

// systemInstruction нь pipeline-ийн тогтмол дүрэм — бүх хариулт Монгол хэлээр.
const systemInstruction = "Чи Gerege платформын туслах AI. БҮХ хариултаа зөвхөн Монгол хэлээр өг. " +
	"Товч, тодорхой, эелдэг хариул. Мэдээлэл хэрэгтэй үед өгөгдсөн функцуудыг ашигла; " +
	"функцийн үр дүнг хэрэглэгчид ойлгомжтой Монгол өгүүлбэрээр тайлбарла. " +
	"Мэдэхгүй зүйл дээр таамаглахгүйгээр мэдэхгүй гэдгээ хэл."

// fallbackReply нь Gemini бүх оролдлогын дараа ч амжилтгүй үед хэрэглэгчид
// очих Монгол мессеж — хүсэлтийг 5xx болгож унагахын оронд degraded хариу өгнө.
const fallbackReply = "Уучлаарай, AI үйлчилгээ түр ачаалалтай байна. Та хэсэг хугацааны дараа дахин оролдоно уу."

const (
	defaultMaxSteps = 4
	maxHistoryTurns = 20
)

// Config нь pipeline-ийн тохируулга.
type Config struct {
	// MaxSteps нь function-calling давталтын дээд тоо — model үүнээс олон
	// удаа дараалан tool дуудвал хамгийн сүүлийн текстээр (эсвэл fallback)
	// тасална. 0 бол өгөгдмөл (4).
	MaxSteps int
}

type usecase struct {
	client gemini.Generator
	tools  map[string]ToolDef
	decls  []gemini.FunctionDeclaration
	cfg    Config
}

// NewUsecase нь AI pipeline usecase үүсгэнэ. tools нь model-д зарлагдах ба
// backend дээр гүйцэтгэгдэх функцууд (DefaultTools()-оос эхэлж болно).
func NewUsecase(client gemini.Generator, tools []ToolDef, cfg Config) Usecase {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = defaultMaxSteps
	}
	byName := make(map[string]ToolDef, len(tools))
	decls := make([]gemini.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		byName[t.Declaration.Name] = t
		decls = append(decls, t.Declaration)
	}
	return &usecase{client: client, tools: byName, decls: decls, cfg: cfg}
}

func (uc *usecase) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	contents := buildContents(req)

	var geminiReq gemini.Request
	geminiReq.SystemInstruction = &gemini.Content{
		Parts: []gemini.Part{{Text: systemInstruction}},
	}
	geminiReq.Contents = contents
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
			return RunResult{Reply: fallbackReply, Steps: steps, Degraded: true}, nil
		}

		calls := resp.FunctionCalls()
		if len(calls) == 0 {
			reply := resp.Text()
			if reply == "" {
				return RunResult{Reply: fallbackReply, Steps: steps, Degraded: true}, nil
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
	return RunResult{Reply: fallbackReply, Steps: steps, Degraded: true}, nil
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

// buildContents нь history + шинэ prompt-оос Gemini contents угсарна.
// History-г сүүлийн maxHistoryTurns ээлжээр тайрна (token хэмнэлт).
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
	contents = append(contents, gemini.Content{Role: "user", Parts: []gemini.Part{{Text: req.Prompt}}})
	return contents
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ai нь /ai/* endpoint-уудыг үйлчилнэ — Gemini-д суурилсан AI
// pipeline-тэй чат харилцаа.
package ai

import (
	"net/http"

	aiuc "template/internal/business/usecases/ai"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
	"template/pkg/validators"
)

type Handler struct {
	usecase aiuc.Usecase
}

func NewHandler(usecase aiuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// Chat godoc
// @Summary      AI туслахтай чатлах (текст/дуут мессеж)
// @Description  Хэрэглэгчийн мессежийг (текст эсвэл audio — дуут мессежийг AI шууд ойлгоно) Gemini AI pipeline-аар боловсруулж Монгол хариулт буцаана. AI шаардлагатай үед backend tool-уудыг (function calling) ашигладаг; гүйцэтгэсэн алхмууд steps талбарт ил гарна. AI үйлчилгээ түр унавал degraded=true + fallback мессеж буцаана (5xx биш).
// @Tags         ai
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.AIChatRequest  true  "Chat message (text and/or audio) + optional history"
// @Success      200      {object}  v1.BaseResponse{data=responses.AIChatResponse}  "AI reply"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body / missing message and audio"
// @Failure      401      {object}  v1.BaseResponse  "Missing/invalid token"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      429      {object}  v1.BaseResponse  "Rate limit exceeded"
// @Router       /ai/chat [post]
func (h Handler) Chat(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "ai"
		funcName       = "Chat"
		fileName       = "ai_handler.go"
	)
	ctx := r.Context()

	var req requests.AIChatRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "Chat: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if req.Message == "" && req.Audio == nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "message or audio is required")
	}

	history := make([]aiuc.Turn, 0, len(req.History))
	for _, t := range req.History {
		history = append(history, aiuc.Turn{Role: t.Role, Text: t.Text})
	}

	result, err := h.usecase.Run(ctx, aiuc.RunRequest{
		Prompt:  req.Message,
		Audio:   toAudio(req.Audio),
		History: history,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "ai reply generated", responses.FromAIRunResult(result))
}

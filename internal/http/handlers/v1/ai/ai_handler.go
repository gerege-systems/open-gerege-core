// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ai нь /ai/* endpoint-уудыг үйлчилнэ — Gemini-д суурилсан AI
// pipeline-тэй чат харилцаа.
package ai

import (
	"net/http"
	"strings"

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

// PublicChat godoc
// @Summary      Нээлттэй AI туслах (нэвтрэлтгүй)
// @Description  Нүүр хуудасны чат виджетэд зориулсан НЭВТРЭЛТГҮЙ чат. Текст эсвэл богино дуут мессеж (push-to-talk, ~250 KB base64 ≈ 15 сек), мессеж 1000 тэмдэгт, түүх 6 ээлж. Дуут мессежийг эхлээд STT-ээр текст болгож, хариунд transcript талбараар буцаана; яриа таниагүй бол degraded=true, хоосон reply. Туслах нь платформын мэдлэгийн санд тулгуурлан хариулах бөгөөд хэрэглэгчийн бүртгэлийн өгөгдөлд ХАНДАХГҮЙ (тусдаа tool багц + нэмэлт guardrail). IP тус бүрт минутанд ~6 хүсэлт.
// @Tags         ai
// @Accept       json
// @Produce      json
// @Param        request  body      requests.AIPublicChatRequest  true  "Chat message + optional short history"
// @Success      200      {object}  v1.BaseResponse{data=responses.AIChatResponse}  "AI reply"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      429      {object}  v1.BaseResponse  "Rate limit exceeded"
// @Router       /public/ai/chat [post]
func (h Handler) PublicChat(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var req requests.AIPublicChatRequest
	if err := v1.DecodeBody(r, &req); err != nil {
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

	// Дуут мессежийг эхлээд ТЕКСТ болгоно (STT), дараа нь текстээр чатлана.
	// Audio-г чат model руу шууд өгч ч болно (мультимодаль) — гэхдээ тэгвэл
	// хэрэглэгч юу сонсогдсоныг хардаггүй, ярианы түүх ч «дуут мессеж» гэсэн
	// орлуулагчаар дүүрдэг. Хуулбарыг буцаана: UI бөмбөлөгт харуулна.
	prompt := req.Message
	var transcript string
	if req.Audio != nil {
		stt, sttErr := h.usecase.Transcribe(ctx, aiuc.TranscribeRequest{
			Audio: aiuc.Audio{Mime: req.Audio.Mime, Data: req.Audio.Data},
			// Зочид платформын тухай асуудаг тул нэр томьёог сануулна —
			// «нэвтрэх»-ийг «нэрших» гэх мэт андуурлыг багасгана.
			Vocabulary: aiuc.PlatformVocabulary,
		})
		if sttErr != nil {
			return v1.RespondWithError(w, r, sttErr)
		}
		transcript = strings.TrimSpace(stt.Text)
		if transcript == "" {
			// Чимээгүй / яриа таниагүй — Gemini-г дахин зовоохгүйгээр буцаана.
			// Клиент нь хоосон хуулбарыг хараад өөрийн хэл дээрх сануулга
			// харуулна (сервер талд хэлний мессеж давхардуулах шаардлагагүй).
			return v1.NewSuccessResponse(w, r, http.StatusOK, "no speech detected",
				responses.AIChatResponse{Degraded: true})
		}
		prompt = transcript
	}

	// Anonymous=true нь system prompt дээр зочны хоригийг нэмнэ; usecase нь
	// нийтэд аюулгүй tool багцтайгаар холбогдсон (server.go).
	result, err := h.usecase.Run(ctx, aiuc.RunRequest{
		Prompt:    prompt,
		History:   history,
		Lang:      req.Lang,
		Anonymous: true,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}

	out := responses.FromAIRunResult(result)
	out.Transcript = transcript
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ai reply generated", out)
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
		Lang:    req.Lang,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "ai reply generated", responses.FromAIRunResult(result))
}

// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"encoding/json"
	"fmt"
	"net/http"

	aiuc "template/internal/business/usecases/ai"
	"template/internal/constants"
	"template/internal/http/datatransfers/requests"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
	"template/pkg/validators"
)

// PublicChatStream godoc
// @Summary      Нээлттэй AI чат — урсгалаар (SSE)
// @Description  PublicChat-тай ижил боловч хариултыг хэсэг хэсгээр нь Server-Sent Events-ээр илгээнэ (text/event-stream). Event-үүд: `transcript` (дуут мессежийн текст хуулбар — НЭГ дуудалтаар хариулттай хамт гардаг, тусдаа STT дуудлага байхгүй), `delta` (хариултын дараагийн хэсэг), `reset` (өмнөх delta-г хаях), `done` (төгсгөл; degraded талбартай), `error`. Хязгаарууд PublicChat-тай ижил.
// @Tags         ai
// @Accept       json
// @Produce      text/event-stream
// @Param        request  body      requests.AIPublicChatRequest  true  "Chat message (text and/or audio) + optional short history"
// @Success      200      {string}  string  "SSE stream"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body / message and audio both missing"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      429      {object}  v1.BaseResponse  "Rate limit exceeded"
// @Router       /public/ai/chat/stream [post]
func (h Handler) PublicChatStream(w http.ResponseWriter, r *http.Request) error {
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

	// Урсгал эхлэхээс өмнө flush хийх боломжтой эсэхийг шалгана — боломжгүй
	// бол урсгал бус хариу руу уналт хийх нь хэрэглэгчид илүү дээр.
	flusher, ok := w.(http.Flusher)
	if !ok {
		return h.PublicChat(w, r)
	}

	history := make([]aiuc.Turn, 0, len(req.History))
	for _, t := range req.History {
		history = append(history, aiuc.Turn{Role: t.Role, Text: t.Text})
	}
	var audio *aiuc.Audio
	if req.Audio != nil {
		audio = &aiuc.Audio{Mime: req.Audio.Mime, Data: req.Audio.Data}
	}

	h.writeSSEHeaders(w)
	flusher.Flush()

	send := func(event string, payload any) error {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	result, err := h.usecase.RunStream(ctx, aiuc.RunRequest{
		Prompt:    req.Message,
		Audio:     audio,
		History:   history,
		Lang:      req.Lang,
		Anonymous: true,
	}, func(ev aiuc.StreamEvent) error {
		switch {
		case ev.Reset:
			return send("reset", map[string]any{})
		case ev.Transcript != "":
			return send("transcript", map[string]any{"text": ev.Transcript})
		case ev.Delta != "":
			return send("delta", map[string]any{"text": ev.Delta})
		}
		return nil
	})
	if err != nil {
		// Толгойнууд аль хэдийн явсан тул HTTP статус солих боломжгүй —
		// алдааг урсгалын event болгож өгнө (клиент ойлгомжтой мессеж гаргана).
		logger.ErrorWithContext(ctx, "ai: public chat stream failed", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryAI,
			"error":                  err.Error(),
		})
		_ = send("error", map[string]any{"message": "ai stream failed"})
		return nil
	}

	return send("done", map[string]any{
		"degraded":   result.Degraded,
		"transcript": result.Transcript,
	})
}

// writeSSEHeaders нь урсгалыг эцэс хүртэл буферлүүлэхгүй байх толгойнуудыг
// тавина. X-Accel-Buffering: no нь урд талын nginx-ийн proxy_buffering-ийг
// ЭНЭ хариуны хувьд унтраана — эс бөгөөс nginx бүх урсгалыг цуглуулаад нэг
// дор гаргадаг тул streaming утгаа алдана (nginx.conf засах эрх хэрэггүй).
func (h Handler) writeSSEHeaders(w http.ResponseWriter) {
	head := w.Header()
	head.Set("Content-Type", "text/event-stream; charset=utf-8")
	head.Set("Cache-Control", "no-cache, no-transform")
	head.Set("Connection", "keep-alive")
	head.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"net/http"

	aiuc "template/internal/business/usecases/ai"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

// toAudio нь DTO audio-г usecase төрөл рүү буулгана.
func toAudio(a *requests.AIAudio) *aiuc.Audio {
	if a == nil {
		return nil
	}
	return &aiuc.Audio{Mime: a.Mime, Data: a.Data}
}

// Transcribe godoc
// @Summary      Яриаг текст болгох (STT)
// @Description  Base64 кодлогдсон audio (webm/ogg/wav/mp3 г.м.)-г Gemini-ээр текст болгоно. Яриа илрээгүй бол хоосон text буцаана.
// @Tags         ai
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.AISTTRequest  true  "Audio (base64)"
// @Success      200      {object}  v1.BaseResponse{data=responses.AISTTResponse}  "Transcript"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body"
// @Failure      401      {object}  v1.BaseResponse  "Missing/invalid token"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      429      {object}  v1.BaseResponse  "Rate limit exceeded"
// @Router       /ai/stt [post]
func (h Handler) Transcribe(w http.ResponseWriter, r *http.Request) error {
	var req requests.AISTTRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.Transcribe(r.Context(), aiuc.TranscribeRequest{
		Audio: aiuc.Audio{Mime: req.Audio.Mime, Data: req.Audio.Data},
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "audio transcribed", responses.AISTTResponse{Text: result.Text})
}

// Speak godoc
// @Summary      Текстийг яриа болгох (TTS)
// @Description  Текстийг Gemini TTS model-ээр дуут (audio/wav, base64) болгоно. voice нь сонголттой prebuilt дуу хоолой (өгөгдмөл: Kore).
// @Tags         ai
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.AITTSRequest  true  "Text + optional voice"
// @Success      200      {object}  v1.BaseResponse{data=responses.AIAudioOut}  "WAV audio (base64)"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body"
// @Failure      401      {object}  v1.BaseResponse  "Missing/invalid token"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      429      {object}  v1.BaseResponse  "Rate limit exceeded"
// @Router       /ai/tts [post]
func (h Handler) Speak(w http.ResponseWriter, r *http.Request) error {
	var req requests.AITTSRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.Speak(r.Context(), aiuc.SpeakRequest{Text: req.Text, Voice: req.Voice})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "speech generated", responses.AIAudioOut{Mime: result.Mime, Data: result.Data})
}

// Translate godoc
// @Summary      Шууд (live) орчуулга
// @Description  Текст эсвэл audio-г зорилтот хэл рүү орчуулна. Audio өгвөл эхлээд STT хийгээд орчуулдаг; speak=true бол орчуулгын дуут (TTS) хувилбарыг хамт буцаана. Live орчуулга = богино audio chunk-уудыг энэ endpoint руу дараалан илгээх урсгал. Чимээгүй chunk-д хоосон үр дүн буцаана.
// @Tags         ai
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.AITranslateRequest  true  "Text or audio + target_lang"
// @Success      200      {object}  v1.BaseResponse{data=responses.AITranslateResponse}  "Translation"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body / missing text and audio"
// @Failure      401      {object}  v1.BaseResponse  "Missing/invalid token"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      429      {object}  v1.BaseResponse  "Rate limit exceeded"
// @Router       /ai/translate [post]
func (h Handler) Translate(w http.ResponseWriter, r *http.Request) error {
	var req requests.AITranslateRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if req.Text == "" && req.Audio == nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "text or audio is required")
	}

	result, err := h.usecase.Translate(r.Context(), aiuc.TranslateRequest{
		Text:       req.Text,
		Audio:      toAudio(req.Audio),
		TargetLang: req.TargetLang,
		Speak:      req.Speak,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "translated", responses.FromAITranslateResult(result))
}

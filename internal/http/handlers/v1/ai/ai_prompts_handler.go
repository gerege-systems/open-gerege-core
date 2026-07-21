// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"net/http"

	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

// ListPrompts godoc
// @Summary      AI prompt давхаргуудыг жагсаах
// @Description  Тохируулдаг prompt давхаргуудыг (scope — хамрах хүрээ, instructions — нэмэлт заавар) буцаана. Suurь (base) дүрэм кодод хатуу бичигдсэн тул энд харагдахгүй, өөрчлөгдөхгүй.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=[]responses.AIPromptResponse}  "Prompt layers"
// @Failure      401  {object}  v1.BaseResponse  "Missing/invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Missing settings.manage permission"
// @Router       /admin/ai/prompts [get]
func (h Handler) ListPrompts(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListPrompts(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "prompts fetched", responses.ToAIPromptList(list))
}

// SetPrompt godoc
// @Summary      AI prompt давхаргыг шинэчлэх
// @Description  Нэг давхаргын (scope | instructions) агуулгыг солино. Өөрчлөлт нэн даруй үйлчилнэ (prompt кэш хүчингүй болдог). AI зөвхөн scope-д заасан хүрээнд ажиллана.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        key      path      string                           true  "Prompt key (scope | instructions)"
// @Param        request  body      requests.AIPromptUpdateRequest   true  "New content"
// @Success      200      {object}  v1.BaseResponse  "Updated"
// @Failure      400      {object}  v1.BaseResponse  "Unknown key / malformed body"
// @Failure      401      {object}  v1.BaseResponse  "Missing/invalid token"
// @Failure      403      {object}  v1.BaseResponse  "Missing settings.manage permission"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Router       /admin/ai/prompts/{key} [put]
func (h Handler) SetPrompt(w http.ResponseWriter, r *http.Request) error {
	key := chi.URLParam(r, "key")

	var req requests.AIPromptUpdateRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	if err := h.usecase.SetPrompt(r.Context(), key, req.Content); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "prompt updated", nil)
}

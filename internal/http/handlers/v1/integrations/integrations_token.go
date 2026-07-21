// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"net/http"

	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"

	"github.com/go-chi/chi/v5"
)

// GetToken godoc
// @Summary      Провайдерын токеныг авах (server-тал, BFF)
// @Description  Тухайн хэрэглэгчийн нэг провайдерын decrypt хийсэн токеныг буцаана. Зөвхөн BFF дотроос дуудагдана — токен browser руу гарахгүй.
// @Tags         integrations
// @Produce      json
// @Param        provider  path  string  true  "Provider id"
// @Success      200  {object}  v1.BaseResponse{data=responses.IntegrationTokenResponse}
// @Failure      401  {object}  v1.BaseResponse
// @Failure      404  {object}  v1.BaseResponse  "Not connected"
// @Security     BearerAuth
// @Router       /integrations/{provider}/token [get]
func (h Handler) GetToken(w http.ResponseWriter, r *http.Request) error {
	current, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	tok, err := h.usecase.Token(r.Context(), current.ID, chi.URLParam(r, "provider"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "token fetched", responses.FromTokenData(tok))
}

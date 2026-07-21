// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"net/http"

	httpauth "template/internal/http/auth"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"

	"github.com/go-chi/chi/v5"
)

// Disconnect godoc
// @Summary      Интеграцийг салгах
// @Description  Тухайн хэрэглэгчийн нэг провайдерын хадгалсан токеныг устгана (idempotent).
// @Tags         integrations
// @Produce      json
// @Param        provider  path      string  true  "Provider id (google-drive, dropbox, google-meet)"
// @Success      200       {object}  v1.BaseResponse  "Disconnected"
// @Failure      400       {object}  v1.BaseResponse  "Unknown provider"
// @Failure      401       {object}  v1.BaseResponse  "Unauthenticated"
// @Security     BearerAuth
// @Router       /integrations/{provider} [delete]
func (h Handler) Disconnect(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "integrations"
		funcName       = "Disconnect"
		fileName       = "integrations_disconnect.go"
	)
	ctx := r.Context()
	current, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}

	provider := chi.URLParam(r, "provider")
	if err := h.usecase.Disconnect(ctx, current.ID, provider); err != nil {
		logger.ErrorWithContext(ctx, "Disconnect failed in controller", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "integration disconnected", map[string]any{"provider": provider})
}

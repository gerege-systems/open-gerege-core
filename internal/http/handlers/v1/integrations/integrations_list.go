// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"net/http"

	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
)

// List godoc
// @Summary      Холбосон интеграцийг жагсаах
// @Description  Тухайн хэрэглэгчийн холбосон провайдеруудыг буцаана (токенгүй — зөвхөн provider + хугацаа).
// @Tags         integrations
// @Produce      json
// @Success      200  {object}  v1.BaseResponse{data=[]responses.IntegrationResponse}  "Connected providers"
// @Failure      401  {object}  v1.BaseResponse  "Unauthenticated"
// @Security     BearerAuth
// @Router       /integrations [get]
func (h Handler) List(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "integrations"
		funcName       = "List"
		fileName       = "integrations_list.go"
	)
	ctx := r.Context()
	current, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}

	list, err := h.usecase.List(ctx, current.ID)
	if err != nil {
		logger.ErrorWithContext(ctx, "List integrations failed in controller", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "integrations fetched", responses.FromConnectedProviders(list))
}

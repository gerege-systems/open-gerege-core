// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"net/http"
	"time"

	integrationsuc "template/internal/business/usecases/integrations"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
	"template/pkg/validators"
)

// Connect godoc
// @Summary      Интеграцийн токеныг хадгалах
// @Description  Гуравдагч этгээдийн (Google Drive/Meet, Dropbox) OAuth токеныг тухайн хэрэглэгчид холбон шифрлэн хадгална. Frontend BFF нь token exchange-ийн дараа дуудна. Давхцвал шинэчилнэ.
// @Tags         integrations
// @Accept       json
// @Produce      json
// @Param        request  body      requests.ConnectIntegrationRequest  true  "Provider tokens"
// @Success      200      {object}  v1.BaseResponse  "Stored"
// @Failure      400      {object}  v1.BaseResponse  "Malformed body / unknown provider"
// @Failure      401      {object}  v1.BaseResponse  "Unauthenticated"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Security     BearerAuth
// @Router       /integrations [post]
func (h Handler) Connect(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "integrations"
		funcName       = "Connect"
		fileName       = "integrations_connect.go"
	)
	ctx := r.Context()
	current, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}

	var req requests.ConnectIntegrationRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "Connect: invalid request body", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	var expiresAt *time.Time
	if req.ExpiresAtMs > 0 {
		t := time.UnixMilli(req.ExpiresAtMs).UTC()
		expiresAt = &t
	}

	if _, err := h.usecase.Connect(ctx, integrationsuc.ConnectRequest{
		UserID:       current.ID,
		Provider:     req.Provider,
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		ExpiresAt:    expiresAt,
	}); err != nil {
		logger.ErrorWithContext(ctx, "Connect failed in controller", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "integration connected", map[string]any{"provider": req.Provider})
}

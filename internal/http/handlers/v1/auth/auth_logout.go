// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"net/http"

	authuc "template/internal/business/usecases/auth"
	"template/internal/http/datatransfers/requests"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/audit"
	"template/pkg/logger"
	"template/pkg/validators"
)

// Logout godoc
// @Summary      Токенуудыг хүчингүй болгох
// @Description  /auth/refresh татгалзахын тулд refresh-токены jti-г Redis-ээс устгана. access_token өгөгдсөн бол түүний jti-г deny-list-д нэмж access токеныг ч мөн шууд хүчингүй болгоно (өгөөгүй бол байгалийн хугацаа дуустлаа хүчинтэй үлдэнэ).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.LogoutRequest  true  "Refresh token to revoke + optional access token to deny"
// @Success      200      {object}  v1.BaseResponse  "Logged out"
// @Failure      401      {object}  v1.BaseResponse  "Refresh token invalid"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Router       /auth/logout [post]
func (h Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "Logout"
		fileName       = "auth_logout.go"
	)
	ctx := r.Context()
	var req requests.LogoutRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "Logout: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "Logout: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"request": logger.Fields{
				"has_refresh_token": req.RefreshToken != "",
			},
		})
		return v1.RespondWithError(w, r, err)
	}

	if err := h.usecase.Logout(ctx, authuc.LogoutRequest{RefreshToken: req.RefreshToken, AccessToken: req.AccessToken}); err != nil {
		logger.ErrorWithContext(ctx, "Logout failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventLogout
	ev.Success = true
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusOK, "logout success", nil)
}

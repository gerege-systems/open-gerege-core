// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"net/http"

	authuc "template/internal/business/usecases/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/audit"
	"template/pkg/logger"
	"template/pkg/validators"
)

// Refresh godoc
// @Summary      Refresh токеныг сэлгэж, шинэ хос буцаах
// @Description  Өгөгдсөн refresh токеныг шалгаж, шинэ access+refresh хос үүсгэж, хуучин jti-г Redis-д хүчингүй болгоно. Аль хэдийн сэлгэгдсэн токеныг дахин тоглуулах нь 401 буцаана.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.RefreshRequest  true  "Refresh token"
// @Success      200      {object}  v1.BaseResponse{data=responses.UserResponse}  "New token pair"
// @Failure      401      {object}  v1.BaseResponse  "Refresh token invalid, expired, or already revoked"
// @Failure      403      {object}  v1.BaseResponse  "Account no longer active"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Router       /auth/refresh [post]
func (h Handler) Refresh(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "Refresh"
		fileName       = "auth_refresh.go"
	)
	ctx := r.Context()
	var req requests.RefreshRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "Refresh: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "Refresh: validation error", logger.Fields{
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

	result, err := h.usecase.Refresh(ctx, authuc.RefreshRequest{RefreshToken: req.RefreshToken})
	if err != nil {
		ev := auditFromRequest(r)
		ev.Type = audit.EventRefreshFail
		ev.Reason = err.Error()
		audit.Record(ev)
		logger.ErrorWithContext(ctx, "Refresh failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventRefreshOK
	ev.Success = true
	ev.UserID = result.User.ID
	ev.Email = result.User.Email
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusOK, "token refreshed", responses.FromLoginResponse(result))
}

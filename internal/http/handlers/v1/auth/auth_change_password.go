// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"net/http"

	authuc "template/internal/business/usecases/auth"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/audit"
	"template/pkg/logger"
	"template/pkg/validators"
)

// ChangePassword godoc
// @Summary      Баталгаажуулагдсан хэрэглэгчийн нууц үгийг солих
// @Description  Одоогийн нууц үгийг шалгаж, шинээр сольж, цуцлалтын хязгаарыг тэмдэглэдэг тул одооноос өмнө олгогдсон refresh токенууд татгалзагдана.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.ChangePasswordRequest  true  "Current + new password"
// @Success      200  {object}  v1.BaseResponse  "Password changed"
// @Failure      400  {object}  v1.BaseResponse  "Malformed JSON body"
// @Failure      401  {object}  v1.BaseResponse  "Current password incorrect"
// @Failure      422  {object}  v1.BaseResponse  "Validation error"
// @Router       /auth/password/change [put]
func (h Handler) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "ChangePassword"
		fileName       = "auth_change_password.go"
	)
	ctx := r.Context()
	current, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		logger.WarnWithContext(ctx, "ChangePassword: not authenticated", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	var req requests.ChangePasswordRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "ChangePassword: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "ChangePassword: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"request": logger.Fields{
				"user_id":              current.ID,
				"has_current_password": req.CurrentPassword != "",
				"has_new_password":     req.NewPassword != "",
			},
		})
		return v1.RespondWithError(w, r, err)
	}

	if err := h.usecase.ChangePassword(ctx, authuc.ChangePasswordRequest{
		UserID:          current.ID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); err != nil {
		ev := auditFromRequest(r)
		ev.Type = audit.EventPasswordChangeFail
		ev.UserID = current.ID
		ev.Email = current.Email
		ev.Reason = err.Error()
		audit.Record(ev)
		logger.ErrorWithContext(ctx, "ChangePassword failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"user_id":    current.ID,
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventPasswordChangeOK
	ev.Success = true
	ev.UserID = current.ID
	ev.Email = current.Email
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusOK, "password changed", nil)
}

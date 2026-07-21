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

// ResetPassword godoc
// @Summary      OTP кодоор шинэ нууц үг тогтоох
// @Description  ForgotPassword-ийн email-ээр илгээсэн OTP кодыг GeregeCloud Verify-ээр баталгаажуулж, шинэ нууц үг тогтоож, токены цуцлалтын хязгаарыг урагшлуулна.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.ResetPasswordRequest  true  "Email + OTP code + new password"
// @Success      200  {object}  v1.BaseResponse  "Password reset"
// @Failure      400  {object}  v1.BaseResponse  "Malformed JSON body"
// @Failure      401  {object}  v1.BaseResponse  "Reset code invalid or expired"
// @Failure      422  {object}  v1.BaseResponse  "Validation error"
// @Router       /auth/password/reset [post]
func (h Handler) ResetPassword(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "ResetPassword"
		fileName       = "auth_reset_password.go"
	)
	ctx := r.Context()
	var req requests.ResetPasswordRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "ResetPassword: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "ResetPassword: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"request": logger.Fields{
				"email":            req.Email,
				"has_code":         req.Code != "",
				"has_new_password": req.NewPassword != "",
			},
		})
		return v1.RespondWithError(w, r, err)
	}

	if err := h.usecase.ResetPassword(ctx, authuc.ResetPasswordRequest{
		Email:       req.Email,
		Code:        req.Code,
		NewPassword: req.NewPassword,
	}); err != nil {
		ev := auditFromRequest(r)
		ev.Type = audit.EventPasswordResetFail
		ev.Reason = err.Error()
		audit.Record(ev)
		logger.ErrorWithContext(ctx, "ResetPassword failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventPasswordResetOK
	ev.Success = true
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusOK, "password reset", nil)
}

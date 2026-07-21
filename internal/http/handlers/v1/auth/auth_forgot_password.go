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

// ForgotPassword godoc
// @Summary      Нууц үг сэргээх OTP код илгээх
// @Description  Хаяг руу нэг удаагийн OTP кодыг (GeregeCloud Verify) и-мэйлээр илгээнэ. Хэрэглэгчийг тоолохоос сэргийлэхийн тулд email бүртгэлгүй байсан ч үргэлж 200 буцаана.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.ForgotPasswordRequest  true  "Email address"
// @Success      200  {object}  v1.BaseResponse  "Reset email queued (or email not registered — same response either way)"
// @Failure      400  {object}  v1.BaseResponse  "Malformed JSON body"
// @Failure      422  {object}  v1.BaseResponse  "Validation error"
// @Router       /auth/password/forgot [post]
func (h Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "ForgotPassword"
		fileName       = "auth_forgot_password.go"
	)
	ctx := r.Context()
	var req requests.ForgotPasswordRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "ForgotPassword: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "ForgotPassword: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"request": logger.Fields{
				"email": req.Email,
			},
		})
		return v1.RespondWithError(w, r, err)
	}

	if err := h.usecase.ForgotPassword(ctx, authuc.ForgotPasswordRequest{Email: req.Email}); err != nil {
		ev := auditFromRequest(r)
		ev.Type = audit.EventPasswordForgotFail
		ev.Email = req.Email
		ev.Reason = err.Error()
		audit.Record(ev)
		logger.ErrorWithContext(ctx, "ForgotPassword failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"email":      req.Email,
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventPasswordForgotOK
	ev.Success = true
	ev.Email = req.Email
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusOK, "if the email is registered, a reset code has been sent", nil)
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"fmt"
	"net/http"

	authuc "template/internal/business/usecases/auth"
	"template/internal/http/datatransfers/requests"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/audit"
	"template/pkg/logger"
	"template/pkg/validators"
)

// SendOTP godoc
// @Summary      Хэрэглэгчийн email рүү OTP код илгээх
// @Description  GeregeCloud Verify API-аар OTP илгээж, буцаасан request_id-г TTL-тэйгээр Redis-д хадгална. Кодын үүсгэлт/хадгалалт/илгээлтийг Verify үйлчилгээ хариуцна.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SendOTPRequest  true  "Email to send OTP to"
// @Success      200      {object}  v1.BaseResponse  "OTP enqueued"
// @Failure      404      {object}  v1.BaseResponse  "Email not registered"
// @Failure      400      {object}  v1.BaseResponse  "Account already activated"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      500      {object}  v1.BaseResponse  "Failed to enqueue mail"
// @Router       /auth/send-otp [post]
func (h Handler) SendOTP(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "SendOTP"
		fileName       = "auth_send_otp.go"
	)
	ctx := r.Context()
	var req requests.SendOTPRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "SendOTP: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "SendOTP: validation error", logger.Fields{
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

	if err := h.usecase.SendOTP(ctx, authuc.SendOTPRequest{Email: req.Email}); err != nil {
		logger.ErrorWithContext(ctx, "SendOTP failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"email":      req.Email,
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventOTPSent
	ev.Success = true
	ev.Email = req.Email
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusOK, fmt.Sprintf("otp code has been send to %s", req.Email), nil)
}

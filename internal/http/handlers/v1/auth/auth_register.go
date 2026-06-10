// Gerege Template Version 27.0
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

// Register godoc
// @Summary      Шинэ хэрэглэгч бүртгэх
// @Description  Хэрэглэгчийн бүртгэлийг идэвхгүй төлөвт үүсгэнэ. Дуудагч нь /auth/login амжилттай болохоос өмнө бүртгэлийг идэвхжүүлэхийн тулд /auth/send-otp + /auth/verify-otp-г дагах ёстой.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.RegisterRequest  true  "Registration payload"
// @Success      201      {object}  v1.BaseResponse{data=responses.UserResponse}  "User created"
// @Failure      400      {object}  v1.BaseResponse                                "Malformed JSON body"
// @Failure      409      {object}  v1.BaseResponse                                "Email or username already in use"
// @Failure      422      {object}  v1.BaseResponse                                "Validation error (per-field detail in data.errors)"
// @Failure      500      {object}  v1.BaseResponse                                "Internal error"
// @Router       /auth/register [post]
func (h Handler) Register(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "Register"
		fileName       = "auth_register.go"
	)
	ctx := r.Context()
	var req requests.RegisterRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "Register: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "Register: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"request": logger.Fields{
				"username":     req.Username,
				"email":        req.Email,
				"has_password": req.Password != "",
			},
		})
		return v1.RespondWithError(w, r, err)
	}

	regResp, err := h.usecase.Register(ctx, authuc.RegisterRequest{User: req.ToV1Domain()})
	user := regResp.User
	if err != nil {
		ev := auditFromRequest(r)
		ev.Type = audit.EventRegister
		ev.Email = req.Email
		ev.Reason = err.Error()
		audit.Record(ev)
		logger.ErrorWithContext(ctx, "Register failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"email":      req.Email,
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventRegister
	ev.Success = true
	ev.UserID = user.ID
	ev.Email = user.Email
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusCreated, "registration user success", map[string]interface{}{
		"user": responses.FromV1Domain(user),
	})
}

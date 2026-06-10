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

// Login godoc
// @Summary      Баталгаажуулж токен хос олгох
// @Description  Access токен (богино TTL) болон refresh токен (урт TTL) буцаана. Хэрэглэгчийг тоолохоос сэргийлэхийн тулд буруу нууц үг болон тодорхойгүй email ижил хугацаа зарцуулна.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.LoginRequest  true  "Login credentials"
// @Success      200      {object}  v1.BaseResponse{data=responses.UserResponse}  "Tokens issued"
// @Failure      400      {object}  v1.BaseResponse                                "Malformed JSON body"
// @Failure      401      {object}  v1.BaseResponse                                "Invalid email or password"
// @Failure      403      {object}  v1.BaseResponse                                "Account not yet activated"
// @Failure      422      {object}  v1.BaseResponse                                "Validation error"
// @Router       /auth/login [post]
func (h Handler) Login(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "Login"
		fileName       = "auth_login.go"
	)
	ctx := r.Context()
	var req requests.LoginRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "Login: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "Login: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"request": logger.Fields{
				"email":        req.Email,
				"has_password": req.Password != "",
			},
		})
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.Login(ctx, authuc.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		ev := auditFromRequest(r)
		ev.Type = audit.EventLoginFailure
		ev.Email = req.Email
		ev.Reason = err.Error()
		audit.Record(ev)
		logger.ErrorWithContext(ctx, "Login failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
			"email":      req.Email,
		})
		return v1.RespondWithError(w, r, err)
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventLoginSuccess
	ev.Success = true
	ev.UserID = result.User.ID
	ev.Email = result.User.Email
	audit.Record(ev)

	return v1.NewSuccessResponse(w, r, http.StatusOK, "login success", responses.FromLoginResponse(result))
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"net/http"

	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
	"template/pkg/validators"
)

// GoogleLogin godoc
// @Summary      Google-ээр нэвтрэх (eID холболттой)
// @Description  Google OAuth callback-ийн code-ийг боловсруулна. Холбогдсон account бол шууд токен олгоно; эхний удаа бол eID-ээр баталгаажуулах link_token буцаана.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.GoogleLoginRequest  true  "OAuth code + redirect_uri"
// @Success      200  {object}  v1.BaseResponse{data=responses.GoogleLoginResponse}  "Linked → tokens, эсвэл эхний удаа → link_token"
// @Failure      400  {object}  v1.BaseResponse  "Invalid code"
// @Router       /auth/google [post]
func (h Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) error {
	const funcName = "GoogleLogin"
	ctx := r.Context()

	var req requests.GoogleLoginRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.GoogleLogin(ctx, req.Code, req.RedirectURI)
	if err != nil {
		logger.ErrorWithContext(ctx, "GoogleLogin failed in controller", logger.Fields{
			"controller": "auth", "method": funcName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "google login", responses.FromGoogleLoginResponse(result))
}

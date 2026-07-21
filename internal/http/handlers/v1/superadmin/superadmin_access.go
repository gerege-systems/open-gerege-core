// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package superadmin

import (
	"net/http"

	"template/internal/http/datatransfers/requests"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

// GetAccessMode godoc
// @Summary      Платформын хандалтын горим
// @Description  Платформын хандалтын горим (public|private)-ыг буцаана. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse  "Access mode fetched"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Router       /v1/superadmin/access-mode [get]
func (h Handler) GetAccessMode(w http.ResponseWriter, r *http.Request) error {
	mode, err := h.usecase.GetAccessMode(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "access mode fetched successfully", map[string]string{"mode": mode})
}

// SetAccessMode godoc
// @Summary      Платформын хандалтын горим тохируулах
// @Description  Платформын хандалтын горимыг тохируулна. public: хэн ч Government SSO-оор нэвтэрнэ; private: зөвхөн урьдчилан бүртгэсэн хэрэглэгч. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      requests.SuperadminAccessModeRequest  true  "Access mode"
// @Success      200  {object}  v1.BaseResponse  "Access mode updated"
// @Failure      400  {object}  v1.BaseResponse  "Invalid request body"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Router       /v1/superadmin/access-mode [put]
func (h Handler) SetAccessMode(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminAccessModeRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.SetAccessMode(r.Context(), req.Mode); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "access mode updated successfully", map[string]string{"mode": req.Mode})
}

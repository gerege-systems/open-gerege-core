// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package site нь сайтын нийтийн харагдацын default-ыг үйлчилнэ — landing
// зэрэг нийтийн хуудсанд уншигдах GET (auth-гүй) болон админ засварлах PUT
// ('settings.manage').
package site

import (
	"net/http"

	"template/internal/business/domain"
	siteuc "template/internal/business/usecases/site"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

type Handler struct {
	usecase siteuc.Usecase
}

func NewHandler(usecase siteuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// GetAppearance godoc
// @Summary Сайтын харагдацын default-ыг унших
// @Description Landing болон нийтийн хуудсанд хэрэглэх нийтийн харагдац (accent · font · style · theme). Нэвтрэлт шаардахгүй.
// @Tags site
// @Produce json
// @Success 200 {object} v1.BaseResponse{data=responses.SiteAppearanceResponse}
// @Router /site/appearance [get]
func (h Handler) GetAppearance(w http.ResponseWriter, r *http.Request) error {
	a, err := h.usecase.GetAppearance(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "appearance fetched", responses.ToSiteAppearance(a))
}

// SetAppearance godoc
// @Summary Сайтын харагдацын default-ыг шинэчлэх
// @Description Админ (settings.manage) сайтын нийтийн харагдацыг өөрчилнө. accent нь preset нэр эсвэл '#rrggbb' hex.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body requests.SiteAppearanceUpdateRequest true "Харагдацын шинэ утга"
// @Success 200 {object} v1.BaseResponse
// @Failure 400 {object} v1.BaseResponse "Буруу утга"
// @Failure 403 {object} v1.BaseResponse "settings.manage эрх дутуу"
// @Router /site/appearance [put]
func (h Handler) SetAppearance(w http.ResponseWriter, r *http.Request) error {
	var req requests.SiteAppearanceUpdateRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	err := h.usecase.SetAppearance(r.Context(), domain.SiteAppearance{
		Accent: req.Accent,
		Font:   req.Font,
		Style:  req.Style,
		Theme:  req.Theme,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "appearance updated", nil)
}

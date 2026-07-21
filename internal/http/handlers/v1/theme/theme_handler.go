// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package theme нь landing-ийн нэрлэсэн загваруудыг үйлчилнэ — нэвтрээгүй зочны
// уншдаг идэвхтэй theme (GET /site/theme) болон админы CRUD/идэвхжүүлэлт
// ('settings.manage').
package theme

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	themeuc "template/internal/business/usecases/theme"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

type Handler struct {
	usecase themeuc.Usecase
}

func NewHandler(usecase themeuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// GetActive godoc
// @Summary Идэвхтэй landing theme-ийг унших
// @Description Нэвтрээгүй зочны landing-д хэрэглэх идэвхтэй (default) theme. Нэвтрэлт шаардахгүй.
// @Tags site
// @Produce json
// @Success 200 {object} v1.BaseResponse{data=responses.ThemeResponse}
// @Router /themes/active [get]
func (h Handler) GetActive(w http.ResponseWriter, r *http.Request) error {
	t, err := h.usecase.GetActive(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "active theme fetched", responses.ToTheme(t))
}

// List godoc
// @Summary Theme-үүдийг жагсаах
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} v1.BaseResponse{data=[]responses.ThemeResponse}
// @Router /themes [get]
func (h Handler) List(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.List(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "themes fetched", responses.ToThemeList(list))
}

// Get godoc
// @Summary Нэг theme-ийг унших
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Theme id"
// @Success 200 {object} v1.BaseResponse{data=responses.ThemeResponse}
// @Router /themes/{id} [get]
func (h Handler) Get(w http.ResponseWriter, r *http.Request) error {
	t, err := h.usecase.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "theme fetched", responses.ToTheme(t))
}

// Create godoc
// @Summary Шинэ theme үүсгэх
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body requests.ThemeUpsertRequest true "Theme"
// @Success 200 {object} v1.BaseResponse{data=responses.ThemeResponse}
// @Router /themes [post]
func (h Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var req requests.ThemeUpsertRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	t, err := h.usecase.Create(r.Context(), req.Name, req.Config)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "theme created", responses.ToTheme(t))
}

// Update godoc
// @Summary Theme-ийг шинэчлэх
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Theme id"
// @Param request body requests.ThemeUpsertRequest true "Theme"
// @Success 200 {object} v1.BaseResponse
// @Router /themes/{id} [put]
func (h Handler) Update(w http.ResponseWriter, r *http.Request) error {
	var req requests.ThemeUpsertRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.Update(r.Context(), chi.URLParam(r, "id"), req.Name, req.Config); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "theme updated", nil)
}

// Delete godoc
// @Summary Theme-ийг устгах
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Theme id"
// @Success 200 {object} v1.BaseResponse
// @Router /themes/{id} [delete]
func (h Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "theme deleted", nil)
}

// SetActive godoc
// @Summary Theme-ийг идэвхтэй (default) болгох
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Theme id"
// @Success 200 {object} v1.BaseResponse
// @Router /themes/{id}/active [put]
func (h Handler) SetActive(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.SetActive(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "theme activated", nil)
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package assets нь гарын үсэг (хувь хүн) ба байгууллагын тамганы дардасын
// зургийн URL-ийг удирдах handler.
package assets

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	assetsuc "template/internal/business/usecases/assets"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

type Handler struct {
	usecase assetsuc.Usecase
}

func NewHandler(usecase assetsuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

type urlResponse struct {
	URL string `json:"url"`
}

func (h Handler) user(w http.ResponseWriter, r *http.Request) (string, bool) {
	u, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		_ = v1.NewAbortResponse(w, r, "invalid token")
		return "", false
	}
	return u.ID, true
}

// GetSignature godoc
// @Summary Гарын үсэг (eID) авах
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} v1.BaseResponse
// @Router /me/signature [get]
func (h Handler) GetSignature(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	url, err := h.usecase.GetSignature(r.Context(), uid)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "signature fetched", urlResponse{URL: url})
}

// SetSignature godoc
// @Summary Гарын үсэг (eID) хадгалах
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body requests.AssetURLRequest true "Зургийн URL"
// @Success 200 {object} v1.BaseResponse
// @Router /me/signature [put]
func (h Handler) SetSignature(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	var req requests.AssetURLRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.SetSignature(r.Context(), uid, req.URL); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "signature saved", urlResponse{URL: req.URL})
}

// DeleteSignature godoc
// @Summary Гарын үсэг устгах
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} v1.BaseResponse
// @Router /me/signature [delete]
func (h Handler) DeleteSignature(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	if err := h.usecase.DeleteSignature(r.Context(), uid); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "signature deleted", urlResponse{})
}

// SetLatinName godoc
// @Summary Латин нэр засах
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body requests.LatinNameRequest true "Латин нэр"
// @Success 200 {object} v1.BaseResponse
// @Router /me/latin-name [put]
func (h Handler) SetLatinName(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	var req requests.LatinNameRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.SetLatinName(r.Context(), uid, req.FirstNameEn, req.LastNameEn); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "latin name saved", nil)
}

// SetOrgNameLatin godoc
// @Summary Байгууллагын латин нэр засах (ADMIN)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param regNo path string true "Байгууллагын регистрийн дугаар"
// @Param payload body requests.OrgNameLatinRequest true "Латин нэр"
// @Success 200 {object} v1.BaseResponse
// @Router /me/org-name-latin/{regNo} [put]
func (h Handler) SetOrgNameLatin(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	var req requests.OrgNameLatinRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.SetOrgNameLatin(r.Context(), uid, chi.URLParam(r, "regNo"), req.NameLatin); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "org name latin saved", nil)
}

// GetStamp godoc
// @Summary Байгууллагын тамга авах
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param regNo path string true "Байгууллагын регистрийн дугаар"
// @Success 200 {object} v1.BaseResponse
// @Router /me/orgstamp/{regNo} [get]
func (h Handler) GetStamp(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	url, err := h.usecase.GetStamp(r.Context(), uid, chi.URLParam(r, "regNo"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "stamp fetched", urlResponse{URL: url})
}

// SetStamp godoc
// @Summary Байгууллагын тамга хадгалах (ADMIN)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param regNo path string true "Байгууллагын регистрийн дугаар"
// @Param payload body requests.AssetURLRequest true "Зургийн URL"
// @Success 200 {object} v1.BaseResponse
// @Router /me/orgstamp/{regNo} [put]
func (h Handler) SetStamp(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	var req requests.AssetURLRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.SetStamp(r.Context(), uid, chi.URLParam(r, "regNo"), req.URL); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "stamp saved", urlResponse{URL: req.URL})
}

// DeleteStamp godoc
// @Summary Байгууллагын тамга устгах (ADMIN)
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param regNo path string true "Байгууллагын регистрийн дугаар"
// @Success 200 {object} v1.BaseResponse
// @Router /me/orgstamp/{regNo} [delete]
func (h Handler) DeleteStamp(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	if err := h.usecase.DeleteStamp(r.Context(), uid, chi.URLParam(r, "regNo")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "stamp deleted", urlResponse{})
}

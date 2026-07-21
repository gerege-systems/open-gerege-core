// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package applications нь /applications/* endpoint-уудыг үйлчилнэ — нэгдсэн
// Gateway consumer + SSO RP бүртгэл (Hydra OAuth2 client). Бүгд 'gateway.manage'
// эрх шаардана (route давхаргад баталгаажна).
package applications

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	applicationsuc "template/internal/business/usecases/applications"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

type Handler struct {
	usecase applicationsuc.Usecase
}

func NewHandler(usecase applicationsuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

func decode[T any](w http.ResponseWriter, r *http.Request, req *T) bool {
	if err := v1.DecodeBody(r, req); err != nil {
		_ = v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
		return false
	}
	if err := validators.ValidatePayloads(*req); err != nil {
		_ = v1.RespondWithError(w, r, err)
		return false
	}
	return true
}

func toInput(req requests.ApplicationRequest) applicationsuc.Input {
	return applicationsuc.Input{
		Name:         req.Name,
		AppType:      req.AppType,
		RedirectURIs: req.RedirectURIs,
		Tags:         req.Tags,
		ServiceIDs:   req.ServiceIDs,
		Enabled:      req.Enabled,
	}
}

// List godoc
// @Summary      Application-уудыг жагсаах
// @Tags         applications
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /applications [get]
func (h Handler) List(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.List(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "applications fetched successfully", responses.ToApplicationList(list))
}

// Get godoc
// @Summary      Application-ыг id-гээр авах
// @Tags         applications
// @Produce      json
// @Param        id  path  string  true  "Application ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /applications/{id} [get]
func (h Handler) Get(w http.ResponseWriter, r *http.Request) error {
	a, err := h.usecase.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "application fetched successfully", responses.FromApplication(a))
}

// Create godoc
// @Summary      Application (OAuth2 client) үүсгэх
// @Tags         applications
// @Accept       json
// @Produce      json
// @Param        body  body  requests.ApplicationRequest  true  "Application"
// @Success      201  {object}  v1.BaseResponse
// @Router       /applications [post]
func (h Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var req requests.ApplicationRequest
	if !decode(w, r, &req) {
		return nil
	}
	a, err := h.usecase.Create(r.Context(), toInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "application created successfully", responses.FromApplication(a))
}

// Update godoc
// @Summary      Application шинэчлэх
// @Tags         applications
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Application ID"
// @Param        body  body  requests.ApplicationRequest  true  "Application"
// @Success      200  {object}  v1.BaseResponse
// @Router       /applications/{id} [put]
func (h Handler) Update(w http.ResponseWriter, r *http.Request) error {
	var req requests.ApplicationRequest
	if !decode(w, r, &req) {
		return nil
	}
	a, err := h.usecase.Update(r.Context(), chi.URLParam(r, "id"), toInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "application updated successfully", responses.FromApplication(a))
}

// Delete godoc
// @Summary      Application устгах (Hydra client + overlay)
// @Tags         applications
// @Produce      json
// @Param        id  path  string  true  "Application ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /applications/{id} [delete]
func (h Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "application deleted successfully", nil)
}

// RotateSecret godoc
// @Summary      Application-ын client secret эргүүлэх
// @Tags         applications
// @Produce      json
// @Param        id  path  string  true  "Application ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /applications/{id}/rotate-secret [post]
func (h Handler) RotateSecret(w http.ResponseWriter, r *http.Request) error {
	a, err := h.usecase.RotateSecret(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "secret rotated successfully", responses.FromApplication(a))
}

// SetSecret godoc
// @Summary      Application-ын client secret-ыг гараар оноох
// @Tags         applications
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Application ID"
// @Param        body  body  requests.ApplicationSecretRequest  true  "Client secret"
// @Success      200  {object}  v1.BaseResponse
// @Router       /applications/{id}/secret [put]
func (h Handler) SetSecret(w http.ResponseWriter, r *http.Request) error {
	var req requests.ApplicationSecretRequest
	if !decode(w, r, &req) {
		return nil
	}
	a, err := h.usecase.SetSecret(r.Context(), chi.URLParam(r, "id"), req.Secret)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "secret updated successfully", responses.FromApplication(a))
}

// SetServices godoc
// @Summary      Application-д зөвшөөрсөн gateway service-үүдийг оноох
// @Tags         applications
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Application ID"
// @Param        body  body  requests.ApplicationServicesRequest  true  "Service IDs"
// @Success      200  {object}  v1.BaseResponse
// @Router       /applications/{id}/services [put]
func (h Handler) SetServices(w http.ResponseWriter, r *http.Request) error {
	var req requests.ApplicationServicesRequest
	if !decode(w, r, &req) {
		return nil
	}
	a, err := h.usecase.SetServices(r.Context(), chi.URLParam(r, "id"), req.ServiceIDs)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "services updated successfully", responses.FromApplication(a))
}

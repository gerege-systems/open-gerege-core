// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package superadmin нь /v1/superadmin/* endpoint-уудыг үйлчилнэ — админ
// хэрэглэгчдийг жагсаах, үүсгэх, эрх олгох/хасах. Бүгд route түвшинд
// RequireSuperAdmin-ээр хамгаалагдсан (зөвхөн RoleSuperAdmin хандана).
package superadmin

import (
	"net/http"

	superadminuc "template/internal/business/usecases/superadmin"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

// Handler нь super admin-домэйн endpoint-уудыг үйлчилнэ — зөвхөн superadmin.Usecase руу дууддаг.
type Handler struct {
	usecase superadminuc.Usecase
}

func NewHandler(usecase superadminuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// ListAdmins godoc
// @Summary      Админуудыг жагсаах
// @Description  Админ түвшний бүх бүртгэлийг (super admin + admin) буцаана. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse  "Admins fetched"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Router       /v1/superadmin/admins [get]
func (h Handler) ListAdmins(w http.ResponseWriter, r *http.Request) error {
	res, err := h.usecase.ListAdmins(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "admins fetched successfully", responses.ToAdminUserList(res.Admins))
}

// CreateAdmin godoc
// @Summary      Шинэ админ үүсгэх
// @Description  Шинэ, идэвхтэй admin бүртгэл (нэр/и-мэйл/нууц үг) үүсгэнэ. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      requests.SuperadminCreateAdminRequest  true  "New admin"
// @Success      201  {object}  v1.BaseResponse  "Admin created"
// @Failure      400  {object}  v1.BaseResponse  "Invalid request body"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Failure      409  {object}  v1.BaseResponse  "Email already in use"
// @Router       /v1/superadmin/admins [post]
func (h Handler) CreateAdmin(w http.ResponseWriter, r *http.Request) error {
	var req requests.SuperadminCreateAdminRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	res, err := h.usecase.CreateAdmin(r.Context(), superadminuc.CreateAdminRequest{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		FirstNameEn: req.FirstNameEn,
		LastNameEn:  req.LastNameEn,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "admin created successfully", responses.FromAdminUser(res.User))
}

// GrantAdmin godoc
// @Summary      Хэрэглэгчид админ эрх олгох
// @Description  Байгаа хэрэглэгчийг admin болгоно. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID (UUID)"
// @Success      200  {object}  v1.BaseResponse  "Admin granted"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Failure      404  {object}  v1.BaseResponse  "User not found"
// @Failure      409  {object}  v1.BaseResponse  "Already an admin"
// @Router       /v1/superadmin/admins/{id}/grant [put]
func (h Handler) GrantAdmin(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if err := h.usecase.GrantAdmin(r.Context(), superadminuc.GrantAdminRequest{UserID: id}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "admin granted successfully", nil)
}

// RevokeAdmin godoc
// @Summary      Админ эрхийг хасах
// @Description  Admin-ийн эрхийг хасч, энгийн хэрэглэгч болгоно. Super admin-г хасахгүй, өөрийгөө хасахгүй. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID (UUID)"
// @Success      200  {object}  v1.BaseResponse  "Admin revoked"
// @Failure      400  {object}  v1.BaseResponse  "User is not an admin"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin / cannot revoke self or super admin"
// @Failure      404  {object}  v1.BaseResponse  "User not found"
// @Router       /v1/superadmin/admins/{id} [delete]
func (h Handler) RevokeAdmin(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	id := chi.URLParam(r, "id")
	if err := h.usecase.RevokeAdmin(r.Context(), superadminuc.RevokeAdminRequest{UserID: id, ActorID: user.ID}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "admin revoked successfully", nil)
}

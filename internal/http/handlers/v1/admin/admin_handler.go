// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package admin нь /admin/* удирдлагын endpoint-уудыг үйлчилнэ — хэрэглэгч
// жагсаах, role солих, идэвхжүүлэх/хаах, устгах. Бүгд 'users.manage' эрхээр
// хамгаалагдсан (route түвшинд).
package admin

import (
	"net/http"
	"strconv"

	"template/internal/business/usecases/users"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	usersUC users.Usecase
}

func NewHandler(usersUC users.Usecase) Handler {
	return Handler{usersUC: usersUC}
}

const (
	defaultLimit = 50
	maxLimit     = 200
)

// ListUsers нь хэрэглэгчдийг хуудаслан буцаана. Query: ?offset=&limit=&role=&active=
func (h Handler) ListUsers(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	offset := atoiDefault(q.Get("offset"), 0)
	limit := atoiDefault(q.Get("limit"), defaultLimit)
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}
	req := users.ListRequest{
		RoleID:     atoiDefault(q.Get("role"), 0),
		ActiveOnly: q.Get("active") == "true",
		Offset:     offset,
		Limit:      limit,
	}
	res, err := h.usersUC.List(r.Context(), req)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "users fetched successfully", responses.ToAdminUserList(res.Users))
}

// UpdateUserRole нь хэрэглэгчийн role-г солино.
func (h Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req requests.AdminUpdateUserRoleRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	// Дуудагчийн эрхийг usecase-д дамжуулна — admin эрх олгох/хасахыг зөвхөн
	// super admin хийнэ (энгийн admin нь зөвхөн manager ↔ user).
	caller, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, "invalid token")
	}
	if err := h.usersUC.UpdateRole(r.Context(), users.UpdateRoleRequest{
		UserID: id, RoleID: req.RoleID, CallerRoleID: caller.RoleID,
	}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "user role updated successfully", nil)
}

// CreateUser нь private платформд иргэнийг регистрийн дугаараар урьдчилан
// бүртгэнэ (role оноож). Иргэн хожим Government SSO-оор эхэлж нэвтэрхэд энэ мөр
// нь civil_id/sso_sub-оор холбогдоно.
//
// @Summary      Хэрэглэгч урьдчилан бүртгэх (private платформ)
// @Description  Иргэнийг регистрийн дугаараар урьдчилан бүртгэж, role ононо. Private горимд зөвхөн ингэж бүртгэсэн иргэн Government SSO-оор нэвтэрнэ. 'users.manage' эрх шаардана; admin/superadmin role-ыг зөвхөн super admin ононо.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      requests.AdminCreateUserRequest  true  "Register + role"
// @Success      201  {object}  v1.BaseResponse  "User pre-registered"
// @Failure      400  {object}  v1.BaseResponse  "Invalid request body"
// @Failure      403  {object}  v1.BaseResponse  "Insufficient privilege for the role"
// @Failure      409  {object}  v1.BaseResponse  "Register already exists"
// @Router       /v1/admin/users [post]
func (h Handler) CreateUser(w http.ResponseWriter, r *http.Request) error {
	var req requests.AdminCreateUserRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	caller, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, "invalid token")
	}
	user, err := h.usersUC.CreatePreRegistered(r.Context(), users.CreatePreRegisterRequest{
		Register:     req.Register,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		FirstNameEn:  req.FirstNameEn,
		LastNameEn:   req.LastNameEn,
		RoleID:       req.RoleID,
		CallerRoleID: caller.RoleID,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "user pre-registered successfully", responses.FromAdminUser(user))
}

// SetUserActive нь хэрэглэгчийг идэвхжүүлэх/идэвхгүй болгоно.
func (h Handler) SetUserActive(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	var req requests.AdminSetActiveRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := h.usersUC.SetActive(r.Context(), users.SetActiveRequest{UserID: id, Active: req.Active}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "user status updated successfully", nil)
}

// DeleteUser нь хэрэглэгчийг зөөлөн устгана.
func (h Handler) DeleteUser(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if err := h.usersUC.Delete(r.Context(), users.DeleteRequest{UserID: id}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "user deleted successfully", nil)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

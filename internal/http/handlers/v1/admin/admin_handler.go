// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package admin нь /admin/* удирдлагын endpoint-уудыг үйлчилнэ — хэрэглэгч
// жагсаах, role солих, идэвхжүүлэх/хаах, устгах. Бүгд 'users.manage' эрхээр
// хамгаалагдсан (route түвшинд).
package admin

import (
	"net/http"
	"strconv"

	"template/internal/business/usecases/users"
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
	if err := h.usersUC.UpdateRole(r.Context(), users.UpdateRoleRequest{UserID: id, RoleID: req.RoleID}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "user role updated successfully", nil)
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

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package rbac нь /rbac/* endpoint-уудыг үйлчилнэ — эрх (roles) CRUD, эрхийн
// каталог, мөн одоогийн хэрэглэгчийн эрхийг буцаах /rbac/me.
package rbac

import (
	"context"
	"net/http"
	"strconv"

	audituc "template/internal/business/usecases/audit"
	rbacuc "template/internal/business/usecases/rbac"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

// Handler нь RBAC endpoint-уудыг үйлчилнэ. auditUC нь role-ийн өөрчлөлтийг
// persisted audit log руу best-effort бичих use case (nil байж болно).
type Handler struct {
	usecase rbacuc.Usecase
	auditUC audituc.Usecase
}

func NewHandler(usecase rbacuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// NewHandlerWithAudit нь audit use case-ийг тарьж handler үүсгэнэ.
func NewHandlerWithAudit(usecase rbacuc.Usecase, auditUC audituc.Usecase) Handler {
	return Handler{usecase: usecase, auditUC: auditUC}
}

// recordAudit нь RBAC үйл явдлыг persisted audit log руу best-effort бичнэ —
// амжилтгүй болсон ч HTTP урсгалыг эвдэхгүй (зөвхөн log).
func (h Handler) recordAudit(ctx context.Context, action, target string, metadata map[string]any) {
	if h.auditUC == nil {
		return
	}
	if err := h.auditUC.RecordEvent(ctx, action, "rbac", target, metadata); err != nil {
		logger.ErrorWithContext(ctx, "rbac: persisted audit write failed (non-fatal)", logger.Fields{
			"action": action,
			"error":  err.Error(),
		})
	}
}

// MyPermissions нь нэвтэрсэн хэрэглэгчийн эрхийн түлхүүрүүдийг буцаана (frontend
// цэс/товчийг шүүхэд). /rbac/me — нэвтэрсэн хэрэглэгч бүрт нээлттэй.
func (h Handler) MyPermissions(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	roleID := effectiveRoleID(user)
	perms, err := h.usecase.Resolve(r.Context(), roleID)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if perms == nil {
		perms = []string{}
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "permissions fetched successfully", perms)
}

// effectiveRoleID нь токенд RoleID байхгүй (хуучин токен) бол IsAdmin-аас
// гаргана (admin=1, бусад=2).
func effectiveRoleID(user httpauth.CurrentUser) int {
	if user.RoleID != 0 {
		return user.RoleID
	}
	if user.IsAdmin {
		return 1
	}
	return 2
}

// ListRoles нь эрх бүрийг permission-уудтай нь жагсаана (RBAC matrix).
func (h Handler) ListRoles(w http.ResponseWriter, r *http.Request) error {
	res, err := h.usecase.ListRoles(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "roles fetched successfully", responses.ToRoleList(res))
}

// ListPermissions нь эрхийн каталогийг буцаана.
func (h Handler) ListPermissions(w http.ResponseWriter, r *http.Request) error {
	res, err := h.usecase.ListPermissions(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "permissions fetched successfully", responses.ToPermissionList(res))
}

// CreateRole нь шинэ role үүсгэнэ.
func (h Handler) CreateRole(w http.ResponseWriter, r *http.Request) error {
	var req requests.CreateRoleRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	role, err := h.usecase.CreateRole(r.Context(), rbacuc.CreateRoleRequest{
		Key: req.Key, Name: req.Name, Description: req.Description, Permissions: req.Permissions,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "role created successfully", responses.FromRole(role))
}

// UpdateRole нь role-ийн нэр/тайлбар (+ заасан бол permission) шинэчилнэ.
func (h Handler) UpdateRole(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusNotFound, "role not found")
	}
	var req requests.UpdateRoleRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	role, err := h.usecase.UpdateRole(r.Context(), rbacuc.UpdateRoleRequest{
		ID: id, Name: req.Name, Description: req.Description, Permissions: req.Permissions,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "role updated successfully", responses.FromRole(role))
}

// SetRolePermissions нь role-ийн permission-уудыг бүхэлд нь солино.
func (h Handler) SetRolePermissions(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusNotFound, "role not found")
	}
	var req requests.SetRolePermissionsRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.SetRolePermissions(r.Context(), id, req.Permissions); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	h.recordAudit(r.Context(), "rbac.role.permissions.set", strconv.Itoa(id), map[string]any{
		"permission_count": len(req.Permissions),
	})
	return v1.NewSuccessResponse(w, r, http.StatusOK, "role permissions updated successfully", nil)
}

// DeleteRole нь системийн бус role-ийг устгана (ашиглагдаагүй бол).
func (h Handler) DeleteRole(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusNotFound, "role not found")
	}
	if err := h.usecase.DeleteRole(r.Context(), id); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "role deleted successfully", nil)
}

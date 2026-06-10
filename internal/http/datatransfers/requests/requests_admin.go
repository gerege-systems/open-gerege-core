// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// AdminUpdateUserRoleRequest нь PUT /admin/users/{id}/role-ийн body.
type AdminUpdateUserRoleRequest struct {
	RoleID int `json:"role_id" validate:"required,min=1"`
}

// AdminSetActiveRequest нь PUT /admin/users/{id}/active-ийн body.
type AdminSetActiveRequest struct {
	Active bool `json:"active"`
}

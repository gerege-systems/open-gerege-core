// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// CreateRoleRequest нь POST /rbac/roles-ийн body. key хоосон бол name-ээс
// гаргана (usecase slugify хийнэ).
type CreateRoleRequest struct {
	Key         string   `json:"key" validate:"omitempty,max=40"`
	Name        string   `json:"name" validate:"required,min=2,max=50"`
	Description string   `json:"description" validate:"omitempty,max=200"`
	Permissions []string `json:"permissions" validate:"omitempty,dive,max=40"`
}

// UpdateRoleRequest нь PUT /rbac/roles/{id}-ийн body. Permissions nil бол
// эрхийг хөндөхгүй.
type UpdateRoleRequest struct {
	Name        string   `json:"name" validate:"required,min=2,max=50"`
	Description string   `json:"description" validate:"omitempty,max=200"`
	Permissions []string `json:"permissions" validate:"omitempty,dive,max=40"`
}

// SetRolePermissionsRequest нь PUT /rbac/roles/{id}/permissions-ийн body.
type SetRolePermissionsRequest struct {
	Permissions []string `json:"permissions" validate:"omitempty,dive,max=40"`
}

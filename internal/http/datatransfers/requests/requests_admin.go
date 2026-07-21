// Government Template Platform V3.0
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

// AdminCreateUserRequest нь POST /admin/users-ийн body — private платформд иргэнийг
// регистрийн дугаараар урьдчилан бүртгэх. role_id: 2=admin, 3=manager, 4=user
// (superadmin(1) энэ замаар оноогдохгүй). Хоосон role_id → user.
type AdminCreateUserRequest struct {
	Register    string `json:"register" validate:"required,min=6,max=20"`
	FirstName   string `json:"first_name" validate:"omitempty,max=100"`
	LastName    string `json:"last_name" validate:"omitempty,max=100"`
	FirstNameEn string `json:"first_name_en" validate:"omitempty,max=100"`
	LastNameEn  string `json:"last_name_en" validate:"omitempty,max=100"`
	RoleID      int    `json:"role_id" validate:"omitempty,oneof=2 3 4"`
}

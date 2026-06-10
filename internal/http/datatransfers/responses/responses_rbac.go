// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"template/internal/business/domain"
	rbacuc "template/internal/business/usecases/rbac"
)

// RoleResponse нь нэг role-ийг (оноосон permission-уудтай нь) клиентэд буцаана.
type RoleResponse struct {
	ID          int      `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
	Permissions []string `json:"permissions"`
}

// PermissionResponse нь эрхийн каталогийн нэг бичлэг.
type PermissionResponse struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Category string `json:"category"`
}

// FromRole нь permission-гүй (эсвэл тусдаа оноох) role-г буцаана.
func FromRole(r domain.Role) RoleResponse {
	return RoleResponse{
		ID: r.ID, Key: r.Key, Name: r.Name, Description: r.Description,
		IsSystem: r.IsSystem, Permissions: []string{},
	}
}

// ToRoleList нь RBAC matrix-д зориулж role бүрийг эрхүүдтэй нь буцаана.
func ToRoleList(list []rbacuc.RoleWithPerms) []RoleResponse {
	out := make([]RoleResponse, 0, len(list))
	for _, rp := range list {
		perms := rp.Permissions
		if perms == nil {
			perms = []string{}
		}
		out = append(out, RoleResponse{
			ID: rp.Role.ID, Key: rp.Role.Key, Name: rp.Role.Name,
			Description: rp.Role.Description, IsSystem: rp.Role.IsSystem, Permissions: perms,
		})
	}
	return out
}

// ToPermissionList нь эрхийн каталогийг буцаана.
func ToPermissionList(list []domain.Permission) []PermissionResponse {
	out := make([]PermissionResponse, 0, len(list))
	for _, p := range list {
		out = append(out, PermissionResponse{Key: p.Key, Label: p.Label, Category: p.Category})
	}
	return out
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package rbac нь динамик эрх (roles) + эрхийн каталог (permissions)-ийг удирдаж,
// нэг role-ийн эрхүүдийг (enforcement-д зориулж) тооцоолж/кэшлэнэ.
package rbac

import (
	"context"

	"template/internal/business/domain"
)

type Usecase interface {
	// ListRoles нь эрх бүрийг түүнд оноогдсон permission түлхүүрүүдтэй нь
	// буцаана (RBAC matrix-д).
	ListRoles(ctx context.Context) ([]RoleWithPerms, error)
	CreateRole(ctx context.Context, req CreateRoleRequest) (domain.Role, error)
	UpdateRole(ctx context.Context, req UpdateRoleRequest) (domain.Role, error)
	DeleteRole(ctx context.Context, id int) error
	ListPermissions(ctx context.Context) ([]domain.Permission, error)
	SetRolePermissions(ctx context.Context, roleID int, keys []string) error
	// Resolve нь нэг role-ийн эрхийн түлхүүрүүдийг буцаана (кэштэй). 'admin'
	// эрх нь каталогийн БҮХ эрхийг автоматаар авна.
	Resolve(ctx context.Context, roleID int) ([]string, error)
}

type (
	RoleWithPerms struct {
		Role        domain.Role
		Permissions []string
	}
	CreateRoleRequest struct {
		Key         string
		Name        string
		Description string
		Permissions []string
	}
	UpdateRoleRequest struct {
		ID          int
		Name        string
		Description string
		Permissions []string // nil бол permission-ийг хөндөхгүй
	}
)

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// repointerface.RBACRepository-д зориулсан гараар бичсэн mock — төслийн
// бусад mock-уудтай ижил testify/mock хэв маяг.

package mocks

import (
	context "context"

	"template/internal/business/domain"

	mock "github.com/stretchr/testify/mock"
)

// RBACRepository нь repointerface.RBACRepository-ийн mock юм.
type RBACRepository struct {
	mock.Mock
}

func (_m *RBACRepository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	ret := _m.Called(ctx)
	var r0 []domain.Role
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]domain.Role)
	}
	return r0, ret.Error(1)
}

func (_m *RBACRepository) GetRole(ctx context.Context, id int) (domain.Role, error) {
	ret := _m.Called(ctx, id)
	return ret.Get(0).(domain.Role), ret.Error(1)
}

func (_m *RBACRepository) CreateRole(ctx context.Context, in *domain.Role) (domain.Role, error) {
	ret := _m.Called(ctx, in)
	return ret.Get(0).(domain.Role), ret.Error(1)
}

func (_m *RBACRepository) UpdateRole(ctx context.Context, in *domain.Role) (domain.Role, error) {
	ret := _m.Called(ctx, in)
	return ret.Get(0).(domain.Role), ret.Error(1)
}

func (_m *RBACRepository) DeleteRole(ctx context.Context, id int) error {
	return _m.Called(ctx, id).Error(0)
}

func (_m *RBACRepository) CountUsersWithRole(ctx context.Context, roleID int) (int, error) {
	ret := _m.Called(ctx, roleID)
	return ret.Get(0).(int), ret.Error(1)
}

func (_m *RBACRepository) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	ret := _m.Called(ctx)
	var r0 []domain.Permission
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]domain.Permission)
	}
	return r0, ret.Error(1)
}

func (_m *RBACRepository) GetRolePermissions(ctx context.Context, roleID int) ([]string, error) {
	ret := _m.Called(ctx, roleID)
	var r0 []string
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]string)
	}
	return r0, ret.Error(1)
}

func (_m *RBACRepository) SetRolePermissions(ctx context.Context, roleID int, keys []string) error {
	return _m.Called(ctx, roleID, keys).Error(0)
}

type mockConstructorTestingTNewRBACRepository interface {
	mock.TestingT
	Cleanup(func())
}

// NewRBACRepository нь RBACRepository mock-ийн шинэ instance үүсгэж, testing
// interface болон cleanup-ийг бүртгэнэ (хүлээлтийг батална).
func NewRBACRepository(t mockConstructorTestingTNewRBACRepository) *RBACRepository {
	m := &RBACRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

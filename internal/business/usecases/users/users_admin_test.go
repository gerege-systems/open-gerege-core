// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Super admin хамгаалалтын тестүүд: users.manage эрхтэй энгийн admin (энэ
// usecase-аар дамжина) super admin зэрэглэлийг оноож, эсвэл super admin
// бүртгэлийг өөрчилж/устгаж чадахгүй байх ёстой (privilege-escalation хаалт).
package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
)

func superAdminFixtureUser() domain.User {
	u := sampleUser()
	u.ID = "22222222-2222-2222-2222-222222222222"
	u.RoleID = domain.RoleSuperAdmin
	return u
}

func TestUpdateRole_RejectsAssigningSuperAdmin(t *testing.T) {
	f := newFixture(t)
	// Guard нь GetByID-аас өмнө буцдаг тул repo дуудлага хэрэггүй.
	err := f.usecase.UpdateRole(context.Background(), users.UpdateRoleRequest{UserID: "u", RoleID: domain.RoleSuperAdmin})
	require.Error(t, err)
	var de *apperror.DomainError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

func TestUpdateRole_RejectsModifyingSuperAdmin(t *testing.T) {
	f := newFixture(t)
	sa := superAdminFixtureUser()
	f.repo.On("GetByID", context.Background(), sa.ID).Return(sa, nil).Once()
	err := f.usecase.UpdateRole(context.Background(), users.UpdateRoleRequest{UserID: sa.ID, RoleID: domain.RoleUser})
	require.Error(t, err)
	var de *apperror.DomainError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

func TestSetActive_RejectsSuperAdmin(t *testing.T) {
	f := newFixture(t)
	sa := superAdminFixtureUser()
	f.repo.On("GetByID", context.Background(), sa.ID).Return(sa, nil).Once()
	err := f.usecase.SetActive(context.Background(), users.SetActiveRequest{UserID: sa.ID, Active: false})
	require.Error(t, err)
	var de *apperror.DomainError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

func TestDelete_RejectsSuperAdmin(t *testing.T) {
	f := newFixture(t)
	sa := superAdminFixtureUser()
	f.repo.On("GetByID", context.Background(), sa.ID).Return(sa, nil).Once()
	err := f.usecase.Delete(context.Background(), users.DeleteRequest{UserID: sa.ID})
	require.Error(t, err)
	var de *apperror.DomainError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

// ── Admin эрх олгох хязгаарлалт ──────────────────────────────────────────—
// Энгийн admin (users.manage) нь бусдад ADMIN эрх өгч, эсвэл admin бүртгэлийг
// өөрчилж чадахгүй — зөвхөн super admin. Admin нь manager ↔ user л сольж болно.

func TestUpdateRole_AdminCannotGrantAdmin(t *testing.T) {
	f := newFixture(t)
	target := sampleUser() // энгийн хэрэглэгч
	f.repo.On("GetByID", context.Background(), target.ID).Return(target, nil).Once()

	err := f.usecase.UpdateRole(context.Background(), users.UpdateRoleRequest{
		UserID: target.ID, RoleID: domain.RoleAdmin, CallerRoleID: domain.RoleAdmin,
	})
	require.Error(t, err)
	var de *apperror.DomainError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

func TestUpdateRole_AdminCannotChangeAdminAccount(t *testing.T) {
	f := newFixture(t)
	target := sampleUser()
	target.RoleID = domain.RoleAdmin // зорилтот нь admin
	f.repo.On("GetByID", context.Background(), target.ID).Return(target, nil).Once()

	err := f.usecase.UpdateRole(context.Background(), users.UpdateRoleRequest{
		UserID: target.ID, RoleID: domain.RoleUser, CallerRoleID: domain.RoleAdmin,
	})
	require.Error(t, err)
	var de *apperror.DomainError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

func TestUpdateRole_AdminCanSetManager(t *testing.T) {
	f := newFixture(t)
	target := sampleUser()
	f.repo.On("GetByID", context.Background(), target.ID).Return(target, nil).Once()
	f.repo.On("UpdateRole", context.Background(), target.ID, domain.RoleManager).Return(nil).Once()
	f.rc.On("Del", "user/"+target.Email).Once()

	err := f.usecase.UpdateRole(context.Background(), users.UpdateRoleRequest{
		UserID: target.ID, RoleID: domain.RoleManager, CallerRoleID: domain.RoleAdmin,
	})
	require.NoError(t, err)
}

func TestUpdateRole_SuperAdminCanGrantAdmin(t *testing.T) {
	f := newFixture(t)
	target := sampleUser()
	f.repo.On("GetByID", context.Background(), target.ID).Return(target, nil).Once()
	f.repo.On("UpdateRole", context.Background(), target.ID, domain.RoleAdmin).Return(nil).Once()
	f.rc.On("Del", "user/"+target.Email).Once()

	err := f.usecase.UpdateRole(context.Background(), users.UpdateRoleRequest{
		UserID: target.ID, RoleID: domain.RoleAdmin, CallerRoleID: domain.RoleSuperAdmin,
	})
	require.NoError(t, err)
}

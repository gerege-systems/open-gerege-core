// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package superadmin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/superadmin"
	"template/internal/business/usecases/users"
	"template/internal/test/mocks"
)

func newUC(t *testing.T) (superadmin.Usecase, *mocks.UsersUsecase, *mocks.AuditUsecase) {
	t.Helper()
	usersUC := mocks.NewUsersUsecase(t)
	auditUC := mocks.NewAuditUsecase(t)
	return superadmin.NewUsecase(usersUC, auditUC), usersUC, auditUC
}

func TestCreateAdmin_ActivatesAndAudits(t *testing.T) {
	uc, usersUC, auditUC := newUC(t)
	stored := domain.User{ID: "u1", Email: "a@b.mn", Username: "adm", RoleID: domain.RoleAdmin}
	usersUC.On("Store", mock.Anything, mock.MatchedBy(func(req users.StoreRequest) bool {
		return req.User.RoleID == domain.RoleAdmin && req.User.Email == "a@b.mn"
	})).Return(users.StoreResponse{User: stored}, nil)
	usersUC.On("SetActive", mock.Anything, users.SetActiveRequest{UserID: "u1", Active: true}).Return(nil)
	auditUC.On("RecordEvent", mock.Anything, "superadmin.create_admin", "superadmin", "u1", mock.Anything).Return(nil)

	res, err := uc.CreateAdmin(context.Background(), superadmin.CreateAdminRequest{Username: "adm", Email: "a@b.mn", Password: "password1"})
	require.NoError(t, err)
	assert.True(t, res.User.Active, "шинэ админ идэвхтэй байх ёстой")
}

func TestGrantAdmin_AlreadyAdmin_Conflict(t *testing.T) {
	uc, usersUC, _ := newUC(t)
	usersUC.On("GetByID", mock.Anything, users.GetByIDRequest{ID: "u1"}).
		Return(users.GetByIDResponse{User: domain.User{ID: "u1", RoleID: domain.RoleAdmin}}, nil)

	err := uc.GrantAdmin(context.Background(), superadmin.GrantAdminRequest{UserID: "u1"})
	require.Error(t, err)
	de, ok := err.(*apperror.DomainError)
	require.True(t, ok)
	assert.Equal(t, apperror.ErrTypeConflict, de.Type)
}

func TestGrantAdmin_Success(t *testing.T) {
	uc, usersUC, auditUC := newUC(t)
	usersUC.On("GetByID", mock.Anything, users.GetByIDRequest{ID: "u2"}).
		Return(users.GetByIDResponse{User: domain.User{ID: "u2", Email: "x@y.mn", RoleID: domain.RoleUser}}, nil)
	usersUC.On("UpdateRole", mock.Anything, users.UpdateRoleRequest{UserID: "u2", RoleID: domain.RoleAdmin}).Return(nil)
	auditUC.On("RecordEvent", mock.Anything, "superadmin.grant_admin", "superadmin", "u2", mock.Anything).Return(nil)

	require.NoError(t, uc.GrantAdmin(context.Background(), superadmin.GrantAdminRequest{UserID: "u2"}))
}

func TestRevokeAdmin_Self_Forbidden(t *testing.T) {
	uc, _, _ := newUC(t)
	err := uc.RevokeAdmin(context.Background(), superadmin.RevokeAdminRequest{UserID: "me", ActorID: "me"})
	require.Error(t, err)
	de, ok := err.(*apperror.DomainError)
	require.True(t, ok)
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

func TestRevokeAdmin_SuperAdmin_Forbidden(t *testing.T) {
	uc, usersUC, _ := newUC(t)
	usersUC.On("GetByID", mock.Anything, users.GetByIDRequest{ID: "sa"}).
		Return(users.GetByIDResponse{User: domain.User{ID: "sa", RoleID: domain.RoleSuperAdmin}}, nil)

	err := uc.RevokeAdmin(context.Background(), superadmin.RevokeAdminRequest{UserID: "sa", ActorID: "root"})
	require.Error(t, err)
	de, ok := err.(*apperror.DomainError)
	require.True(t, ok)
	assert.Equal(t, apperror.ErrTypeForbidden, de.Type)
}

func TestRevokeAdmin_NotAdmin_BadRequest(t *testing.T) {
	uc, usersUC, _ := newUC(t)
	usersUC.On("GetByID", mock.Anything, users.GetByIDRequest{ID: "u"}).
		Return(users.GetByIDResponse{User: domain.User{ID: "u", RoleID: domain.RoleUser}}, nil)

	err := uc.RevokeAdmin(context.Background(), superadmin.RevokeAdminRequest{UserID: "u", ActorID: "root"})
	require.Error(t, err)
	de, ok := err.(*apperror.DomainError)
	require.True(t, ok)
	assert.Equal(t, apperror.ErrTypeBadRequest, de.Type)
}

func TestRevokeAdmin_Success(t *testing.T) {
	uc, usersUC, auditUC := newUC(t)
	usersUC.On("GetByID", mock.Anything, users.GetByIDRequest{ID: "u3"}).
		Return(users.GetByIDResponse{User: domain.User{ID: "u3", Email: "z@y.mn", RoleID: domain.RoleAdmin}}, nil)
	usersUC.On("UpdateRole", mock.Anything, users.UpdateRoleRequest{UserID: "u3", RoleID: domain.RoleUser}).Return(nil)
	auditUC.On("RecordEvent", mock.Anything, "superadmin.revoke_admin", "superadmin", "u3", mock.Anything).Return(nil)

	require.NoError(t, uc.RevokeAdmin(context.Background(), superadmin.RevokeAdminRequest{UserID: "u3", ActorID: "root"}))
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Owner дүрийн эскалацийн дүрмүүдийн тест: owner дүрийг зөвхөн owner олгоно,
// owner-ийн дүр өөрчлөгдөхгүй (demote→remove тойрох замыг хаана), гишүүн/
// гишүүн-бус хэн ч дүр солихгүй. Эдгээр нь org_impl_test.go-гийн үндсэн
// эрх олголтын тестүүдийг owner-т чиглэсэн дүрмээр нөхнө.
package org_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/org"
)

// requireErrType нь apperror.DomainError-ийн төрлийг шалгадаг жижиг туслах.
func requireErrType(t *testing.T, err error, want apperror.ErrorType) {
	t.Helper()
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr), "apperror.DomainError хүлээж байсан, авсан: %v", err)
	assert.Equal(t, want, domErr.Type)
}

func TestUpdateMemberRole(t *testing.T) {
	t.Run("admin can change member to admin", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleAdmin}, nil).Once()
		f.repo.On("GetMembership", mock.Anything, orgID, targetID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: targetID, Role: domain.OrgRoleMember}, nil).Once()
		f.repo.On("UpdateMemberRole", mock.Anything, orgID, targetID, domain.OrgRoleAdmin).Return(nil).Once()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleAdmin,
		})
		require.NoError(t, err)
	})

	t.Run("plain member is forbidden from changing roles", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleMember}, nil).Once()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleAdmin,
		})
		requireErrType(t, err, apperror.ErrTypeForbidden)
	})

	t.Run("non-member is forbidden from changing roles", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{}, apperror.NotFound("membership not found")).Once()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleMember,
		})
		requireErrType(t, err, apperror.ErrTypeForbidden)
	})

	t.Run("admin cannot grant the owner role (escalation)", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleAdmin}, nil).Once()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleOwner,
		})
		requireErrType(t, err, apperror.ErrTypeForbidden)
	})

	t.Run("admin cannot grant the owner role to themselves (self-escalation)", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleAdmin}, nil).Once()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: callerID, Role: domain.OrgRoleOwner,
		})
		requireErrType(t, err, apperror.ErrTypeForbidden)
	})

	t.Run("admin cannot demote the owner (bypass of remove-protection)", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleAdmin}, nil).Once()
		f.repo.On("GetMembership", mock.Anything, orgID, targetID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: targetID, Role: domain.OrgRoleOwner}, nil).Once()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleMember,
		})
		requireErrType(t, err, apperror.ErrTypeBadRequest)
	})

	t.Run("owner cannot demote themselves (org would become ownerless)", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleOwner}, nil).Twice()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: callerID, Role: domain.OrgRoleMember,
		})
		requireErrType(t, err, apperror.ErrTypeBadRequest)
	})

	t.Run("owner can grant the owner role (co-owner)", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleOwner}, nil).Once()
		f.repo.On("GetMembership", mock.Anything, orgID, targetID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: targetID, Role: domain.OrgRoleAdmin}, nil).Once()
		f.repo.On("UpdateMemberRole", mock.Anything, orgID, targetID, domain.OrgRoleOwner).Return(nil).Once()

		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleOwner,
		})
		require.NoError(t, err)
	})

	t.Run("invalid role is rejected", func(t *testing.T) {
		f := newFixture(t)
		err := f.usecase.UpdateMemberRole(context.Background(), org.UpdateMemberRoleRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: "superuser",
		})
		requireErrType(t, err, apperror.ErrTypeBadRequest)
	})
}

func TestAddMemberOwnerRule(t *testing.T) {
	t.Run("admin cannot add a member with the owner role", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleAdmin}, nil).Once()

		_, err := f.usecase.AddMember(context.Background(), org.AddMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleOwner,
		})
		requireErrType(t, err, apperror.ErrTypeForbidden)
	})

	t.Run("owner can add a co-owner", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleOwner}, nil).Once()
		f.repo.On("AddMember", mock.Anything, mock.MatchedBy(func(m *domain.OrganizationMembership) bool {
			return m.OrgID == orgID && m.UserID == targetID && m.Role == domain.OrgRoleOwner
		})).Return(domain.OrganizationMembership{OrgID: orgID, UserID: targetID, Role: domain.OrgRoleOwner}, nil).Once()

		resp, err := f.usecase.AddMember(context.Background(), org.AddMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleOwner,
		})
		require.NoError(t, err)
		assert.Equal(t, domain.OrgRoleOwner, resp.Membership.Role)
	})

	t.Run("invalid role is rejected before any repo call", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.usecase.AddMember(context.Background(), org.AddMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: "root",
		})
		requireErrType(t, err, apperror.ErrTypeBadRequest)
	})
}

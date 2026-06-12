// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

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
	"template/internal/test/mocks"
)

// fixture нь тест тус бүрийн холболт юм. Тест бүр цэвэр mock авахын тулд
// newFixture()-ийг дуудна — тестүүдийн хооронд хуваалцсан төлөв байхгүй.
type fixture struct {
	usecase org.Usecase
	repo    *mocks.OrgRepository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := mocks.NewOrgRepository(t)
	return &fixture{usecase: org.NewUsecase(repo), repo: repo}
}

const (
	callerID = "11111111-1111-1111-1111-111111111111"
	orgID    = "22222222-2222-2222-2222-222222222222"
	targetID = "33333333-3333-3333-3333-333333333333"
)

// TestCreateOrganization нь happy path болон validation алдааг шалгана.
func TestCreateOrganization(t *testing.T) {
	t.Run("creates org and returns stored row", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("CreateOrg", mock.Anything, mock.MatchedBy(func(o *domain.Organization) bool {
			return o.RegNo == "1234567" && o.Name == "Тест ХХК" && o.CreatedBy == callerID
		})).Return(domain.Organization{ID: orgID, RegNo: "1234567", Name: "Тест ХХК", CreatedBy: callerID}, nil).Once()

		resp, err := f.usecase.CreateOrganization(context.Background(), org.CreateOrganizationRequest{
			CallerID: callerID, RegNo: "  1234567 ", Name: " Тест ХХК ",
		})
		require.NoError(t, err)
		assert.Equal(t, orgID, resp.Organization.ID)
	})

	t.Run("rejects empty reg_no with BadRequest", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.usecase.CreateOrganization(context.Background(), org.CreateOrganizationRequest{
			CallerID: callerID, RegNo: "  ", Name: "Тест",
		})
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
	})
}

// TestAddMember нь эрх олголтыг (owner/admin шаардлага) шалгана.
func TestAddMember(t *testing.T) {
	t.Run("owner can add a member", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleOwner}, nil).Once()
		f.repo.On("AddMember", mock.Anything, mock.MatchedBy(func(m *domain.OrganizationMembership) bool {
			return m.OrgID == orgID && m.UserID == targetID && m.Role == domain.OrgRoleMember
		})).Return(domain.OrganizationMembership{OrgID: orgID, UserID: targetID, Role: domain.OrgRoleMember}, nil).Once()

		resp, err := f.usecase.AddMember(context.Background(), org.AddMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: "",
		})
		require.NoError(t, err)
		assert.Equal(t, domain.OrgRoleMember, resp.Membership.Role)
	})

	t.Run("plain member is forbidden from adding", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleMember}, nil).Once()

		_, err := f.usecase.AddMember(context.Background(), org.AddMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleMember,
		})
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type)
	})

	t.Run("non-member is forbidden from adding", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{}, apperror.NotFound("membership not found")).Once()

		_, err := f.usecase.AddMember(context.Background(), org.AddMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID, Role: domain.OrgRoleMember,
		})
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type)
	})
}

// TestRemoveMember нь owner-ийг хасахаас сэргийлэхийг шалгана.
func TestRemoveMember(t *testing.T) {
	t.Run("cannot remove the owner", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleAdmin}, nil).Once()
		f.repo.On("GetMembership", mock.Anything, orgID, targetID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: targetID, Role: domain.OrgRoleOwner}, nil).Once()

		err := f.usecase.RemoveMember(context.Background(), org.RemoveMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID,
		})
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
	})

	t.Run("admin removes a member", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetMembership", mock.Anything, orgID, callerID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: callerID, Role: domain.OrgRoleAdmin}, nil).Once()
		f.repo.On("GetMembership", mock.Anything, orgID, targetID).
			Return(domain.OrganizationMembership{OrgID: orgID, UserID: targetID, Role: domain.OrgRoleMember}, nil).Once()
		f.repo.On("RemoveMember", mock.Anything, orgID, targetID).Return(nil).Once()

		err := f.usecase.RemoveMember(context.Background(), org.RemoveMemberRequest{
			CallerID: callerID, OrgID: orgID, UserID: targetID,
		})
		require.NoError(t, err)
	})
}

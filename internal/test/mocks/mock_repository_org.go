// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// repointerface.OrgRepository-д зориулсан гараар бичсэн mock бөгөөд төслийн
// бусад хэсгийн mock хэв маягтай таарахын тулд testify/mock ашигласан.
// Тестүүд үүнийг OrgRepository хүлээгдэж буй газар үүсгэдэг тул compile-time
// дахь гарын үсэг тааруулалт хүчинд ордог.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	"template/internal/business/domain"
)

// OrgRepository нь repointerface.OrgRepository-ийн mock юм.
type OrgRepository struct {
	mock.Mock
}

func (_m *OrgRepository) CreateOrg(ctx context.Context, in *domain.Organization) (domain.Organization, error) {
	ret := _m.Called(ctx, in)
	return ret.Get(0).(domain.Organization), ret.Error(1)
}

func (_m *OrgRepository) GetOrgByID(ctx context.Context, id string) (domain.Organization, error) {
	ret := _m.Called(ctx, id)
	return ret.Get(0).(domain.Organization), ret.Error(1)
}

func (_m *OrgRepository) GetOrgByRegNo(ctx context.Context, regNo string) (domain.Organization, error) {
	ret := _m.Called(ctx, regNo)
	return ret.Get(0).(domain.Organization), ret.Error(1)
}

func (_m *OrgRepository) ListOrgsForUser(ctx context.Context, userID string) ([]domain.Organization, error) {
	ret := _m.Called(ctx, userID)
	var r0 []domain.Organization
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]domain.Organization)
	}
	return r0, ret.Error(1)
}

func (_m *OrgRepository) GetMembership(ctx context.Context, orgID, userID string) (domain.OrganizationMembership, error) {
	ret := _m.Called(ctx, orgID, userID)
	return ret.Get(0).(domain.OrganizationMembership), ret.Error(1)
}

func (_m *OrgRepository) ListMembers(ctx context.Context, orgID string) ([]domain.OrganizationMembership, error) {
	ret := _m.Called(ctx, orgID)
	var r0 []domain.OrganizationMembership
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]domain.OrganizationMembership)
	}
	return r0, ret.Error(1)
}

func (_m *OrgRepository) AddMember(ctx context.Context, in *domain.OrganizationMembership) (domain.OrganizationMembership, error) {
	ret := _m.Called(ctx, in)
	return ret.Get(0).(domain.OrganizationMembership), ret.Error(1)
}

func (_m *OrgRepository) UpdateMemberRole(ctx context.Context, orgID, userID, role string) error {
	return _m.Called(ctx, orgID, userID, role).Error(0)
}

func (_m *OrgRepository) RemoveMember(ctx context.Context, orgID, userID string) error {
	return _m.Called(ctx, orgID, userID).Error(0)
}

type mockConstructorTestingTNewOrgRepository interface {
	mock.TestingT
	Cleanup(func())
}

// NewOrgRepository нь OrgRepository mock-ийн шинэ instance үүсгэж, testing
// interface болон cleanup-ийг бүртгэнэ (хүлээлтийг батална).
func NewOrgRepository(t mockConstructorTestingTNewOrgRepository) *OrgRepository {
	m := &OrgRepository{}
	m.Mock.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

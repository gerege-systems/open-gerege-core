// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// users.Usecase-д зориулсан гараар бичсэн mock бөгөөд төслийн бусад
// хэсгийн mock хэв маягтай таарахын тулд testify/mock ашигласан.
// Тестүүд үүнийг users.Usecase хүлээгдэж буй газар үүсгэдэг тул
// compile-time дахь гарын үсэг тааруулалт хүчинд ордог — зөрөх нь
// ажиллах үеийн гэнэтийн зүйлийн оронд build алдаа үүсгэдэг.

package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
)

// UsersUsecase нь users.Usecase-ийн mock юм.
type UsersUsecase struct {
	mock.Mock
}

func (_m *UsersUsecase) Store(ctx context.Context, req users.StoreRequest) (users.StoreResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(users.StoreResponse), ret.Error(1)
}

func (_m *UsersUsecase) GetByEmail(ctx context.Context, req users.GetByEmailRequest) (users.GetByEmailResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(users.GetByEmailResponse), ret.Error(1)
}

func (_m *UsersUsecase) GetByID(ctx context.Context, req users.GetByIDRequest) (users.GetByIDResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(users.GetByIDResponse), ret.Error(1)
}

func (_m *UsersUsecase) UpdatePassword(ctx context.Context, req users.UpdatePasswordRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *UsersUsecase) GetByNationalID(ctx context.Context, req users.GetByNationalIDRequest) (users.GetByNationalIDResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(users.GetByNationalIDResponse), ret.Error(1)
}

func (_m *UsersUsecase) UpsertFromEID(ctx context.Context, req users.UpsertFromEIDRequest) (users.UpsertFromEIDResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(users.UpsertFromEIDResponse), ret.Error(1)
}

func (_m *UsersUsecase) GetByGoogleSub(ctx context.Context, sub string) (domain.User, error) {
	ret := _m.Called(ctx, sub)
	return ret.Get(0).(domain.User), ret.Error(1)
}

func (_m *UsersUsecase) UnlinkGoogle(ctx context.Context, userID string) error {
	return _m.Called(ctx, userID).Error(0)
}

func (_m *UsersUsecase) LinkGoogleAccount(ctx context.Context, userID string, acct domain.GoogleAccount) error {
	return _m.Called(ctx, userID, acct).Error(0)
}

func (_m *UsersUsecase) Activate(ctx context.Context, req users.ActivateRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *UsersUsecase) List(ctx context.Context, req users.ListRequest) (users.ListResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(users.ListResponse), ret.Error(1)
}

func (_m *UsersUsecase) ListAdmins(ctx context.Context) (users.ListResponse, error) {
	ret := _m.Called(ctx)
	return ret.Get(0).(users.ListResponse), ret.Error(1)
}

func (_m *UsersUsecase) UpdateRole(ctx context.Context, req users.UpdateRoleRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *UsersUsecase) SetActive(ctx context.Context, req users.SetActiveRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *UsersUsecase) Delete(ctx context.Context, req users.DeleteRequest) error {
	return _m.Called(ctx, req).Error(0)
}

type mockConstructorTestingTNewUsersUsecase interface {
	mock.TestingT
	Cleanup(func())
}

func NewUsersUsecase(t mockConstructorTestingTNewUsersUsecase) *UsersUsecase {
	m := &UsersUsecase{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// auth.Usecase-д зориулсан гараар бичсэн mock бөгөөд төслийн бусад
// хэсэгт ашигладаг testify/mock хэв маягийг тусгасан. Handler тестүүд
// үүнийг auth.Usecase хүлээгдэж буй газар үүсгэдэг тул compile-time дахь
// гарын үсэг тааруулалт хүчинд ордог — зөрөх нь build алдаа үүсгэдэг.

package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"
	"template/internal/business/usecases/auth"
	"template/pkg/eid"
)

type AuthUsecase struct {
	mock.Mock
}

func (_m *AuthUsecase) Register(ctx context.Context, req auth.RegisterRequest) (auth.RegisterResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(auth.RegisterResponse), ret.Error(1)
}

func (_m *AuthUsecase) Login(ctx context.Context, req auth.LoginRequest) (auth.LoginResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(auth.LoginResponse), ret.Error(1)
}

func (_m *AuthUsecase) SendOTP(ctx context.Context, req auth.SendOTPRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *AuthUsecase) VerifyOTP(ctx context.Context, req auth.VerifyOTPRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *AuthUsecase) Refresh(ctx context.Context, req auth.RefreshRequest) (auth.LoginResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(auth.LoginResponse), ret.Error(1)
}

func (_m *AuthUsecase) Logout(ctx context.Context, req auth.LogoutRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *AuthUsecase) ChangePassword(ctx context.Context, req auth.ChangePasswordRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *AuthUsecase) ForgotPassword(ctx context.Context, req auth.ForgotPasswordRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *AuthUsecase) ResetPassword(ctx context.Context, req auth.ResetPasswordRequest) error {
	return _m.Called(ctx, req).Error(0)
}

func (_m *AuthUsecase) EIDStart(ctx context.Context) (auth.EIDStartResponse, error) {
	ret := _m.Called(ctx)
	return ret.Get(0).(auth.EIDStartResponse), ret.Error(1)
}

func (_m *AuthUsecase) EIDStartByNationalID(ctx context.Context, nationalID string) (auth.EIDStartResponse, error) {
	ret := _m.Called(ctx, nationalID)
	return ret.Get(0).(auth.EIDStartResponse), ret.Error(1)
}

func (_m *AuthUsecase) EIDPoll(ctx context.Context, req auth.EIDPollRequest) (auth.EIDPollResponse, error) {
	ret := _m.Called(ctx, req)
	return ret.Get(0).(auth.EIDPollResponse), ret.Error(1)
}

func (_m *AuthUsecase) EIDRepresentations(ctx context.Context, userID string) ([]eid.Representation, error) {
	ret := _m.Called(ctx, userID)
	var r0 []eid.Representation
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Representation)
	}
	return r0, ret.Error(1)
}

type mockConstructorTestingTNewAuthUsecase interface {
	mock.TestingT
	Cleanup(func())
}

func NewAuthUsecase(t mockConstructorTestingTNewAuthUsecase) *AuthUsecase {
	m := &AuthUsecase{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

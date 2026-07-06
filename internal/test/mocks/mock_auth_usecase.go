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

func (_m *AuthUsecase) EIDStart(ctx context.Context, callbackURL string) (auth.EIDStartResponse, error) {
	ret := _m.Called(ctx, callbackURL)
	return ret.Get(0).(auth.EIDStartResponse), ret.Error(1)
}

func (_m *AuthUsecase) EIDStartByNationalID(ctx context.Context, nationalID, callbackURL string) (auth.EIDStartResponse, error) {
	ret := _m.Called(ctx, nationalID, callbackURL)
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

func (_m *AuthUsecase) RegisterEIDOrganization(ctx context.Context, userID, regNo string) ([]eid.Representation, error) {
	ret := _m.Called(ctx, userID, regNo)
	var r0 []eid.Representation
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Representation)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) UnlinkEIDOrganization(ctx context.Context, userID, orgRegister string) ([]eid.Representation, error) {
	ret := _m.Called(ctx, userID, orgRegister)
	var r0 []eid.Representation
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Representation)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) ListEIDOrgSigners(ctx context.Context, userID, orgRegister string) ([]eid.Signer, error) {
	ret := _m.Called(ctx, userID, orgRegister)
	var r0 []eid.Signer
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Signer)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) AddEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo, role string) (*eid.SignersResult, error) {
	ret := _m.Called(ctx, userID, orgRegister, signerRegNo, role)
	var r0 *eid.SignersResult
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.SignersResult)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) RemoveEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo string) ([]eid.Signer, error) {
	ret := _m.Called(ctx, userID, orgRegister, signerRegNo)
	var r0 []eid.Signer
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Signer)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) ResendEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo string) (*eid.SignersResult, error) {
	ret := _m.Called(ctx, userID, orgRegister, signerRegNo)
	var r0 *eid.SignersResult
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.SignersResult)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) EIDSummary(ctx context.Context, userID string) (*eid.PersonSummary, error) {
	ret := _m.Called(ctx, userID)
	var r0 *eid.PersonSummary
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonSummary)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) EIDCertificates(ctx context.Context, userID string) (*eid.PersonCertificates, error) {
	ret := _m.Called(ctx, userID)
	var r0 *eid.PersonCertificates
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonCertificates)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) EIDDevices(ctx context.Context, userID string) (*eid.PersonDevices, error) {
	ret := _m.Called(ctx, userID)
	var r0 *eid.PersonDevices
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonDevices)
	}
	return r0, ret.Error(1)
}

func (_m *AuthUsecase) EIDActivity(ctx context.Context, userID string, limit, offset int) (*eid.PersonActivity, error) {
	ret := _m.Called(ctx, userID, limit, offset)
	var r0 *eid.PersonActivity
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonActivity)
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

func (_m *AuthUsecase) GoogleLogin(ctx context.Context, code, redirectURI string) (auth.GoogleLoginResponse, error) {
	ret := _m.Called(ctx, code, redirectURI)
	return ret.Get(0).(auth.GoogleLoginResponse), ret.Error(1)
}

func (_m *AuthUsecase) UnlinkGoogleFromUser(ctx context.Context, userID string) error {
	return _m.Called(ctx, userID).Error(0)
}

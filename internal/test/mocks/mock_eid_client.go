// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// eid.Client-д зориулсан гараар бичсэн mock бөгөөд төслийн бусад хэсгийн
// mock хэв маягтай таарахын тулд testify/mock ашигласан. Тестүүд үүнийг
// eid.Client хүлээгдэж буй газар үүсгэдэг тул compile-time дахь гарын үсэг
// тааруулалт хүчинд ордог.

package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"
	"template/pkg/eid"
)

// EIDClient нь eid.Client-ийн mock юм.
type EIDClient struct {
	mock.Mock
}

func (_m *EIDClient) QRInitiate(ctx context.Context, displayText, callbackURL, nonce string) (*eid.StartResult, error) {
	ret := _m.Called(ctx, displayText, callbackURL, nonce)
	var r0 *eid.StartResult
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.StartResult)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) Initiate(ctx context.Context, nationalID, displayText, callbackURL string) (*eid.StartResult, error) {
	ret := _m.Called(ctx, nationalID, displayText, callbackURL)
	var r0 *eid.StartResult
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.StartResult)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) Session(ctx context.Context, sessionID string, timeoutMs int) (*eid.SessionResult, error) {
	ret := _m.Called(ctx, sessionID, timeoutMs)
	var r0 *eid.SessionResult
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.SessionResult)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) Representations(ctx context.Context, personEtsi string) ([]eid.Representation, error) {
	ret := _m.Called(ctx, personEtsi)
	var r0 []eid.Representation
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Representation)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) AddRepresentation(ctx context.Context, personEtsi string, in eid.AddRepresentationInput) ([]eid.Representation, error) {
	ret := _m.Called(ctx, personEtsi, in)
	var r0 []eid.Representation
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Representation)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) RemoveRepresentation(ctx context.Context, personEtsi, orgRegister string) ([]eid.Representation, error) {
	ret := _m.Called(ctx, personEtsi, orgRegister)
	var r0 []eid.Representation
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Representation)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) OrgSigners(ctx context.Context, orgRegister, actingPersonEtsi string) ([]eid.Signer, error) {
	ret := _m.Called(ctx, orgRegister, actingPersonEtsi)
	var r0 []eid.Signer
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Signer)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) AddSigner(ctx context.Context, orgRegister, actingPersonEtsi string, in eid.AddSignerInput) (*eid.SignersResult, error) {
	ret := _m.Called(ctx, orgRegister, actingPersonEtsi, in)
	var r0 *eid.SignersResult
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.SignersResult)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) RemoveSigner(ctx context.Context, orgRegister, actingPersonEtsi, signerRegNo string) ([]eid.Signer, error) {
	ret := _m.Called(ctx, orgRegister, actingPersonEtsi, signerRegNo)
	var r0 []eid.Signer
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Signer)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) ResendSigner(ctx context.Context, orgRegister, actingPersonEtsi, signerRegNo string) (*eid.SignersResult, error) {
	ret := _m.Called(ctx, orgRegister, actingPersonEtsi, signerRegNo)
	var r0 *eid.SignersResult
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.SignersResult)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) UpdateOrgNameLatin(ctx context.Context, orgRegister, actingPersonEtsi, nameLatin string) ([]eid.Representation, error) {
	ret := _m.Called(ctx, orgRegister, actingPersonEtsi, nameLatin)
	var r0 []eid.Representation
	if v := ret.Get(0); v != nil {
		r0 = v.([]eid.Representation)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) PersonSummary(ctx context.Context, personEtsi string) (*eid.PersonSummary, error) {
	ret := _m.Called(ctx, personEtsi)
	var r0 *eid.PersonSummary
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonSummary)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) PersonCertificates(ctx context.Context, personEtsi string) (*eid.PersonCertificates, error) {
	ret := _m.Called(ctx, personEtsi)
	var r0 *eid.PersonCertificates
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonCertificates)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) PersonDevices(ctx context.Context, personEtsi string) (*eid.PersonDevices, error) {
	ret := _m.Called(ctx, personEtsi)
	var r0 *eid.PersonDevices
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonDevices)
	}
	return r0, ret.Error(1)
}

func (_m *EIDClient) PersonActivity(ctx context.Context, personEtsi string, limit, offset int) (*eid.PersonActivity, error) {
	ret := _m.Called(ctx, personEtsi, limit, offset)
	var r0 *eid.PersonActivity
	if v := ret.Get(0); v != nil {
		r0 = v.(*eid.PersonActivity)
	}
	return r0, ret.Error(1)
}

type mockConstructorTestingTNewEIDClient interface {
	mock.TestingT
	Cleanup(func())
}

func NewEIDClient(t mockConstructorTestingTNewEIDClient) *EIDClient {
	m := &EIDClient{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

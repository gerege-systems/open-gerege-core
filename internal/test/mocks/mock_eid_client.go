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

func (_m *EIDClient) Initiate(ctx context.Context, nationalID, displayText, nonce string) (*eid.StartResult, error) {
	ret := _m.Called(ctx, nationalID, displayText, nonce)
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

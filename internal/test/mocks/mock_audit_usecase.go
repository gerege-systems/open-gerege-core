// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// audit.Usecase-д зориулсан гараар бичсэн mock (handler тестэд).

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	audituc "template/internal/business/usecases/audit"
	repointerface "template/internal/datasources/repositories/interface"
)

// AuditUsecase нь audit.Usecase-ийн mock юм.
type AuditUsecase struct {
	mock.Mock
}

func (_m *AuditUsecase) RecordEvent(ctx context.Context, action, category, target string, metadata map[string]any) error {
	return _m.Called(ctx, action, category, target, metadata).Error(0)
}

func (_m *AuditUsecase) ListEvents(ctx context.Context, filter repointerface.AuditListFilter, limit, offset int) ([]repointerface.AuditLogRow, error) {
	ret := _m.Called(ctx, filter, limit, offset)
	var r0 []repointerface.AuditLogRow
	if v := ret.Get(0); v != nil {
		r0 = v.([]repointerface.AuditLogRow)
	}
	return r0, ret.Error(1)
}

func (_m *AuditUsecase) VerifyChain(ctx context.Context) (audituc.VerifyResult, error) {
	ret := _m.Called(ctx)
	return ret.Get(0).(audituc.VerifyResult), ret.Error(1)
}

type mockConstructorTestingTNewAuditUsecase interface {
	mock.TestingT
	Cleanup(func())
}

// NewAuditUsecase нь AuditUsecase mock-ийн шинэ instance үүсгэнэ.
func NewAuditUsecase(t mockConstructorTestingTNewAuditUsecase) *AuditUsecase {
	m := &AuditUsecase{}
	m.Mock.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// repointerface.SecurityEventRepository-д зориулсан гараар бичсэн mock бөгөөд
// төслийн бусад хэсгийн mock хэв маягтай таарахын тулд testify/mock ашигласан.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	repointerface "template/internal/datasources/repositories/interface"
)

// SecurityEventRepository нь repointerface.SecurityEventRepository-ийн mock юм.
type SecurityEventRepository struct {
	mock.Mock
}

func (_m *SecurityEventRepository) Ingest(ctx context.Context, e repointerface.SecurityEventRecord) error {
	return _m.Called(ctx, e).Error(0)
}

func (_m *SecurityEventRepository) List(ctx context.Context, limit, offset int) ([]repointerface.SecurityEventRecord, error) {
	ret := _m.Called(ctx, limit, offset)
	var r0 []repointerface.SecurityEventRecord
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]repointerface.SecurityEventRecord)
	}
	return r0, ret.Error(1)
}

type mockConstructorTestingTNewSecurityEventRepository interface {
	mock.TestingT
	Cleanup(func())
}

// NewSecurityEventRepository нь SecurityEventRepository mock-ийн шинэ instance
// үүсгэж, testing interface болон cleanup-ийг бүртгэнэ.
func NewSecurityEventRepository(t mockConstructorTestingTNewSecurityEventRepository) *SecurityEventRepository {
	m := &SecurityEventRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

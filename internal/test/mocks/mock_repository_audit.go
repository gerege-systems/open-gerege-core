// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// repointerface.AuditRepository-д зориулсан гараар бичсэн mock бөгөөд төслийн
// бусад хэсгийн mock хэв маягтай таарахын тулд testify/mock ашигласан.
// Тестүүд үүнийг AuditRepository хүлээгдэж буй газар үүсгэдэг тул compile-time
// дахь гарын үсэг тааруулалт хүчинд ордог.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/audit"
)

// AuditRepository нь repointerface.AuditRepository-ийн mock юм.
type AuditRepository struct {
	mock.Mock
}

func (_m *AuditRepository) Append(ctx context.Context, e audit.ChainEntry) (string, error) {
	ret := _m.Called(ctx, e)
	return ret.Get(0).(string), ret.Error(1)
}

func (_m *AuditRepository) List(ctx context.Context, filter repointerface.AuditListFilter, limit, offset int) ([]repointerface.AuditLogRow, error) {
	ret := _m.Called(ctx, filter, limit, offset)
	var r0 []repointerface.AuditLogRow
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]repointerface.AuditLogRow)
	}
	return r0, ret.Error(1)
}

func (_m *AuditRepository) VerifyChain(ctx context.Context) (bool, int64, error) {
	ret := _m.Called(ctx)
	return ret.Get(0).(bool), ret.Get(1).(int64), ret.Error(2)
}

type mockConstructorTestingTNewAuditRepository interface {
	mock.TestingT
	Cleanup(func())
}

// NewAuditRepository нь AuditRepository mock-ийн шинэ instance үүсгэж, testing
// interface болон cleanup-ийг бүртгэнэ (хүлээлтийг батална).
func NewAuditRepository(t mockConstructorTestingTNewAuditRepository) *AuditRepository {
	m := &AuditRepository{}
	m.Mock.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

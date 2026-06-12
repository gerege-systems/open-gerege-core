// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package audit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	audituc "template/internal/business/usecases/audit"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"
	"template/internal/test/mocks"
	pkgaudit "template/pkg/audit"
)

const actorID = "11111111-1111-1111-1111-111111111111"

// TestRecordEvent_ReadsActorFromRLS нь RecordEvent нь хүсэлтийн RLS context
// дахь actor-г уншиж repository.Append руу зөв дамжуулахыг шалгана.
func TestRecordEvent_ReadsActorFromRLS(t *testing.T) {
	repo := mocks.NewAuditRepository(t)
	uc := audituc.NewUsecase(repo)

	repo.On("Append", mock.Anything, mock.MatchedBy(func(e pkgaudit.ChainEntry) bool {
		return e.ActorUserID == actorID &&
			e.Action == "org.create" &&
			e.Category == "org" &&
			e.Target == "org-1" &&
			e.Metadata["reg_no"] == "1234567" &&
			!e.OccurredAt.IsZero()
	})).Return("hash-1", nil).Once()

	ctx := rls.WithUser(context.Background(), actorID)
	err := uc.RecordEvent(ctx, "org.create", "org", "org-1", map[string]any{"reg_no": "1234567"})
	require.NoError(t, err)
}

// TestRecordEvent_RejectsEmptyAction нь action хоосон бол BadRequest буцаахыг
// шалгана (repository дуудагдахгүй).
func TestRecordEvent_RejectsEmptyAction(t *testing.T) {
	repo := mocks.NewAuditRepository(t)
	uc := audituc.NewUsecase(repo)

	err := uc.RecordEvent(context.Background(), "", "org", "", nil)
	require.Error(t, err)
}

// TestVerifyChain_OK нь repository ok=true буцаавал VerifyResult.OK=true болохыг
// шалгана.
func TestVerifyChain_OK(t *testing.T) {
	repo := mocks.NewAuditRepository(t)
	uc := audituc.NewUsecase(repo)
	repo.On("VerifyChain", mock.Anything).Return(true, int64(0), nil).Once()

	res, err := uc.VerifyChain(context.Background())
	require.NoError(t, err)
	assert.True(t, res.OK)
	assert.Equal(t, int64(0), res.BrokenID)
}

// TestVerifyChain_Broken нь зөрчил илэрвэл broken_id-г дамжуулахыг шалгана.
func TestVerifyChain_Broken(t *testing.T) {
	repo := mocks.NewAuditRepository(t)
	uc := audituc.NewUsecase(repo)
	repo.On("VerifyChain", mock.Anything).Return(false, int64(42), nil).Once()

	res, err := uc.VerifyChain(context.Background())
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Equal(t, int64(42), res.BrokenID)
}

// TestListEvents_PropagatesFilter нь filter/limit/offset-г repository руу зөв
// дамжуулж, алдааг боохыг шалгана.
func TestListEvents_PropagatesFilter(t *testing.T) {
	repo := mocks.NewAuditRepository(t)
	uc := audituc.NewUsecase(repo)

	want := []repointerface.AuditLogRow{{ID: 1, Action: "org.create"}}
	repo.On("List", mock.Anything, repointerface.AuditListFilter{Action: "org.create"}, 50, 0).
		Return(want, nil).Once()

	got, err := uc.ListEvents(context.Background(), repointerface.AuditListFilter{Action: "org.create"}, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestListEvents_WrapsError нь repository алдааг InternalCause-аар боохыг шалгана.
func TestListEvents_WrapsError(t *testing.T) {
	repo := mocks.NewAuditRepository(t)
	uc := audituc.NewUsecase(repo)
	repo.On("List", mock.Anything, mock.Anything, 50, 0).Return(nil, errors.New("db down")).Once()

	_, err := uc.ListEvents(context.Background(), repointerface.AuditListFilter{}, 50, 0)
	require.Error(t, err)
}

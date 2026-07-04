// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Security event usecase-ийн unit тест: Ingest-ийн kind шаардлага + trim,
// repo алдааг InternalCause болгож нуух, List-ийн дамжуулалт.
package security_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/usecases/security"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/test/mocks"
)

func newFixture(t *testing.T) (security.Usecase, *mocks.SecurityEventRepository) {
	t.Helper()
	repo := mocks.NewSecurityEventRepository(t)
	return security.NewUsecase(repo), repo
}

func TestIngest(t *testing.T) {
	t.Run("empty kind → BadRequest, repo not called", func(t *testing.T) {
		uc, _ := newFixture(t)
		err := uc.Ingest(context.Background(), security.IngestRequest{Kind: "  "})
		require.Error(t, err)
		var d *apperror.DomainError
		require.True(t, errors.As(err, &d))
		assert.Equal(t, apperror.ErrTypeBadRequest, d.Type)
	})

	t.Run("trims fields and forwards to repo", func(t *testing.T) {
		uc, repo := newFixture(t)
		repo.On("Ingest", mock.Anything, mock.MatchedBy(func(e repointerface.SecurityEventRecord) bool {
			return e.Kind == "csp_violation" && e.Severity == "high" && e.Source == "web" && e.UserID == "u1"
		})).Return(nil).Once()

		err := uc.Ingest(context.Background(), security.IngestRequest{
			UserID: "u1", Kind: "  csp_violation ", Severity: " high ", Source: " web ",
		})
		require.NoError(t, err)
	})

	t.Run("repo error is hidden as Internal", func(t *testing.T) {
		uc, repo := newFixture(t)
		repo.On("Ingest", mock.Anything, mock.Anything).Return(errors.New("pq: table locked")).Once()

		err := uc.Ingest(context.Background(), security.IngestRequest{Kind: "x"})
		require.Error(t, err)
		var d *apperror.DomainError
		require.True(t, errors.As(err, &d))
		assert.Equal(t, apperror.ErrTypeInternal, d.Type)
		assert.Equal(t, "internal server error", d.Error())
	})
}

func TestList(t *testing.T) {
	t.Run("forwards rows", func(t *testing.T) {
		uc, repo := newFixture(t)
		want := []repointerface.SecurityEventRecord{{ID: 1, Kind: "login_fail"}}
		repo.On("List", mock.Anything, 50, 0).Return(want, nil).Once()

		got, err := uc.List(context.Background(), 50, 0)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("repo error is hidden as Internal", func(t *testing.T) {
		uc, repo := newFixture(t)
		repo.On("List", mock.Anything, 10, 0).Return(nil, errors.New("boom")).Once()

		_, err := uc.List(context.Background(), 10, 0)
		require.Error(t, err)
		var d *apperror.DomainError
		require.True(t, errors.As(err, &d))
		assert.Equal(t, apperror.ErrTypeInternal, d.Type)
	})
}

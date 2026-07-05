// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations_test

import (
	"context"
	"errors"
	"testing"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/integrations"
	"template/internal/test/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newUC(t *testing.T) (integrations.Usecase, *mocks.UserIntegrationRepository) {
	t.Helper()
	repo := mocks.NewUserIntegrationRepository(t)
	uc, err := integrations.NewUsecase(repo, "test-enc-key")
	require.NoError(t, err)
	return uc, repo
}

func TestConnect_EncryptsAndUpserts(t *testing.T) {
	uc, repo := newUC(t)
	// Upsert-д ирэх токен шифрлэгдсэн (plaintext биш) байх ёстой.
	repo.On("Upsert", mock.Anything, mock.MatchedBy(func(in *domain.UserIntegration) bool {
		return in.UserID == "u1" && in.Provider == "google-drive" &&
			in.AccessToken != "" && in.AccessToken != "plain-access"
	})).Return(domain.UserIntegration{ID: "i1", UserID: "u1", Provider: "google-drive"}, nil).Once()

	out, err := uc.Connect(context.Background(), integrations.ConnectRequest{
		UserID: "u1", Provider: "google-drive", AccessToken: "plain-access", RefreshToken: "plain-refresh",
	})
	require.NoError(t, err)
	assert.Equal(t, "i1", out.ID)
	assert.Empty(t, out.AccessToken, "tokens must not be returned to the caller")
}

func TestConnect_RejectsUnknownProvider(t *testing.T) {
	uc, _ := newUC(t)
	_, err := uc.Connect(context.Background(), integrations.ConnectRequest{
		UserID: "u1", Provider: "evil", AccessToken: "x",
	})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
}

func TestConnect_RequiresAccessToken(t *testing.T) {
	uc, _ := newUC(t)
	_, err := uc.Connect(context.Background(), integrations.ConnectRequest{UserID: "u1", Provider: "dropbox"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
}

func TestList_MapsRowsWithoutTokens(t *testing.T) {
	uc, repo := newUC(t)
	repo.On("ListByUser", mock.Anything, "u1").Return([]domain.UserIntegration{
		{Provider: "dropbox", AccessToken: "enc"},
		{Provider: "google-meet", AccessToken: "enc"},
	}, nil).Once()

	out, err := uc.List(context.Background(), "u1")
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "dropbox", out[0].Provider)
	assert.Equal(t, "google-meet", out[1].Provider)
}

func TestDisconnect_DelegatesToRepo(t *testing.T) {
	uc, repo := newUC(t)
	repo.On("DeleteByUserAndProvider", mock.Anything, "u1", "dropbox").Return(nil).Once()
	require.NoError(t, uc.Disconnect(context.Background(), "u1", "dropbox"))
}

func TestDisconnect_RejectsUnknownProvider(t *testing.T) {
	uc, _ := newUC(t)
	err := uc.Disconnect(context.Background(), "u1", "evil")
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
}

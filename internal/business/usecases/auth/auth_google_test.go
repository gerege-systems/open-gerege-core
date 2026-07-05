// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Google нэвтрэлт + eID холболтын usecase unit тест: холбогдсон account шууд
// нэвтрэх, эхний удаа link_token үүсгэх, exchange алдаа, EIDPoll дахь холболт.
package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/pkg/eid"
	"template/pkg/google"
)

func TestGoogleLogin(t *testing.T) {
	t.Run("linked account → direct login with tokens", func(t *testing.T) {
		f := newFixture(t)
		f.google.user = &google.User{Sub: "g-123", Email: "bat@gmail.com"}
		user := eidUser()
		f.users.On("GetByGoogleSub", mock.Anything, "g-123").Return(user, nil).Once()
		// Дараагийн нэвтрэлт бүрд профайлыг сүүлийн Google утгаар шинэчилнэ.
		f.users.On("LinkGoogleAccount", mock.Anything, user.ID,
			domain.GoogleAccount{Sub: "g-123", Email: "bat@gmail.com"}).Return(nil).Once()
		f.jwt.On("GenerateTokenPair", user.ID, false, user.RoleID, user.Email).Return(samplePair(), nil).Once()
		f.redis.On("Set", mock.Anything, "refresh:refresh-jti", "refresh-jti").Return(nil).Once()
		f.redis.On("Expire", mock.Anything, "refresh:refresh-jti", mock.AnythingOfType("time.Duration")).Return(nil).Once()

		res, err := f.usecase.GoogleLogin(context.Background(), "code", "https://app/cb")
		require.NoError(t, err)
		assert.True(t, res.Linked)
		assert.Equal(t, "access-tok", res.Login.AccessToken)
		assert.Equal(t, user.ID, res.Login.User.ID)
	})

	t.Run("first time → link token (needs eID)", func(t *testing.T) {
		f := newFixture(t)
		f.google.user = &google.User{Sub: "g-new", Email: "new@gmail.com"}
		f.users.On("GetByGoogleSub", mock.Anything, "g-new").
			Return(domain.User{}, apperror.NotFound("user not found")).Once()
		// link token нь Redis-д google_link:<random> түлхүүрээр хадгалагдана; утга
		// нь Google профайлын бүтэн JSON (Sub орсон).
		f.redis.On("Set", mock.Anything, mock.MatchedBy(func(k string) bool {
			return strings.HasPrefix(k, "google_link:")
		}), mock.MatchedBy(func(v string) bool {
			return strings.Contains(v, "g-new")
		})).Return(nil).Once()
		f.redis.On("Expire", mock.Anything, mock.MatchedBy(func(k string) bool {
			return strings.HasPrefix(k, "google_link:")
		}), mock.AnythingOfType("time.Duration")).Return(nil).Once()

		res, err := f.usecase.GoogleLogin(context.Background(), "code", "https://app/cb")
		require.NoError(t, err)
		assert.False(t, res.Linked)
		assert.NotEmpty(t, res.LinkToken)
		assert.Equal(t, "new@gmail.com", res.Email)
	})

	t.Run("exchange error → BadRequest", func(t *testing.T) {
		f := newFixture(t)
		f.google.err = errors.New("invalid_grant")

		_, err := f.usecase.GoogleLogin(context.Background(), "bad", "https://app/cb")
		requireDomainType(t, err, apperror.ErrTypeBadRequest)
	})
}

func TestEIDPollLinksGoogle(t *testing.T) {
	f := newFixture(t)
	user := eidUser()
	f.eid.On("Session", mock.Anything, "sess-1", mock.AnythingOfType("int")).
		Return(&eid.SessionResult{State: eid.StateComplete, Identity: &eid.Identity{CivilID: "УБ99887766"}}, nil).Once()
	f.users.On("UpsertFromEID", mock.Anything, mock.Anything).
		Return(users.UpsertFromEIDResponse{User: user}, nil).Once()
	// GoogleLinkToken байгаа тул холболт: GetDel → профайл JSON, дараа нь LinkGoogleAccount.
	gjson := `{"Sub":"g-new","Email":"new@gmail.com","EmailVerified":true,"Name":"New User","Picture":"https://pic"}`
	f.redis.On("GetDel", mock.Anything, "google_link:tok-1").Return(gjson, nil).Once()
	f.users.On("LinkGoogleAccount", mock.Anything, user.ID, domain.GoogleAccount{
		Sub: "g-new", Email: "new@gmail.com", EmailVerified: true, Name: "New User", Picture: "https://pic",
	}).Return(nil).Once()
	f.jwt.On("GenerateTokenPair", user.ID, false, user.RoleID, user.Email).Return(samplePair(), nil).Once()
	f.redis.On("Set", mock.Anything, "refresh:refresh-jti", "refresh-jti").Return(nil).Once()
	f.redis.On("Expire", mock.Anything, "refresh:refresh-jti", mock.AnythingOfType("time.Duration")).Return(nil).Once()

	resp, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-1", GoogleLinkToken: "tok-1"})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETE", resp.State)
}

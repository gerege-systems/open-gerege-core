// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// eID нэвтрэлтийн usecase давхаргын unit тестүүд (mock-той, DB-гүй): EIDStart
// (QR), EIDStartByNationalID (РД push), EIDPoll-ийн бүх session төлөв
// (RUNNING/EXPIRED/REFUSED/COMPLETE) ба алдааны замууд. Handler → usecase →
// eid.Client → users.Upsert → jwt → redis гинжийг mock-оор бүрэн шалгаж,
// апп-аас ирэх бодит хариунуудыг (SessionResult) дуурайна.
package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/pkg/eid"
	"template/pkg/jwt"
)

func eidUser() domain.User {
	return domain.User{ID: "eid-user-1", Username: "eid_уб99887766", CivilID: "уб99887766", RoleID: domain.RoleUser, Active: true}
}

// requireDomainType нь алдаа apperror.DomainError бөгөөд хүлээгдсэн төрөлтэй
// гэдгийг батална.
func requireDomainType(t *testing.T, err error, want apperror.ErrorType) {
	t.Helper()
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr), "apperror.DomainError хүлээж байсан, авсан: %v", err)
	assert.Equal(t, want, domErr.Type)
}

func jwtPairZero() jwt.TokenPair { return jwt.TokenPair{} }

func TestEIDStart(t *testing.T) {
	t.Run("success maps start result to response", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("QRInitiate", mock.Anything, "template.gerege.mn", "", mock.AnythingOfType("string")).
			Return(&eid.StartResult{
				SessionID:        "sess-1",
				VerificationCode: "1234",
				DeviceLinkURL:    "https://eidmongolia.mn/dl?deviceLinkType=QR&sessionToken=tok",
			}, nil).Once()

		resp, err := f.usecase.EIDStart(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "sess-1", resp.SessionID)
		assert.Equal(t, "1234", resp.VerificationCode)
		assert.Contains(t, resp.DeviceLinkURL, "sessionToken=tok")
	})

	t.Run("provider 4xx surfaces as BadRequest", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("QRInitiate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, eid.ErrInitiateRejected).Once()

		_, err := f.usecase.EIDStart(context.Background())
		requireDomainType(t, err, apperror.ErrTypeBadRequest)
	})

	t.Run("provider network error surfaces as Internal", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("QRInitiate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("dial tcp: timeout")).Once()

		_, err := f.usecase.EIDStart(context.Background())
		requireDomainType(t, err, apperror.ErrTypeInternal)
	})
}

func TestEIDStartByNationalID(t *testing.T) {
	t.Run("empty national_id is rejected before any provider call", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.usecase.EIDStartByNationalID(context.Background(), "  ")
		requireDomainType(t, err, apperror.ErrTypeBadRequest)
	})

	t.Run("success returns session without device link (push flow)", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("Initiate", mock.Anything, "УБ99887766", "template.gerege.mn", mock.AnythingOfType("string")).
			Return(&eid.StartResult{SessionID: "sess-2", VerificationCode: "5678"}, nil).Once()

		resp, err := f.usecase.EIDStartByNationalID(context.Background(), "УБ99887766")
		require.NoError(t, err)
		assert.Equal(t, "sess-2", resp.SessionID)
		assert.Equal(t, "5678", resp.VerificationCode)
		assert.Empty(t, resp.DeviceLinkURL)
	})

	t.Run("unknown citizen (4xx) surfaces as BadRequest", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("Initiate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, eid.ErrInitiateRejected).Once()

		_, err := f.usecase.EIDStartByNationalID(context.Background(), "УБ00000000")
		requireDomainType(t, err, apperror.ErrTypeBadRequest)
	})

	t.Run("provider error surfaces as Internal", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("Initiate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("connection refused")).Once()

		_, err := f.usecase.EIDStartByNationalID(context.Background(), "УБ11111111")
		requireDomainType(t, err, apperror.ErrTypeInternal)
	})
}

func TestEIDPoll(t *testing.T) {
	t.Run("empty session_id is rejected", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: ""})
		requireDomainType(t, err, apperror.ErrTypeBadRequest)
	})

	t.Run("provider error surfaces as Internal", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("Session", mock.Anything, "sess-x", mock.AnythingOfType("int")).
			Return(nil, errors.New("timeout")).Once()

		_, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-x"})
		requireDomainType(t, err, apperror.ErrTypeInternal)
	})

	// Терминал биш ба terminal-fail төлвүүд — зөвхөн State буцаах ба ямар ч
	// хэрэглэгч үүсгэхгүй / токен олгохгүй (mock-ууд AssertExpectations-аар
	// нэмэлт дуудлага байхгүйг батална).
	for _, state := range []string{eid.StateRunning, eid.StateExpired, eid.StateRefused} {
		t.Run("non-complete state "+state+" returns state only", func(t *testing.T) {
			f := newFixture(t)
			f.eid.On("Session", mock.Anything, "sess-1", mock.AnythingOfType("int")).
				Return(&eid.SessionResult{State: state}, nil).Once()

			resp, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-1"})
			require.NoError(t, err)
			assert.Equal(t, state, resp.State)
			assert.Empty(t, resp.AccessToken)
			assert.Empty(t, resp.User.ID)
		})
	}

	t.Run("COMPLETE without identity is an Internal error", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("Session", mock.Anything, "sess-1", mock.AnythingOfType("int")).
			Return(&eid.SessionResult{State: eid.StateComplete, Identity: nil}, nil).Once()

		_, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-1"})
		requireDomainType(t, err, apperror.ErrTypeInternal)
	})

	t.Run("COMPLETE with identity upserts user and mints token pair", func(t *testing.T) {
		f := newFixture(t)
		user := eidUser()
		f.eid.On("Session", mock.Anything, "sess-1", mock.AnythingOfType("int")).
			Return(&eid.SessionResult{State: eid.StateComplete, Identity: &eid.Identity{
				CivilID: "УБ99887766", GivenName: "Бат", Surname: "Дорж",
				GivenNameEn: "Bat", SurnameEn: "Dorj", KYCLevel: "QUALIFIED",
			}}, nil).Once()
		// civil_id-г субьект болгож (жижиг үсгээр) хэрэглэгч бүтээнэ.
		f.users.On("UpsertFromEID", mock.Anything, mock.MatchedBy(func(req users.UpsertFromEIDRequest) bool {
			return req.User != nil && req.User.CivilID == "уб99887766" && req.User.FirstName == "Бат"
		})).Return(users.UpsertFromEIDResponse{User: user}, nil).Once()
		f.jwt.On("GenerateTokenPair", user.ID, false, user.RoleID, user.Email).Return(samplePair(), nil).Once()
		f.redis.On("Set", mock.Anything, "refresh:refresh-jti", "refresh-jti").Return(nil).Once()
		f.redis.On("Expire", mock.Anything, "refresh:refresh-jti", mock.AnythingOfType("time.Duration")).Return(nil).Once()

		resp, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-1"})
		require.NoError(t, err)
		assert.Equal(t, eid.StateComplete, resp.State)
		assert.Equal(t, user.ID, resp.User.ID)
		assert.Equal(t, "access-tok", resp.AccessToken)
		assert.Equal(t, "refresh-tok", resp.RefreshToken)
	})

	t.Run("upsert failure propagates", func(t *testing.T) {
		f := newFixture(t)
		f.eid.On("Session", mock.Anything, "sess-1", mock.AnythingOfType("int")).
			Return(&eid.SessionResult{State: eid.StateComplete, Identity: &eid.Identity{CivilID: "УБ99887766"}}, nil).Once()
		f.users.On("UpsertFromEID", mock.Anything, mock.Anything).
			Return(users.UpsertFromEIDResponse{}, apperror.InternalCause(errors.New("db down"))).Once()

		_, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-1"})
		requireDomainType(t, err, apperror.ErrTypeInternal)
	})

	t.Run("token generation failure surfaces as Internal", func(t *testing.T) {
		f := newFixture(t)
		user := eidUser()
		f.eid.On("Session", mock.Anything, "sess-1", mock.AnythingOfType("int")).
			Return(&eid.SessionResult{State: eid.StateComplete, Identity: &eid.Identity{CivilID: "УБ99887766"}}, nil).Once()
		f.users.On("UpsertFromEID", mock.Anything, mock.Anything).
			Return(users.UpsertFromEIDResponse{User: user}, nil).Once()
		f.jwt.On("GenerateTokenPair", user.ID, false, user.RoleID, user.Email).
			Return(jwtPairZero(), errors.New("sign error")).Once()

		_, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-1"})
		requireDomainType(t, err, apperror.ErrTypeInternal)
	})
}

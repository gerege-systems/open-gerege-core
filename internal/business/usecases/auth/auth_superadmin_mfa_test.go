// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// MFA хаалтын (gate) unit тест: super admin БҮР Google/eID нэвтрэлтээр session
// АВАХГҮЙ, зөвхөн mfa_token авна (MFA бүртгэл нь superadmin_accounts satellite-д
// тул requiresMFA нь users дээрх флаг уншихгүй — super admin гэдгээр л шийднэ,
// fail-closed). Энгийн хэрэглэгч/админы нэвтрэлт огт өөрчлөгдөхгүй.
//
// Тэмдэглэл: mockery-ийн mock-ууд нь тохируулаагүй дуудлагад унадаг тул
// "jwt.GenerateTokenPair дуудагдаагүй" гэдэг нь mock-ийг тохируулаагүйгээр
// (доорх MFA тестүүдэд) автоматаар батлагдана — session олгогдоогүйн баталгаа.
package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/pkg/eid"
	"template/pkg/google"
)

// superadminMFAUser нь MFA идэвхжүүлсэн super admin.
func superadminMFAUser() domain.User {
	return domain.User{
		ID: "sa-1", Username: "eid_уб11223344", CivilID: "уб11223344",
		Email: "sa@dgov.mn", RoleID: domain.RoleSuperAdmin, MFAEnabled: true, Active: true,
	}
}

// isMFAKey нь superadmin_mfa:<token> түлхүүрийг таних matcher.
func isMFAKey(k string) bool { return strings.HasPrefix(k, "superadmin_mfa:") }

func TestGoogleLoginSuperadminMFAGate(t *testing.T) {
	t.Run("MFA-тай super admin → session БИШ, mfa_token", func(t *testing.T) {
		f := newFixture(t)
		user := superadminMFAUser()
		f.google.user = &google.User{Sub: "g-sa", Email: user.Email}
		f.users.On("GetByGoogleSub", mock.Anything, "g-sa").Return(user, nil).Once()
		f.users.On("LinkGoogleAccount", mock.Anything, user.ID, mock.Anything).Return(nil).Once()
		// mfa_token нь superadmin_mfa:<random> түлхүүрээр user_id рүү заана.
		f.redis.On("Set", mock.Anything, mock.MatchedBy(isMFAKey), user.ID).Return(nil).Once()
		f.redis.On("Expire", mock.Anything, mock.MatchedBy(isMFAKey), mock.AnythingOfType("time.Duration")).
			Return(nil).Once()

		res, err := f.usecase.GoogleLogin(context.Background(), "code", "https://app/cb")
		require.NoError(t, err)
		assert.True(t, res.MFARequired)
		assert.NotEmpty(t, res.MFAToken)
		// Токен ОЛГОГДООГҮЙ байх ёстой.
		assert.Empty(t, res.Login.AccessToken)
		assert.Empty(t, res.Login.RefreshToken)
	})

	t.Run("super admin → MFAEnabled флагаас үл хамааран үргэлж MFA gate (fail-closed)", func(t *testing.T) {
		f := newFixture(t)
		user := superadminMFAUser()
		user.MFAEnabled = false // хуучин users-флаг — шинэ загварт requiresMFA үүнийг үл харгалзана
		f.google.user = &google.User{Sub: "g-sa", Email: user.Email}
		f.users.On("GetByGoogleSub", mock.Anything, "g-sa").Return(user, nil).Once()
		f.users.On("LinkGoogleAccount", mock.Anything, user.ID, mock.Anything).Return(nil).Once()
		// Session БИШ — super admin гэдгээр л MFA gate дамжина.
		f.redis.On("Set", mock.Anything, mock.MatchedBy(isMFAKey), user.ID).Return(nil).Once()
		f.redis.On("Expire", mock.Anything, mock.MatchedBy(isMFAKey), mock.AnythingOfType("time.Duration")).
			Return(nil).Once()

		res, err := f.usecase.GoogleLogin(context.Background(), "code", "https://app/cb")
		require.NoError(t, err)
		assert.True(t, res.MFARequired)
		assert.NotEmpty(t, res.MFAToken)
		assert.Empty(t, res.Login.AccessToken)
	})

	t.Run("Redis алдаа → fail-closed (нэвтрүүлэхгүй)", func(t *testing.T) {
		f := newFixture(t)
		user := superadminMFAUser()
		f.google.user = &google.User{Sub: "g-sa", Email: user.Email}
		f.users.On("GetByGoogleSub", mock.Anything, "g-sa").Return(user, nil).Once()
		f.users.On("LinkGoogleAccount", mock.Anything, user.ID, mock.Anything).Return(nil).Once()
		f.redis.On("Set", mock.Anything, mock.MatchedBy(isMFAKey), user.ID).
			Return(assert.AnError).Once()

		// mfa_token хадгалагдаагүй бол MFA-г давах боломжгүй тул нэвтрэлт
		// АМЖИЛТГҮЙ болно (MFA-г алгасаж session олгохгүй).
		_, err := f.usecase.GoogleLogin(context.Background(), "code", "https://app/cb")
		require.Error(t, err)
	})
}

func TestEIDPollSuperadminMFAGate(t *testing.T) {
	f := newFixture(t)
	user := superadminMFAUser()
	f.eid.On("Session", mock.Anything, "sess-sa", mock.AnythingOfType("int")).
		Return(&eid.SessionResult{State: eid.StateComplete, Identity: &eid.Identity{CivilID: user.CivilID}}, nil).Once()
	f.users.On("UpsertFromEID", mock.Anything, mock.Anything).
		Return(users.UpsertFromEIDResponse{User: user}, nil).Once()
	f.redis.On("Set", mock.Anything, mock.MatchedBy(isMFAKey), user.ID).Return(nil).Once()
	f.redis.On("Expire", mock.Anything, mock.MatchedBy(isMFAKey), mock.AnythingOfType("time.Duration")).
		Return(nil).Once()

	resp, err := f.usecase.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: "sess-sa"})
	require.NoError(t, err)
	assert.Equal(t, eid.StateComplete, resp.State)
	assert.True(t, resp.MFARequired)
	assert.NotEmpty(t, resp.MFAToken)
	// eID баталгаажсан ч session олгогдоогүй.
	assert.Empty(t, resp.AccessToken)
	assert.Empty(t, resp.RefreshToken)
	assert.Empty(t, resp.User.ID)
}

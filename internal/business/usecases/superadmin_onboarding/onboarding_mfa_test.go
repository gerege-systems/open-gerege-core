// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Super admin MFA нэвтрэлтийн (2 дахь шат) unit тест: зөв TOTP → session;
// нөөц код → session + код хэрэглэгдэнэ; буруу код / хүчингүй токен →
// татгалзана.
package onboarding_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pqtotp "github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	onboarding "template/internal/business/usecases/superadmin_onboarding"
	"template/internal/test/mocks"
	"template/pkg/crypto"
	"template/pkg/jwt"
	"template/pkg/recovery"
	"template/pkg/totp"
)

// testEncKey нь тестийн TOTP secret шифрлэх түлхүүр (production-д INTEGRATION_ENC_KEY).
const testEncKey = "unit-test-encryption-key"

// fakeRecovery нь RecoveryCodeRepository-ийн тест fake. Өгөгдмөл нь "код
// олдсонгүй" (NotFound) — иймээс TOTP-ийн тестүүд санамсаргүйгээр нөөц кодын
// замаар амжилттай болж чадахгүй.
type fakeRecovery struct {
	replaced []string
	consumed []string
	active   []domain.RecoveryCode
	// validHash нь энэ hash-тай кодыг л хүлээн авна (хоосон бол бүгдийг татгалзана).
	validHash string
}

func (f *fakeRecovery) Replace(_ context.Context, _ string, hashes []string) error {
	f.replaced = hashes
	return nil
}

func (f *fakeRecovery) ListActive(_ context.Context, _ string) ([]domain.RecoveryCode, error) {
	return f.active, nil
}

func (f *fakeRecovery) Consume(_ context.Context, _, hash string) error {
	if f.validHash != "" && hash == f.validHash {
		f.consumed = append(f.consumed, hash)
		f.validHash = "" // нэг удаагийн — дахин ажиллахгүй
		return nil
	}
	return apperror.NotFound("recovery code not found or already used")
}

// fakeInvites нь SuperadminInviteRepository-ийн тест fake (MFA урсгалд хэрэглэгддэггүй).
type fakeInvites struct{}

func (f *fakeInvites) Create(_ context.Context, email, invitedBy string) (domain.SuperadminInvite, error) {
	return domain.SuperadminInvite{Email: email, InvitedBy: invitedBy}, nil
}
func (f *fakeInvites) List(_ context.Context) ([]domain.SuperadminInvite, error) { return nil, nil }
func (f *fakeInvites) GetByEmail(_ context.Context, _ string) (domain.SuperadminInvite, error) {
	return domain.SuperadminInvite{}, apperror.NotFound("superadmin invite not found")
}
func (f *fakeInvites) Delete(_ context.Context, _ string) error       { return nil }
func (f *fakeInvites) MarkAccepted(_ context.Context, _ string) error { return nil }

// fakeSuperadminAccts нь SuperadminAccountRepository-ийн тест fake. found=false бол
// NotFound (account алга → fail-closed) — иймээс MFA урсгал зөвхөн тохируулсан үед амжина.
type fakeSuperadminAccts struct {
	acct  domain.SuperadminAccount
	found bool
}

func (f *fakeSuperadminAccts) Get(_ context.Context, _ string) (domain.SuperadminAccount, error) {
	if !f.found {
		return domain.SuperadminAccount{}, apperror.NotFound("superadmin account not found")
	}
	return f.acct, nil
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

type fixture struct {
	usecase  onboarding.Usecase
	users    *mocks.UserRepository
	redis    *mocks.RedisCache
	jwt      *mocks.JWTService
	recovery *fakeRecovery
	accts    *fakeSuperadminAccts
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	usersRepo := mocks.NewUserRepository(t)
	redis := mocks.NewRedisCache(t)
	jwtSvc := mocks.NewJWTService(t)
	rec := &fakeRecovery{}
	accts := &fakeSuperadminAccts{}

	// eID / verify client-ууд MFA урсгалд хэрэглэгддэггүй тул nil.
	uc, err := onboarding.NewUsecase(
		nil, nil, nil,
		usersRepo, rec, accts, &fakeInvites{},
		jwtSvc, redis, testEncKey,
		onboarding.Config{MFAMaxAttempts: 5, Issuer: "DAN-Test"},
	)
	require.NoError(t, err)

	return &fixture{usecase: uc, users: usersRepo, redis: redis, jwt: jwtSvc, recovery: rec, accts: accts}
}

func samplePair() jwt.TokenPair {
	return jwt.TokenPair{
		AccessToken:      "access-tok",
		RefreshToken:     "refresh-tok",
		AccessExpiresAt:  time.Now().Add(time.Hour),
		RefreshExpiresAt: time.Now().Add(24 * time.Hour),
		AccessJTI:        "access-jti",
		RefreshJTI:       "refresh-jti",
	}
}

// mfaUser нь MFA идэвхтэй super admin-г (шифрлэгдсэн TOTP secret-тэй) буцаана.
func mfaUser(t *testing.T, secret string) domain.User {
	t.Helper()
	c, err := crypto.New(testEncKey)
	require.NoError(t, err)
	enc, err := c.Encrypt(secret)
	require.NoError(t, err)
	return domain.User{
		ID: "sa-1", Username: "eid_уб11223344", Email: "sa@dgov.mn",
		RoleID: domain.RoleSuperAdmin, MFAEnabled: true, Active: true, TOTPSecret: enc,
	}
}

// expectSessionMinted нь амжилттай MFA-ийн дараах session олголтыг тохируулна.
func expectSessionMinted(f *fixture, user domain.User) {
	f.jwt.On("GenerateTokenPair", user.ID, true, user.RoleID, user.Email).Return(samplePair(), nil).Once()
	f.redis.On("Set", mock.Anything, "refresh:refresh-jti", "refresh-jti").Return(nil).Once()
	f.redis.On("Expire", mock.Anything, "refresh:refresh-jti", mock.AnythingOfType("time.Duration")).
		Return(nil).Once()
}

// expectTokenLookup нь mfa_token → user_id хайлт + оролдлогын тоологчийг тохируулна.
func expectTokenLookup(f *fixture, user domain.User) {
	f.redis.On("Get", mock.Anything, "superadmin_mfa:tok-1").Return(user.ID, nil).Once()
	f.redis.On("Incr", mock.Anything, "superadmin_mfa_attempts:tok-1").Return(int64(1), nil).Once()
	f.redis.On("Expire", mock.Anything, "superadmin_mfa_attempts:tok-1", mock.AnythingOfType("time.Duration")).
		Return(nil).Once()
	f.users.On("GetByID", mock.Anything, user.ID).Return(user, nil).Once()
	// MFA бүртгэл (TOTP secret) нь satellite-д — user-ийн MFA талбараас fake-д тохируулна.
	f.accts.found = true
	f.accts.acct = domain.SuperadminAccount{UserID: user.ID, MFAEnabled: user.MFAEnabled, TOTPSecret: user.TOTPSecret}
}

func TestSuperadminMFA(t *testing.T) {
	t.Run("зөв TOTP код → session олгоно", func(t *testing.T) {
		f := newFixture(t)
		secret, _, err := totp.Generate("DAN-Test", "sa@dgov.mn")
		require.NoError(t, err)
		user := mfaUser(t, secret)
		code, err := pqtotp.GenerateCode(secret, time.Now())
		require.NoError(t, err)

		expectTokenLookup(f, user)
		// Амжилттай → токен ба тоологч цэвэрлэгдэнэ (нэг удаагийн).
		f.redis.On("Del", mock.Anything, "superadmin_mfa:tok-1").Return(nil).Once()
		f.redis.On("Del", mock.Anything, "superadmin_mfa_attempts:tok-1").Return(nil).Once()
		expectSessionMinted(f, user)

		res, err := f.usecase.SuperadminMFA(context.Background(), onboarding.MFARequest{
			MFAToken: "tok-1", Code: code,
		})
		require.NoError(t, err)
		assert.Equal(t, "access-tok", res.AccessToken)
		assert.Equal(t, "refresh-tok", res.RefreshToken)
		assert.Equal(t, user.ID, res.User.ID)
		assert.False(t, res.UsedRecoveryCode)
		assert.Empty(t, f.recovery.consumed, "TOTP-ээр нэвтэрсэн үед нөөц код хэрэглэгдэх ёсгүй")
	})

	t.Run("нөөц код → session олгож, кодыг хэрэглэнэ (нэг удаагийн)", func(t *testing.T) {
		f := newFixture(t)
		secret, _, err := totp.Generate("DAN-Test", "sa@dgov.mn")
		require.NoError(t, err)
		user := mfaUser(t, secret)

		const recoveryCode = "ABCD-EFGH"
		f.recovery.validHash = recovery.Hash(recoveryCode)
		f.recovery.active = []domain.RecoveryCode{{ID: "r1"}, {ID: "r2"}} // үлдсэн кодууд

		expectTokenLookup(f, user)
		f.redis.On("Del", mock.Anything, "superadmin_mfa:tok-1").Return(nil).Once()
		f.redis.On("Del", mock.Anything, "superadmin_mfa_attempts:tok-1").Return(nil).Once()
		expectSessionMinted(f, user)

		res, err := f.usecase.SuperadminMFA(context.Background(), onboarding.MFARequest{
			MFAToken: "tok-1", Code: recoveryCode,
		})
		require.NoError(t, err)
		assert.Equal(t, "access-tok", res.AccessToken)
		assert.True(t, res.UsedRecoveryCode)
		assert.Equal(t, 2, res.RecoveryCodesLeft)
		// Код нь hash хэлбэрээр хэрэглэгдсэн байх ёстой (энгийн текст биш).
		require.Len(t, f.recovery.consumed, 1)
		assert.Equal(t, recovery.Hash(recoveryCode), f.recovery.consumed[0])
	})

	t.Run("нөөц код нь нормчлолоос үл хамаарна (жижиг үсэг/зайгүй)", func(t *testing.T) {
		f := newFixture(t)
		user := mfaUser(t, "")
		user.TOTPSecret = "" // зөвхөн нөөц кодын зам

		f.recovery.validHash = recovery.Hash("ABCD-EFGH")
		expectTokenLookup(f, user)
		f.redis.On("Del", mock.Anything, "superadmin_mfa:tok-1").Return(nil).Once()
		f.redis.On("Del", mock.Anything, "superadmin_mfa_attempts:tok-1").Return(nil).Once()
		expectSessionMinted(f, user)

		res, err := f.usecase.SuperadminMFA(context.Background(), onboarding.MFARequest{
			MFAToken: "tok-1", Code: "abcd efgh",
		})
		require.NoError(t, err)
		assert.True(t, res.UsedRecoveryCode)
	})

	t.Run("буруу код → BadRequest, session олгохгүй", func(t *testing.T) {
		f := newFixture(t)
		secret, _, err := totp.Generate("DAN-Test", "sa@dgov.mn")
		require.NoError(t, err)
		user := mfaUser(t, secret)

		expectTokenLookup(f, user)

		// jwt.GenerateTokenPair-ийг тохируулаагүй — дуудагдвал тест унана.
		_, err = f.usecase.SuperadminMFA(context.Background(), onboarding.MFARequest{
			MFAToken: "tok-1", Code: "000000",
		})
		requireDomainType(t, err, apperror.ErrTypeBadRequest)
	})

	t.Run("хүчингүй/хугацаа дууссан mfa_token → Forbidden", func(t *testing.T) {
		f := newFixture(t)
		f.redis.On("Get", mock.Anything, "superadmin_mfa:bad").Return("", assert.AnError).Once()

		_, err := f.usecase.SuperadminMFA(context.Background(), onboarding.MFARequest{
			MFAToken: "bad", Code: "123456",
		})
		requireDomainType(t, err, apperror.ErrTypeForbidden)
	})

	t.Run("оролдлого хэтэрвэл токен цуцлагдана", func(t *testing.T) {
		f := newFixture(t)
		user := mfaUser(t, "")
		f.redis.On("Get", mock.Anything, "superadmin_mfa:tok-1").Return(user.ID, nil).Once()
		// MFAMaxAttempts = 5; 6 дахь оролдлого хэтэрсэн.
		f.redis.On("Incr", mock.Anything, "superadmin_mfa_attempts:tok-1").Return(int64(6), nil).Once()
		f.redis.On("Del", mock.Anything, "superadmin_mfa:tok-1").Return(nil).Once()
		f.redis.On("Del", mock.Anything, "superadmin_mfa_attempts:tok-1").Return(nil).Once()

		_, err := f.usecase.SuperadminMFA(context.Background(), onboarding.MFARequest{
			MFAToken: "tok-1", Code: "123456",
		})
		requireDomainType(t, err, apperror.ErrTypeForbidden)
	})

	t.Run("super admin биш хэрэглэгч → Forbidden", func(t *testing.T) {
		f := newFixture(t)
		user := mfaUser(t, "")
		user.RoleID = domain.RoleUser // токен олгогдсоноос хойш эрх хасагдсан
		f.redis.On("Get", mock.Anything, "superadmin_mfa:tok-1").Return(user.ID, nil).Once()
		f.redis.On("Incr", mock.Anything, "superadmin_mfa_attempts:tok-1").Return(int64(1), nil).Once()
		f.redis.On("Expire", mock.Anything, "superadmin_mfa_attempts:tok-1", mock.AnythingOfType("time.Duration")).
			Return(nil).Once()
		f.users.On("GetByID", mock.Anything, user.ID).Return(user, nil).Once()
		f.redis.On("Del", mock.Anything, "superadmin_mfa:tok-1").Return(nil).Once()

		_, err := f.usecase.SuperadminMFA(context.Background(), onboarding.MFARequest{
			MFAToken: "tok-1", Code: "123456",
		})
		requireDomainType(t, err, apperror.ErrTypeForbidden)
	})
}

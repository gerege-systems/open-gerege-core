// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package onboarding_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	onboarding "github.com/gerege-systems/open-gerege-core/core/business/usecases/superadmin_onboarding"
	repointerface "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/interface"
	"github.com/gerege-systems/open-gerege-core/core/test/mocks"
	"github.com/gerege-systems/open-gerege-core/pkg/oidc"
)

// fakeSSO нь SSOClient-ийн тест хувилбар.
type fakeSSO struct {
	configured  bool
	exchangeErr error
	info        oidc.UserInfo
	infoErr     error
}

func (f *fakeSSO) Configured() bool { return f.configured }
func (f *fakeSSO) Exchange(_ context.Context, _ string) (accessToken, idToken string, err error) {
	if f.exchangeErr != nil {
		return "", "", f.exchangeErr
	}
	return "access-tok", "id-tok", nil
}
func (f *fakeSSO) UserInfo(_ context.Context, _ string) (oidc.UserInfo, error) {
	return f.info, f.infoErr
}

// acctsWith нь "super admin байгаа эсэх"-ийг удирдах fake.
type acctsWith struct {
	fakeSuperadminAccts
	any bool
}

func (a *acctsWith) AnySuperAdminExists(_ context.Context) (bool, error) { return a.any, nil }

// invitesWith нь заасан и-мэйлд урилга БУЦААДАГ fake.
type invitesWith struct {
	fakeInvites
	email    string
	accepted bool
}

func (i *invitesWith) GetByEmail(_ context.Context, email string) (domain.SuperadminInvite, error) {
	if email != i.email {
		return domain.SuperadminInvite{}, apperror.NotFound("superadmin invite not found")
	}
	inv := domain.SuperadminInvite{Email: i.email}
	if i.accepted {
		now := time.Now().UTC()
		inv.AcceptedAt = &now
	}
	return inv, nil
}

func ssoUsecase(t *testing.T, sso onboarding.SSOClient, invites interface {
	GetByEmail(context.Context, string) (domain.SuperadminInvite, error)
}, accts repointerface.SuperadminAccountRepository) onboarding.Usecase {
	t.Helper()
	redis := mocks.NewRedisCache(t)
	// Bootstrap зам нь pending session хадгалах хүртэл ЯВНА — тэр нь зөв
	// (хаалга нээгдсэн). Урилгагүй замууд энд хүрэхгүй тул Maybe().
	redis.On("Set", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	redis.On("Expire", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	uc, err := onboarding.NewUsecase(
		nil, sso, nil, nil,
		mocks.NewUserRepository(t), &fakeRecovery{}, accts,
		invites.(interface {
			Create(context.Context, string, string) (domain.SuperadminInvite, error)
			List(context.Context) ([]domain.SuperadminInvite, error)
			GetByEmail(context.Context, string) (domain.SuperadminInvite, error)
			Delete(context.Context, string) error
			MarkAccepted(context.Context, string) error
		}),
		mocks.NewJWTService(t), redis, testEncKey,
		onboarding.Config{Issuer: "DAN-Test"},
	)
	require.NoError(t, err)
	return uc
}

// Урилгагүй и-мэйл нь SSO-гоор ирсэн ч ХААЛГАНААС давахгүй — энэ бол super
// admin болох цорын ганц хаалга, IdP солигдсоноор суларч болохгүй.
func TestSSORejectsUninvitedEmail(t *testing.T) {
	sso := &fakeSSO{configured: true, info: oidc.UserInfo{
		Sub: "sub-1", Email: "stranger@example.com", EmailVerified: true,
	}}
	uc := ssoUsecase(t, sso, &invitesWith{email: "invited@example.com"}, &acctsWith{any: true})

	_, err := uc.SSO(context.Background(), onboarding.SSORequest{Code: "c"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type)
}

// Баталгаажаагүй и-мэйлээр урилгын allow-list-ыг тойрч болохгүй.
func TestSSORejectsUnverifiedEmail(t *testing.T) {
	sso := &fakeSSO{configured: true, info: oidc.UserInfo{
		Sub: "sub-1", Email: "invited@example.com", EmailVerified: false,
	}}
	uc := ssoUsecase(t, sso, &invitesWith{email: "invited@example.com"}, &acctsWith{any: true})

	_, err := uc.SSO(context.Background(), onboarding.SSORequest{Code: "c"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type)
}

// Аль хэдийн ашигласан урилгыг дахин хэрэглэж болохгүй.
func TestSSORejectsAcceptedInvite(t *testing.T) {
	sso := &fakeSSO{configured: true, info: oidc.UserInfo{
		Sub: "sub-1", Email: "invited@example.com", EmailVerified: true,
	}}
	uc := ssoUsecase(t, sso, &invitesWith{email: "invited@example.com", accepted: true}, &acctsWith{any: true})

	_, err := uc.SSO(context.Background(), onboarding.SSORequest{Code: "c"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type)
}

// SSO тохируулаагүй бол чимээгүй үргэлжлэхгүй.
func TestSSONotConfigured(t *testing.T) {
	uc := ssoUsecase(t, &fakeSSO{configured: false}, &invitesWith{email: "x@example.com"}, &acctsWith{any: true})
	_, err := uc.SSO(context.Background(), onboarding.SSORequest{Code: "c"})
	require.Error(t, err)
}

// ПЛАТФОРМЫН АНХНЫ АЖИЛЛУУЛАЛТ: super admin огт байхгүй бол урилгагүй ч
// эхний хүн бүртгэлээ эхлүүлж чадна (урилга өгөх хүн байхгүй тул).
func TestSSOBootstrapAllowsFirstWhenNoSuperAdmin(t *testing.T) {
	sso := &fakeSSO{configured: true, info: oidc.UserInfo{
		Sub: "sub-1", Email: "first@example.com", EmailVerified: true,
	}}
	uc := ssoUsecase(t, sso, &invitesWith{email: "nobody@example.com"}, &acctsWith{any: false})

	res, err := uc.SSO(context.Background(), onboarding.SSORequest{Code: "c"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.OnboardToken)
	assert.Equal(t, "first@example.com", res.Email)
}

// Super admin НЭГЭНТ байгаа бол bootstrap хаалга ХААГДАНА — урилга заавал.
func TestSSOBootstrapClosesOnceSuperAdminExists(t *testing.T) {
	sso := &fakeSSO{configured: true, info: oidc.UserInfo{
		Sub: "sub-1", Email: "second@example.com", EmailVerified: true,
	}}
	uc := ssoUsecase(t, sso, &invitesWith{email: "nobody@example.com"}, &acctsWith{any: true})

	_, err := uc.SSO(context.Background(), onboarding.SSORequest{Code: "c"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type)
}

// Bootstrap ч гэсэн и-мэйл баталгаажаагүй бол зөвшөөрөхгүй.
func TestSSOBootstrapStillRequiresVerifiedEmail(t *testing.T) {
	sso := &fakeSSO{configured: true, info: oidc.UserInfo{
		Sub: "sub-1", Email: "first@example.com", EmailVerified: false,
	}}
	uc := ssoUsecase(t, sso, &invitesWith{email: "nobody@example.com"}, &acctsWith{any: false})

	_, err := uc.SSO(context.Background(), onboarding.SSORequest{Code: "c"})
	require.Error(t, err)
}

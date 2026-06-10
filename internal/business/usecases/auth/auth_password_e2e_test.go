//go:build integration

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	"template/internal/business/usecases/auth"
	"template/internal/test/testenv"
)

func TestE2E_ChangePassword_OldPasswordRejected(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	user := register(t, fix, "change@example.com", "Old_Pwd_123!")

	require.NoError(t, fix.Auth.ChangePassword(ctx, auth.ChangePasswordRequest{
		UserID:          user.ID,
		CurrentPassword: "Old_Pwd_123!",
		NewPassword:     "New_Pwd_456!",
	}))

	_, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "change@example.com", Password: "Old_Pwd_123!"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeUnauthorized, domErr.Type)

	out, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "change@example.com", Password: "New_Pwd_456!"})
	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken)
}

func TestE2E_ChangePassword_WrongCurrentRejected(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	user := register(t, fix, "wrong@example.com", "Old_Pwd_123!")

	err := fix.Auth.ChangePassword(ctx, auth.ChangePasswordRequest{
		UserID:          user.ID,
		CurrentPassword: "totally-wrong",
		NewPassword:     "New_Pwd_456!",
	})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeUnauthorized, domErr.Type)
}

func TestE2E_ChangePassword_RevokesPreExistingRefreshTokens(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	user := register(t, fix, "revoke@example.com", "Old_Pwd_123!")

	loginOut, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "revoke@example.com", Password: "Old_Pwd_123!"})
	require.NoError(t, err)
	staleRefresh := loginOut.RefreshToken
	require.NotEmpty(t, staleRefresh)

	require.NoError(t, fix.Auth.ChangePassword(ctx, auth.ChangePasswordRequest{
		UserID:          user.ID,
		CurrentPassword: "Old_Pwd_123!",
		NewPassword:     "New_Pwd_456!",
	}))

	_, err = fix.Auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: staleRefresh})
	require.Error(t, err, "refresh token issued before password change must be revoked")
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeUnauthorized, domErr.Type)
}

func TestE2E_ForgotPassword_ResetUnlocksLogin(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	register(t, fix, "forgot@example.com", "Old_Pwd_123!")

	require.NoError(t, fix.Auth.ForgotPassword(ctx, auth.ForgotPasswordRequest{Email: "forgot@example.com"}))
	resetCode := fix.Verifier.LastCode(t, "forgot@example.com")
	require.NotEmpty(t, resetCode)

	require.NoError(t, fix.Auth.ResetPassword(ctx, auth.ResetPasswordRequest{Email: "forgot@example.com", Code: resetCode, NewPassword: "Reset_Pwd_789!"}))

	_, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "forgot@example.com", Password: "Old_Pwd_123!"})
	require.Error(t, err)

	out, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "forgot@example.com", Password: "Reset_Pwd_789!"})
	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken)
}

func TestE2E_ForgotPassword_UnknownEmailNoOp(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	require.NoError(t, fix.Auth.ForgotPassword(ctx, auth.ForgotPasswordRequest{Email: "nobody@example.com"}))
}

func TestE2E_ResetPassword_TokenIsSingleUse(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	register(t, fix, "su@example.com", "Old_Pwd_123!")

	require.NoError(t, fix.Auth.ForgotPassword(ctx, auth.ForgotPasswordRequest{Email: "su@example.com"}))
	resetCode := fix.Verifier.LastCode(t, "su@example.com")

	require.NoError(t, fix.Auth.ResetPassword(ctx, auth.ResetPasswordRequest{Email: "su@example.com", Code: resetCode, NewPassword: "First_Reset_123!"}))

	err := fix.Auth.ResetPassword(ctx, auth.ResetPasswordRequest{Email: "su@example.com", Code: resetCode, NewPassword: "Second_Reset_456!"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeUnauthorized, domErr.Type)
}

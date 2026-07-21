//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/test/testenv"
)

// register нь шинэ хэрэглэгчийг register + verify-OTP хаалгаар нэвтрүүлдэг тул
// тест бүр идэвхжүүлэх дарааллыг дахин хийхийн оронд шалгаж буй хувилбартаа
// төвлөрч чадна.
func register(t *testing.T, fix *testenv.AuthFixture, email, password string) domain.User {
	t.Helper()
	ctx := context.Background()

	resp, err := fix.Auth.Register(ctx, auth.RegisterRequest{User: &domain.User{
		Username: "user_" + email,
		Email:    email,
		Password: password,
		RoleID:   domain.RoleUser,
	}})
	require.NoError(t, err)

	require.NoError(t, fix.Auth.SendOTP(ctx, auth.SendOTPRequest{Email: email}))
	otp := fix.Verifier.LastCode(t, email)
	require.NoError(t, fix.Auth.VerifyOTP(ctx, auth.VerifyOTPRequest{Email: email, OTPCode: otp}))

	return resp.User
}

func TestE2E_HappyPath_RegisterVerifyLogin(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	register(t, fix, "happy@example.com", "Secret_123!")

	out, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "happy@example.com", Password: "Secret_123!"})
	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken, "Login must return an access token")
	assert.NotEmpty(t, out.RefreshToken, "Login must return a refresh token")
}

func TestE2E_OTPBruteForceLockout(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	_, err := fix.Auth.Register(ctx, auth.RegisterRequest{User: &domain.User{
		Username: "lockout",
		Email:    "lock@example.com",
		Password: "Secret_123!",
		RoleID:   domain.RoleUser,
	}})
	require.NoError(t, err)
	require.NoError(t, fix.Auth.SendOTP(ctx, auth.SendOTPRequest{Email: "lock@example.com"}))

	for i := 0; i < 5; i++ {
		err := fix.Auth.VerifyOTP(ctx, auth.VerifyOTPRequest{Email: "lock@example.com", OTPCode: "000000"})
		require.Error(t, err, "attempt %d", i+1)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type, "attempt %d: %v", i+1, domErr)
	}

	correctOTP := fix.Verifier.LastCode(t, "lock@example.com")
	err = fix.Auth.VerifyOTP(ctx, auth.VerifyOTPRequest{Email: "lock@example.com", OTPCode: correctOTP})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type,
		"after MaxAttempts the correct code must also be rejected with Forbidden, got %v", domErr.Type)
}

func TestE2E_LoginRejectsInactiveUser(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	_, err := fix.Auth.Register(ctx, auth.RegisterRequest{User: &domain.User{
		Username: "inactive",
		Email:    "inactive@example.com",
		Password: "Secret_123!",
		RoleID:   domain.RoleUser,
	}})
	require.NoError(t, err)

	_, err = fix.Auth.Login(ctx, auth.LoginRequest{Email: "inactive@example.com", Password: "Secret_123!"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeForbidden, domErr.Type)
}

func TestE2E_RefreshRotatesAndRevokesOldToken(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	register(t, fix, "rotate@example.com", "Secret_123!")
	loggedIn, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "rotate@example.com", Password: "Secret_123!"})
	require.NoError(t, err)

	refreshed, err := fix.Auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: loggedIn.RefreshToken})
	require.NoError(t, err)
	assert.NotEqual(t, loggedIn.RefreshToken, refreshed.RefreshToken,
		"Refresh must rotate the token")

	_, err = fix.Auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: loggedIn.RefreshToken})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeUnauthorized, domErr.Type)

	_, err = fix.Auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: refreshed.RefreshToken})
	require.NoError(t, err)
}

func TestE2E_LogoutRevokesRefreshToken(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	register(t, fix, "logout@example.com", "Secret_123!")
	loggedIn, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "logout@example.com", Password: "Secret_123!"})
	require.NoError(t, err)

	require.NoError(t, fix.Auth.Logout(ctx, auth.LogoutRequest{RefreshToken: loggedIn.RefreshToken}))

	_, err = fix.Auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: loggedIn.RefreshToken})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeUnauthorized, domErr.Type)
}

func TestE2E_VerifyOTPActivatesUserAndAllowsLogin(t *testing.T) {
	fix := testenv.NewAuthFixture(t)
	ctx := context.Background()

	_, err := fix.Auth.Register(ctx, auth.RegisterRequest{User: &domain.User{
		Username: "activate",
		Email:    "activate@example.com",
		Password: "Secret_123!",
		RoleID:   domain.RoleUser,
	}})
	require.NoError(t, err)

	_, err = fix.Auth.Login(ctx, auth.LoginRequest{Email: "activate@example.com", Password: "Secret_123!"})
	require.Error(t, err, "login must fail before OTP verification")

	require.NoError(t, fix.Auth.SendOTP(ctx, auth.SendOTPRequest{Email: "activate@example.com"}))
	otp := fix.Verifier.LastCode(t, "activate@example.com")
	require.NoError(t, fix.Auth.VerifyOTP(ctx, auth.VerifyOTPRequest{Email: "activate@example.com", OTPCode: otp}))

	out, err := fix.Auth.Login(ctx, auth.LoginRequest{Email: "activate@example.com", Password: "Secret_123!"})
	require.NoError(t, err)
	assert.NotEmpty(t, out.AccessToken)
}

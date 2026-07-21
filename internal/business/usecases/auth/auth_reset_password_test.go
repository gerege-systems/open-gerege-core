// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"

	"template/internal/apperror"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/pkg/verify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResetPassword(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		code        string
		newPassword string
		setup       func(f *fixture)
		wantErr     bool
		wantErrType apperror.ErrorType
	}{
		{
			name:        "happy path verifies code, updates password, clears request",
			email:       "patrick@example.com",
			code:        "123456",
			newPassword: "Newpwd_999!",
			setup: func(f *fixture) {
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: activeUser(t)}, nil).Once()
				f.redis.On("Get", mock.Anything, "pwd_reset_req:patrick@example.com").Return("gcv_reset", nil).Once()
				f.verifier.On("Check", mock.Anything, "gcv_reset", "123456").Return(nil).Once()
				f.users.On("UpdatePassword", mock.Anything, mock.MatchedBy(func(req users.UpdatePasswordRequest) bool {
					u := req.User
					return u.ID == "user-1" && u.PasswordChangedAt != nil
				})).Return(nil).Once()
				f.redis.On("Del", mock.Anything, "pwd_reset_req:patrick@example.com").Return(nil).Once()
				f.redis.On("Set", mock.Anything, "pwd_cutoff:user-1", mock.AnythingOfType("string")).Return(nil).Once()
				f.redis.On("Expire", mock.Anything, "pwd_cutoff:user-1", mock.AnythingOfType("time.Duration")).Return(nil).Once()
			},
		},
		{
			name:        "missing code returns BadRequest",
			email:       "patrick@example.com",
			code:        "",
			newPassword: "Newpwd_999!",
			setup:       func(f *fixture) {},
			wantErr:     true,
			wantErrType: apperror.ErrTypeBadRequest,
		},
		{
			name:        "empty new password returns BadRequest",
			email:       "patrick@example.com",
			code:        "123456",
			newPassword: "",
			setup:       func(f *fixture) {},
			wantErr:     true,
			wantErrType: apperror.ErrTypeBadRequest,
		},
		{
			name:        "reset request expired or missing surfaces as Unauthorized",
			email:       "patrick@example.com",
			code:        "123456",
			newPassword: "Newpwd_999!",
			setup: func(f *fixture) {
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: activeUser(t)}, nil).Once()
				f.redis.On("Get", mock.Anything, "pwd_reset_req:patrick@example.com").Return("", errors.New("redis: nil")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeUnauthorized,
		},
		{
			name:        "invalid code surfaces as Unauthorized",
			email:       "patrick@example.com",
			code:        "999999",
			newPassword: "Newpwd_999!",
			setup: func(f *fixture) {
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: activeUser(t)}, nil).Once()
				f.redis.On("Get", mock.Anything, "pwd_reset_req:patrick@example.com").Return("gcv_reset", nil).Once()
				f.verifier.On("Check", mock.Anything, "gcv_reset", "999999").Return(verify.ErrNotApproved).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)
			err := f.usecase.ResetPassword(context.Background(), auth.ResetPasswordRequest{Email: tt.email, Code: tt.code, NewPassword: tt.newPassword})
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var domErr *apperror.DomainError
			require.True(t, errors.As(err, &domErr))
			assert.Equal(t, tt.wantErrType, domErr.Type)
		})
	}
}

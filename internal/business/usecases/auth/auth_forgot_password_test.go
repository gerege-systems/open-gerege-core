// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
)

func TestForgotPassword(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		setup       func(f *fixture)
		wantErr     bool
		wantErrType apperror.ErrorType
	}{
		{
			name:  "happy path increments rate counter, sends OTP via verify, stores request_id",
			email: "patrick@example.com",
			setup: func(f *fixture) {
				f.redis.On("Incr", mock.Anything, "forgot_attempts:patrick@example.com").Return(int64(1), nil).Once()
				f.redis.On("Expire", mock.Anything, "forgot_attempts:patrick@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: activeUser(t)}, nil).Once()
				f.verifier.On("Send", mock.Anything, "patrick@example.com", "").Return("gcv_reset", nil).Once()
				f.redis.On("Set", mock.Anything, "pwd_reset_req:patrick@example.com", "gcv_reset").Return(nil).Once()
				f.redis.On("Expire", mock.Anything, "pwd_reset_req:patrick@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
			},
		},
		{
			// Тодорхойгүй email-д код илгээхгүй, ижил nil буцаана (enumeration таслах).
			name:  "unknown email increments counter but is swallowed silently",
			email: "ghost@example.com",
			setup: func(f *fixture) {
				f.redis.On("Incr", mock.Anything, "forgot_attempts:ghost@example.com").Return(int64(1), nil).Once()
				f.redis.On("Expire", mock.Anything, "forgot_attempts:ghost@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "ghost@example.com"}).
					Return(users.GetByEmailResponse{}, apperror.NotFound("email not found")).Once()
			},
		},
		{
			name:  "rate limit exceeded surfaces as Forbidden, no GetByEmail call",
			email: "victim@example.com",
			setup: func(f *fixture) {
				// Fixture нь ForgotMaxAttempts=3-аар хязгаарладаг; 4 дэх хүсэлт үүнийг өдөөдөг.
				f.redis.On("Incr", mock.Anything, "forgot_attempts:victim@example.com").Return(int64(4), nil).Once()
				// attempts != 1 тул incrWithExpiry нь TTL байгаа эсэхийг
				// PTTL-ээр шалгана; эерэг утга буцаавал дахин Expire хийхгүй.
				f.redis.On("PTTL", mock.Anything, "forgot_attempts:victim@example.com").Return(15*time.Minute, nil).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeForbidden,
		},
		{
			name:  "infra error from users.GetByEmail bubbles up",
			email: "patrick@example.com",
			setup: func(f *fixture) {
				f.redis.On("Incr", mock.Anything, "forgot_attempts:patrick@example.com").Return(int64(1), nil).Once()
				f.redis.On("Expire", mock.Anything, "forgot_attempts:patrick@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).
					Return(users.GetByEmailResponse{}, apperror.InternalCause(errors.New("redis down"))).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)
			err := f.usecase.ForgotPassword(context.Background(), auth.ForgotPasswordRequest{Email: tt.email})
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tt.wantErrType != 0 {
				var domErr *apperror.DomainError
				require.True(t, errors.As(err, &domErr))
				assert.Equal(t, tt.wantErrType, domErr.Type)
			}
		})
	}
}

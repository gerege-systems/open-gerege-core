// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
)

func TestSendOTP(t *testing.T) {
	tests := []struct {
		name  string
		email string
		setup func(f *fixture)
		// wantErr / wantErrType-ийг хослуулсан, учир нь apperror.ErrTypeInternal
		// нь iota-гийн тэг — ганц sentinel нь тэр төрөлтэй мөргөлдөж, чимээгүйхэн
		// тэнцэх байсан.
		wantErr     bool
		wantErrType apperror.ErrorType
	}{
		{
			name:  "happy path sends via verify API, stores request_id, resets attempt counter",
			email: "patrick@example.com",
			setup: func(f *fixture) {
				user := activeUser(t)
				user.Active = false // SendOTP нь зөвхөн идэвхгүй бүртгэлд хүчинтэй
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: user}, nil).Once()
				// Verify API кодыг илгээж request_id буцаана; түүнийг Redis-д
				// OTPTTL-тэйгээр хадгалж, оролдлогын тоологчийг тэглэнэ.
				f.verifier.On("Send", mock.Anything, "patrick@example.com", "").Return("gcv_test123", nil).Once()
				f.redis.On("Set", mock.Anything, "user_otp:patrick@example.com", "gcv_test123").Return(nil).Once()
				f.redis.On("Expire", mock.Anything, "user_otp:patrick@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
				f.redis.On("Del", mock.Anything, "otp_attempts:patrick@example.com").Return(nil).Once()
			},
		},
		{
			name:  "already-active account short-circuits with BadRequest (no mailer / redis)",
			email: "patrick@example.com",
			setup: func(f *fixture) {
				// Идэвхтэй хэрэглэгч — эрт буцалт; mailer / redis дуудлага байхгүй.
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: activeUser(t)}, nil).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeBadRequest,
		},
		{
			name:  "unknown email surfaces as NotFound from users.GetByEmail",
			email: "ghost@example.com",
			setup: func(f *fixture) {
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "ghost@example.com"}).
					Return(users.GetByEmailResponse{}, apperror.NotFound("email not found")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			err := f.usecase.SendOTP(context.Background(), auth.SendOTPRequest{Email: tt.email})

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

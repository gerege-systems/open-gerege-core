// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"template/internal/apperror"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/pkg/verify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestVerifyOTP(t *testing.T) {
	tests := []struct {
		name  string
		email string
		code  string
		setup func(f *fixture)
		// wantErr / wantErrType-ийг хослуулсан, учир нь apperror.ErrTypeInternal
		// нь iota-гийн тэг — ганц sentinel нь тэр төрөлтэй мөргөлдөж, чимээгүйхэн
		// тэнцэх байсан.
		wantErr     bool
		wantErrType apperror.ErrorType
	}{
		{
			name:  "happy path activates user and clears OTP + attempt keys",
			email: "patrick@example.com",
			code:  "123456",
			setup: func(f *fixture) {
				user := activeUser(t)
				user.Active = false
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: user}, nil).Once()
				f.redis.On("Incr", mock.Anything, "otp_attempts:patrick@example.com").Return(int64(1), nil).Once()
				f.redis.On("Expire", mock.Anything, "otp_attempts:patrick@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
				f.redis.On("Get", mock.Anything, "user_otp:patrick@example.com").Return("gcv_test", nil).Once()
				f.verifier.On("Check", mock.Anything, "gcv_test", "123456").Return(nil).Once()
				f.users.On("Activate", mock.Anything, users.ActivateRequest{UserID: user.ID}).Return(nil).Once()
				f.redis.On("Del", mock.Anything, "user_otp:patrick@example.com").Return(nil).Once()
				f.redis.On("Del", mock.Anything, "otp_attempts:patrick@example.com").Return(nil).Once()
			},
		},
		{
			name:  "wrong code returns BadRequest after counting the attempt",
			email: "patrick@example.com",
			code:  "999999",
			setup: func(f *fixture) {
				user := activeUser(t)
				user.Active = false
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: user}, nil).Once()
				f.redis.On("Incr", mock.Anything, "otp_attempts:patrick@example.com").Return(int64(1), nil).Once()
				f.redis.On("Expire", mock.Anything, "otp_attempts:patrick@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
				f.redis.On("Get", mock.Anything, "user_otp:patrick@example.com").Return("gcv_test", nil).Once()
				f.verifier.On("Check", mock.Anything, "gcv_test", "999999").Return(verify.ErrNotApproved).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeBadRequest,
		},
		{
			name:  "verify API rejects code returns BadRequest",
			email: "patrick@example.com",
			code:  "12",
			setup: func(f *fixture) {
				user := activeUser(t)
				user.Active = false
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: user}, nil).Once()
				f.redis.On("Incr", mock.Anything, "otp_attempts:patrick@example.com").Return(int64(1), nil).Once()
				f.redis.On("Expire", mock.Anything, "otp_attempts:patrick@example.com", mock.AnythingOfType("time.Duration")).Return(nil).Once()
				f.redis.On("Get", mock.Anything, "user_otp:patrick@example.com").Return("gcv_test", nil).Once()
				f.verifier.On("Check", mock.Anything, "gcv_test", "12").Return(verify.ErrNotApproved).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeBadRequest,
		},
		{
			name:  "lockout after exceeding OTPMaxAttempts surfaces as Forbidden",
			email: "patrick@example.com",
			code:  "123456",
			setup: func(f *fixture) {
				user := activeUser(t)
				user.Active = false
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: user}, nil).Once()
				// 6 дахь оролдлого OTPMaxAttempts=5-аас давна; Incr нь 6
				// буцаана. Forbidden нь BadRequest "буруу код"-оос ялгаатай
				// дохио тул rate-limit / сэрэмжлүүлэг нь brute-force загварыг
				// бичгийн алдаанаас ялгаж чадна.
				f.redis.On("Incr", mock.Anything, "otp_attempts:patrick@example.com").Return(int64(6), nil).Once()
				// attempts != 1 тул incrWithExpiry нь TTL байгаа эсэхийг
				// PTTL-ээр шалгана; эерэг утга буцаавал дахин Expire хийхгүй.
				f.redis.On("PTTL", mock.Anything, "otp_attempts:patrick@example.com").Return(5*time.Minute, nil).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeForbidden,
		},
		{
			name:  "already-active account short-circuits with BadRequest",
			email: "patrick@example.com",
			code:  "123456",
			setup: func(f *fixture) {
				// Аль хэдийн идэвхтэй — эрт буцалт, Incr / Get / Activate байхгүй.
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: "patrick@example.com"}).Return(users.GetByEmailResponse{User: activeUser(t)}, nil).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			err := f.usecase.VerifyOTP(context.Background(), auth.VerifyOTPRequest{Email: tt.email, OTPCode: tt.code})

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

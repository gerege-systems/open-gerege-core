// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"

	golangJWT "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/pkg/jwt"
)

func refreshClaims(jti, email string) jwt.JwtCustomClaim {
	return jwt.JwtCustomClaim{
		UserID:           "user-1",
		Email:            email,
		Kind:             jwt.KindRefresh,
		RegisteredClaims: golangJWT.RegisteredClaims{ID: jti},
	}
}

func TestRefresh(t *testing.T) {
	tests := []struct {
		name  string
		token string
		setup func(f *fixture)
		// wantErr / wantErrType-ийг хослуулсан, учир нь apperror.ErrTypeInternal
		// нь iota-гийн тэг — ганц sentinel нь тэр төрөлтэй мөргөлдөж, чимээгүйхэн
		// тэнцэх байсан.
		wantErr     bool
		wantErrType apperror.ErrorType
	}{
		{
			name:  "happy path mints new pair and atomically consumes the old JTI",
			token: "old-refresh-tok",
			setup: func(f *fixture) {
				user := activeUser(t)
				oldJTI := "old-jti"
				f.jwt.On("ParseRefreshToken", "old-refresh-tok").Return(refreshClaims(oldJTI, user.Email), nil).Once()
				// Хуучин jti-г эхэнд нь GetDel-ээр атомаар уншиж-устгана
				// (single-use); хоосон бус утга → токен амьд байсан.
				f.redis.On("GetDel", mock.Anything, "refresh:"+oldJTI).Return(oldJTI, nil).Once()
				f.users.On("GetByEmail", mock.Anything, users.GetByEmailRequest{Email: user.Email}).Return(users.GetByEmailResponse{User: user}, nil).Once()
				f.jwt.On("GenerateTokenPair", user.ID, false, user.RoleID, user.Email).Return(samplePair(), nil).Once()
				f.redis.On("Set", mock.Anything, "refresh:refresh-jti", "refresh-jti").Return(nil).Once()
				f.redis.On("Expire", mock.Anything, "refresh:refresh-jti", mock.AnythingOfType("time.Duration")).Return(nil).Once()
			},
		},
		{
			name:  "revoked token (Redis miss on JTI) returns Unauthorized",
			token: "stale-tok",
			setup: func(f *fixture) {
				jti := "stale-jti"
				f.jwt.On("ParseRefreshToken", "stale-tok").Return(refreshClaims(jti, "x@y.com"), nil).Once()
				// GetDel алдаа буцаана → токен хүчингүй болсон / аль хэдийн ашигласан.
				f.redis.On("GetDel", mock.Anything, "refresh:"+jti).Return("", errors.New("redis: nil")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeUnauthorized,
		},
		{
			name:  "invalid signature surfaces as Unauthorized (no Redis call)",
			token: "bogus",
			setup: func(f *fixture) {
				f.jwt.On("ParseRefreshToken", "bogus").Return(jwt.JwtCustomClaim{}, errors.New("bad signature")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			out, err := f.usecase.Refresh(context.Background(), auth.RefreshRequest{RefreshToken: tt.token})

			if !tt.wantErr {
				require.NoError(t, err)
				assert.Equal(t, "access-tok", out.AccessToken)
				assert.Equal(t, "refresh-tok", out.RefreshToken)
				return
			}
			require.Error(t, err)
			var domErr *apperror.DomainError
			require.True(t, errors.As(err, &domErr))
			assert.Equal(t, tt.wantErrType, domErr.Type)
		})
	}
}

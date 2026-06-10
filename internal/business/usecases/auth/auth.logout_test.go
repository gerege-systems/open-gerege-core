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
	"template/pkg/jwt"
)

func TestLogout(t *testing.T) {
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
			name:  "deletes refresh JTI when token parses cleanly",
			token: "good-tok",
			setup: func(f *fixture) {
				claims := jwt.JwtCustomClaim{
					Kind:             jwt.KindRefresh,
					RegisteredClaims: golangJWT.RegisteredClaims{ID: "jti-to-delete"},
				}
				f.jwt.On("ParseRefreshToken", "good-tok").Return(claims, nil).Once()
				f.redis.On("Del", mock.Anything, "refresh:jti-to-delete").Return(nil).Once()
			},
		},
		{
			name:  "invalid token returns Unauthorized (no Redis call)",
			token: "bad",
			setup: func(f *fixture) {
				f.jwt.On("ParseRefreshToken", "bad").Return(jwt.JwtCustomClaim{}, errors.New("bad sig")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			err := f.usecase.Logout(context.Background(), auth.LogoutRequest{RefreshToken: tt.token})

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

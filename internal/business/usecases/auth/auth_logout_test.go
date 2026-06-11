// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
		name        string
		token       string
		accessToken string
		setup       func(f *fixture)
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
		{
			name:        "valid access token lands on the deny-list with remaining TTL",
			token:       "good-tok",
			accessToken: "acc-tok",
			setup: func(f *fixture) {
				refreshClaims := jwt.JwtCustomClaim{
					Kind:             jwt.KindRefresh,
					RegisteredClaims: golangJWT.RegisteredClaims{ID: "jti-to-delete"},
				}
				f.jwt.On("ParseRefreshToken", "good-tok").Return(refreshClaims, nil).Once()
				f.redis.On("Del", mock.Anything, "refresh:jti-to-delete").Return(nil).Once()

				accessClaims := jwt.JwtCustomClaim{
					Kind: jwt.KindAccess,
					RegisteredClaims: golangJWT.RegisteredClaims{
						ID:        "acc-jti",
						ExpiresAt: golangJWT.NewNumericDate(time.Now().Add(time.Hour)),
					},
				}
				f.jwt.On("ParseToken", "acc-tok").Return(accessClaims, nil).Once()
				f.redis.On("Set", mock.Anything, "access_deny:acc-jti", "1").Return(nil).Once()
				f.redis.On("Expire", mock.Anything, "access_deny:acc-jti", mock.Anything).Return(nil).Once()
			},
		},
		{
			name:        "unparseable access token does not fail logout",
			token:       "good-tok",
			accessToken: "garbage",
			setup: func(f *fixture) {
				refreshClaims := jwt.JwtCustomClaim{
					Kind:             jwt.KindRefresh,
					RegisteredClaims: golangJWT.RegisteredClaims{ID: "jti-to-delete"},
				}
				f.jwt.On("ParseRefreshToken", "good-tok").Return(refreshClaims, nil).Once()
				f.redis.On("Del", mock.Anything, "refresh:jti-to-delete").Return(nil).Once()
				f.jwt.On("ParseToken", "garbage").Return(jwt.JwtCustomClaim{}, errors.New("bad sig")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			err := f.usecase.Logout(context.Background(), auth.LogoutRequest{RefreshToken: tt.token, AccessToken: tt.accessToken})

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

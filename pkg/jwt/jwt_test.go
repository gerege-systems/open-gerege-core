// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package jwt_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"template/pkg/clock"
	"template/pkg/jwt"
)

const (
	testSecret  = "test-secret-key"
	testIssuer  = "test-issuer"
	testExpired = 5
)

func TestGenerateToken(t *testing.T) {
	jwtService := jwt.NewJWTService(testSecret, testIssuer, testExpired)
	token, err := jwtService.GenerateToken("asf-asf-asfdasd-asdfsa", false, 2, "john.doe@example.com")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestParseToken(t *testing.T) {
	t.Run("With Valid Token", func(t *testing.T) {
		jwtService := jwt.NewJWTService(testSecret, testIssuer, testExpired)

		token, _ := jwtService.GenerateToken("asf-asf-asfdasd-asdfsa", false, 2, "john.doe@example.com")

		claims, err := jwtService.ParseToken(token)
		assert.NoError(t, err)
		assert.Equal(t, "asf-asf-asfdasd-asdfsa", claims.UserID)
		assert.Equal(t, false, claims.IsAdmin)
		assert.Equal(t, "john.doe@example.com", claims.Email)
		assert.True(t, claims.ExpiresAt.After(time.Now()) || claims.ExpiresAt.Equal(time.Now()))
		assert.Equal(t, testIssuer, claims.Issuer)
		assert.True(t, claims.IssuedAt.Before(time.Now()) || claims.IssuedAt.Equal(time.Now()))
	})
	t.Run("With Invalid Token", func(t *testing.T) {
		jwtService := jwt.NewJWTService(testSecret, testIssuer, testExpired)

		_, err := jwtService.ParseToken("invalid_token")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, jwt.ErrInvalidToken), "expected error to wrap ErrInvalidToken, got %v", err)
	})
}

func TestGenerateTokenPair_RespectsInjectedClock(t *testing.T) {
	// Цагийг ойрын ирээдүйн мэдэгдэж буй агшинд царцаана — ингэснээр шалгаж буй
	// ExpiresAt болон IssuedAt утгууд яг таг байх ба олгогдсон токенуудыг
	// ParseToken (нь exp-г бодит time.Now()-тэй харьцуулдаг) одоо ч хүчинтэйд
	// тооцно. Энд хатуу кодлогдсон огноо ашиглавал тэр огноо өнгөрөнгүүт чимээгүй
	// хуучирна; time.Now()-аас гарган авах нь нэг ажиллагааны дотор тодорхой
	// (deterministic) байдлаа алдалгүйгээр тестийг үүрд шинэлэг байлгана.
	at := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	svc := jwt.WithClock(
		jwt.NewJWTServiceWithRefresh(testSecret, testIssuer, 5, 7),
		clock.Frozen(at),
	)

	pair, err := svc.GenerateTokenPair("user-1", false, 2, "alice@example.com")
	require.NoError(t, err)

	// Access токен: IssuedAt = at, ExpiresAt = at + 5h.
	accessClaims, err := svc.ParseToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, at, accessClaims.IssuedAt.UTC())
	assert.Equal(t, at.Add(5*time.Hour), accessClaims.ExpiresAt.UTC())

	// Refresh токен: IssuedAt = at, ExpiresAt = at + 7d.
	refreshClaims, err := svc.ParseRefreshToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, at, refreshClaims.IssuedAt.UTC())
	assert.Equal(t, at.Add(7*24*time.Hour), refreshClaims.ExpiresAt.UTC())
}

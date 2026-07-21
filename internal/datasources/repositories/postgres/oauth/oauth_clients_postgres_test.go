//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// oauth_clients repository-ийн integration тест (жинхэнэ Postgres). Энэ хүснэгт
// нь системийн тохиргоо тул RLS-гүй — тестийн гол зорилго нь SQL, text[] багана
// болон алдааны хөрвүүлэлт (NotFound / Conflict) зөв эсэхийг батлах.
package oauth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	oauthpg "template/internal/datasources/repositories/postgres/oauth"
	"template/internal/test/testenv"
)

func seedClient() domain.OAuthClient {
	return domain.OAuthClient{
		ClientID:                "ring-dgov-mn",
		ClientName:              "ring.dgov.mn",
		SecretHash:              "$pbkdf2-sha256$i=25000,l=32$Xk3NjhYzw2vo0iHb0dENsw$85qvQUf5V71AmArvdGdczye399QcGfByVrEhTAIX4XU",
		TokenEndpointAuthMethod: domain.AuthMethodBasic,
		AppType:                 "web",
		GrantTypes:              []string{domain.GrantAuthorizationCode, domain.GrantRefreshToken},
		ResponseTypes:           []string{"code"},
		Scopes:                  []string{"openid", "profile", "email"},
		RedirectURIs:            []string{"https://ring.dgov.mn/sso/callback"},
		PostLogoutRedirectURIs:  []string{"https://ring.dgov.mn/"},
		Tags:                    []string{"rp"},
		Enabled:                 true,
		CreatedBy:               "test",
	}
}

// assertErrType нь repo-ийн буцаасан DomainError-ийн төрлийг шалгана
// (rbac_postgres_test.go-ийн хэв маяг).
func assertErrType(t *testing.T, err error, want apperror.ErrorType) {
	t.Helper()
	var d *apperror.DomainError
	if assert.ErrorAs(t, err, &d) {
		assert.Equal(t, want, d.Type)
	}
}

func TestOAuthClientRepository(t *testing.T) {
	admin := testenv.StartPostgres(t)
	repo := oauthpg.NewClientRepository(admin)
	ctx := context.Background()

	t.Run("create returns the stored row", func(t *testing.T) {
		got, err := repo.Create(ctx, seedClient())
		require.NoError(t, err)
		assert.Equal(t, "ring-dgov-mn", got.ClientID)
		assert.Equal(t, []string{"openid", "profile", "email"}, got.Scopes)
		assert.Equal(t, []string{"https://ring.dgov.mn/sso/callback"}, got.RedirectURIs)
		assert.True(t, got.Enabled)
		assert.False(t, got.CreatedAt.IsZero())
	})

	t.Run("duplicate client_id conflicts", func(t *testing.T) {
		_, err := repo.Create(ctx, seedClient())
		require.Error(t, err)
		assertErrType(t, err, apperror.ErrTypeConflict)
	})

	t.Run("get round-trips every field", func(t *testing.T) {
		got, err := repo.Get(ctx, "ring-dgov-mn")
		require.NoError(t, err)
		want := seedClient()
		assert.Equal(t, want.SecretHash, got.SecretHash)
		assert.Equal(t, want.GrantTypes, got.GrantTypes)
		assert.Equal(t, want.PostLogoutRedirectURIs, got.PostLogoutRedirectURIs)
		assert.Equal(t, want.Tags, got.Tags)
		assert.Equal(t, want.AppType, got.AppType)
	})

	t.Run("get unknown client is NotFound", func(t *testing.T) {
		_, err := repo.Get(ctx, "no-such-client")
		require.Error(t, err)
		assertErrType(t, err, apperror.ErrTypeNotFound)
	})

	t.Run("update leaves the secret hash untouched", func(t *testing.T) {
		c := seedClient()
		c.ClientName = "ring.dgov.mn (renamed)"
		c.SecretHash = "" // Update нь secret-д хүрэхгүй байх ёстой
		c.Scopes = []string{"openid"}
		c.Enabled = false

		got, err := repo.Update(ctx, c)
		require.NoError(t, err)
		assert.Equal(t, "ring.dgov.mn (renamed)", got.ClientName)
		assert.Equal(t, []string{"openid"}, got.Scopes)
		assert.False(t, got.Enabled)
		assert.Equal(t, seedClient().SecretHash, got.SecretHash,
			"Update must never clear the secret — only SetSecretHash may change it")
		require.NotNil(t, got.UpdatedAt)
	})

	t.Run("update unknown client is NotFound", func(t *testing.T) {
		c := seedClient()
		c.ClientID = "no-such-client"
		_, err := repo.Update(ctx, c)
		require.Error(t, err)
		assertErrType(t, err, apperror.ErrTypeNotFound)
	})

	t.Run("set secret hash", func(t *testing.T) {
		require.NoError(t, repo.SetSecretHash(ctx, "ring-dgov-mn", "$argon2id$new"))
		got, err := repo.Get(ctx, "ring-dgov-mn")
		require.NoError(t, err)
		assert.Equal(t, "$argon2id$new", got.SecretHash)

		err = repo.SetSecretHash(ctx, "no-such-client", "$argon2id$x")
		require.Error(t, err)
		assertErrType(t, err, apperror.ErrTypeNotFound)
	})

	t.Run("list returns newest first", func(t *testing.T) {
		second := seedClient()
		second.ClientID = "template-dgov-mn"
		second.RedirectURIs = []string{"https://template.dgov.mn/auth/callback"}
		_, err := repo.Create(ctx, second)
		require.NoError(t, err)

		list, err := repo.List(ctx)
		require.NoError(t, err)
		require.Len(t, list, 2)
		assert.Equal(t, "template-dgov-mn", list[0].ClientID, "newest client should sort first")
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, "template-dgov-mn"))

		_, err := repo.Get(ctx, "template-dgov-mn")
		assertErrType(t, err, apperror.ErrTypeNotFound)

		err = repo.Delete(ctx, "template-dgov-mn")
		require.Error(t, err)
		assertErrType(t, err, apperror.ErrTypeNotFound)
	})

	t.Run("empty slices are stored as empty arrays not null", func(t *testing.T) {
		c := seedClient()
		c.ClientID = "m2m-app"
		c.AppType = "m2m"
		c.GrantTypes = []string{domain.GrantClientCredentials}
		c.ResponseTypes = nil
		c.RedirectURIs = nil
		c.PostLogoutRedirectURIs = nil
		c.Tags = nil

		got, err := repo.Create(ctx, c)
		require.NoError(t, err)
		assert.Empty(t, got.RedirectURIs)
		assert.Empty(t, got.Tags)
		assert.NotNil(t, got.RedirectURIs, "NOT NULL column must come back as an empty slice")
	})
}

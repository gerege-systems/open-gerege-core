//go:build integration

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	postgresrepo "template/internal/datasources/repositories/postgres/users"
	"template/internal/datasources/rls"
	"template/internal/test/testenv"
)

// TestRLS_UsersEnforcement нь Postgres Row-Level Security бодлогууд non-superuser
// app_user холболтоор ҮНЭХЭЭР хэрэгждэгийг батална: service нь бүгдийг харна,
// энгийн хэрэглэгч зөвхөн өөрийн мөрийг, identity байхгүй бол юу ч (fail-closed),
// admin нь бүгдийг.
func TestRLS_UsersEnforcement(t *testing.T) {
	admin := testenv.StartPostgres(t)    // superuser pool — migration + RLS суулгана
	app := testenv.AppUserPool(t, admin) // non-superuser pool — RLS-д захирагдана
	repo := postgresrepo.NewUserRepository(app)

	svc := rls.WithService(context.Background())

	alice, err := repo.Store(svc, fixture("alice@example.com"))
	require.NoError(t, err, "service нь INSERT хийж чадах ёстой")
	bob, err := repo.Store(svc, fixture("bob@example.com"))
	require.NoError(t, err)

	t.Run("service sees all rows", func(t *testing.T) {
		list, err := repo.List(svc, repointerface.UserListFilter{}, 0, 100)
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})

	t.Run("no identity is fail-closed (sees nothing)", func(t *testing.T) {
		list, err := repo.List(context.Background(), repointerface.UserListFilter{}, 0, 100)
		require.NoError(t, err)
		assert.Empty(t, list, "identity байхгүй үед RLS бүх мөрийг хаах ёстой")
	})

	t.Run("user sees only own row", func(t *testing.T) {
		aliceCtx := rls.WithUser(context.Background(), alice.ID)

		got, err := repo.GetByID(aliceCtx, alice.ID)
		require.NoError(t, err)
		assert.Equal(t, alice.ID, got.ID)

		_, err = repo.GetByID(aliceCtx, bob.ID)
		require.Error(t, err, "alice нь bob-ийн мөрийг харах ёсгүй")
		var de *apperror.DomainError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, apperror.ErrTypeNotFound, de.Type)

		list, err := repo.List(aliceCtx, repointerface.UserListFilter{}, 0, 100)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, alice.ID, list[0].ID)
	})

	t.Run("user cannot update another user's row", func(t *testing.T) {
		aliceCtx := rls.WithUser(context.Background(), alice.ID)

		// bob-ийн мөрийг шинэчлэх оролдлого RLS-ээр хаагдаж, нөлөөлсөн мөр 0 →
		// NotFound (repository RowsAffected == 0-г NotFound болгодог).
		err := repo.UpdatePassword(aliceCtx, &domain.User{ID: bob.ID, Password: "$2a$10$hijacked"})
		require.Error(t, err)
		var de *apperror.DomainError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, apperror.ErrTypeNotFound, de.Type)

		// Өөрийн мөрийг шинэчлэх нь зөвшөөрөгдөнө.
		err = repo.UpdatePassword(aliceCtx, &domain.User{ID: alice.ID, Password: "$2a$10$ownrow"})
		require.NoError(t, err)
	})

	t.Run("admin sees all rows", func(t *testing.T) {
		adminCtx := rls.WithAdmin(context.Background(), "00000000-0000-0000-0000-000000000000")
		list, err := repo.List(adminCtx, repointerface.UserListFilter{}, 0, 100)
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
}

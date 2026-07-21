//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// RBAC repository-ийн integration тест (жинхэнэ Postgres): seed хийгдсэн
// system role-ууд, role CRUD, permission олголт/уншилт, system role устгах
// хамгаалалт (DELETE ... AND is_system=false → 0 мөр → NotFound), ашиглагдаж
// буй role-ийн тоо.
package rbac_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	rbacpg "template/internal/datasources/repositories/postgres/rbac"
	"template/internal/test/testenv"
)

func TestRBACRepository(t *testing.T) {
	pool := testenv.StartPostgres(t)
	repo := rbacpg.NewRBACRepository(pool)
	ctx := context.Background()

	t.Run("seeded system roles exist", func(t *testing.T) {
		roles, err := repo.ListRoles(ctx)
		require.NoError(t, err)
		byKey := map[string]domain.Role{}
		for _, r := range roles {
			byKey[r.Key] = r
		}
		for _, k := range []string{"admin", "user", "manager"} {
			r, ok := byKey[k]
			require.True(t, ok, "%s role seed байх ёстой", k)
			assert.True(t, r.IsSystem, "%s нь is_system байх ёстой", k)
		}
	})

	t.Run("seeded permission grants (user role)", func(t *testing.T) {
		perms, err := repo.GetRolePermissions(ctx, 2) // user
		require.NoError(t, err)
		assert.Contains(t, perms, "dashboard.view")
		assert.Contains(t, perms, "personal.view")
		assert.NotContains(t, perms, "users.manage")
	})

	t.Run("create → set perms → read → delete", func(t *testing.T) {
		role, err := repo.CreateRole(ctx, &domain.Role{Key: "auditor_it", Name: "IT Auditor"})
		require.NoError(t, err)
		require.NotZero(t, role.ID)
		assert.False(t, role.IsSystem)

		require.NoError(t, repo.SetRolePermissions(ctx, role.ID, []string{"dashboard.view", "manager.view"}))
		perms, err := repo.GetRolePermissions(ctx, role.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"dashboard.view", "manager.view"}, perms)

		// Дахин SetRolePermissions нь орлуулна (нэмдэггүй).
		require.NoError(t, repo.SetRolePermissions(ctx, role.ID, []string{"dashboard.view"}))
		perms, _ = repo.GetRolePermissions(ctx, role.ID)
		assert.ElementsMatch(t, []string{"dashboard.view"}, perms)

		require.NoError(t, repo.DeleteRole(ctx, role.ID))
		_, err = repo.GetRole(ctx, role.ID)
		require.Error(t, err, "устгасны дараа GetRole алдаа өгөх ёстой")
	})

	t.Run("system role cannot be deleted", func(t *testing.T) {
		// DELETE ... AND is_system=false → 0 мөр → NotFound.
		// DELETE ... AND is_system=false → 0 мөр → repo нь Conflict буцаана.
		err := repo.DeleteRole(ctx, 1) // admin (is_system)
		require.Error(t, err)
		var d *apperror.DomainError
		if assert.ErrorAs(t, err, &d) {
			assert.Equal(t, apperror.ErrTypeConflict, d.Type)
		}
		// Устгагдаагүй эсэхийг батал.
		_, err = repo.GetRole(ctx, 1)
		require.NoError(t, err)
	})

	t.Run("CountUsersWithRole for unused role is 0", func(t *testing.T) {
		role, err := repo.CreateRole(ctx, &domain.Role{Key: "temp_role", Name: "Temp"})
		require.NoError(t, err)
		n, err := repo.CountUsersWithRole(ctx, role.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})
}

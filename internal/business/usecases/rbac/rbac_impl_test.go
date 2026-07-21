// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// RBAC usecase-ийн unit тестүүд: Resolve (admin→бүх каталог, энгийн role→
// өөрийн олголт, кэш + бичих үеийн invalidate), role CRUD-ийн validation
// (slugify, хоосон нэр, ашиглагдаж буй role устгахгүй).
package rbac_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/rbac"
	"template/internal/test/mocks"
)

type fixture struct {
	usecase rbac.Usecase
	repo    *mocks.RBACRepository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := mocks.NewRBACRepository(t)
	return &fixture{usecase: rbac.NewUsecase(repo), repo: repo}
}

// catalogue нь migration 8-ын seed-тэй ижил эрхийн каталог.
var catalogue = []domain.Permission{
	{Key: "dashboard.view"}, {Key: "settings.manage"}, {Key: "users.manage"},
	{Key: "roles.manage"}, {Key: "manager.view"}, {Key: "personal.view"},
}

func TestResolve(t *testing.T) {
	t.Run("admin role resolves to the FULL catalogue", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetRole", mock.Anything, 1).
			Return(domain.Role{ID: 1, Key: "admin", IsSystem: true}, nil).Once()
		f.repo.On("ListPermissions", mock.Anything).Return(catalogue, nil).Once()

		keys, err := f.usecase.Resolve(context.Background(), 1)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{
			"dashboard.view", "settings.manage", "users.manage",
			"roles.manage", "manager.view", "personal.view",
		}, keys)
		assert.True(t, sort.StringsAreSorted(keys), "Resolve нь эрэмбэлэгдсэн байх ёстой")
	})

	t.Run("plain role resolves to its explicit grants only", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetRole", mock.Anything, 2).
			Return(domain.Role{ID: 2, Key: "user", IsSystem: true}, nil).Once()
		f.repo.On("GetRolePermissions", mock.Anything, 2).
			Return([]string{"personal.view", "dashboard.view"}, nil).Once()

		keys, err := f.usecase.Resolve(context.Background(), 2)
		require.NoError(t, err)
		assert.Equal(t, []string{"dashboard.view", "personal.view"}, keys)
	})

	t.Run("unknown role returns the repo error", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetRole", mock.Anything, 99).
			Return(domain.Role{}, apperror.NotFound("role not found")).Once()

		_, err := f.usecase.Resolve(context.Background(), 99)
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeNotFound, domErr.Type)
	})

	t.Run("second resolve within TTL is served from cache", func(t *testing.T) {
		f := newFixture(t)
		// .Once() — repo хоёр дахь удаа дуудагдвал mock унана.
		f.repo.On("GetRole", mock.Anything, 3).
			Return(domain.Role{ID: 3, Key: "manager", IsSystem: true}, nil).Once()
		f.repo.On("GetRolePermissions", mock.Anything, 3).
			Return([]string{"users.manage"}, nil).Once()

		for i := 0; i < 3; i++ {
			keys, err := f.usecase.Resolve(context.Background(), 3)
			require.NoError(t, err)
			assert.Equal(t, []string{"users.manage"}, keys)
		}
	})

	t.Run("SetRolePermissions invalidates the cache", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetRole", mock.Anything, 3).
			Return(domain.Role{ID: 3, Key: "manager", IsSystem: true}, nil).Times(3)
		f.repo.On("GetRolePermissions", mock.Anything, 3).
			Return([]string{"users.manage"}, nil).Once()
		f.repo.On("SetRolePermissions", mock.Anything, 3, []string{"users.manage", "manager.view"}).
			Return(nil).Once()
		f.repo.On("GetRolePermissions", mock.Anything, 3).
			Return([]string{"users.manage", "manager.view"}, nil).Once()

		_, err := f.usecase.Resolve(context.Background(), 3)
		require.NoError(t, err)
		require.NoError(t, f.usecase.SetRolePermissions(context.Background(), 3, []string{"users.manage", "manager.view"}))
		keys, err := f.usecase.Resolve(context.Background(), 3)
		require.NoError(t, err)
		assert.Equal(t, []string{"manager.view", "users.manage"}, keys)
	})
}

func TestCreateRole(t *testing.T) {
	t.Run("slugifies the key from the name", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("CreateRole", mock.Anything, mock.MatchedBy(func(r *domain.Role) bool {
			return r.Key == "sales_manager" && r.Name == "Sales Manager"
		})).Return(domain.Role{ID: 10, Key: "sales_manager", Name: "Sales Manager"}, nil).Once()

		role, err := f.usecase.CreateRole(context.Background(), rbac.CreateRoleRequest{Name: "Sales Manager"})
		require.NoError(t, err)
		assert.Equal(t, "sales_manager", role.Key)
	})

	t.Run("rejects empty key and name", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.usecase.CreateRole(context.Background(), rbac.CreateRoleRequest{Key: "  ", Name: "  "})
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
	})

	t.Run("assigns initial permissions when provided", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("CreateRole", mock.Anything, mock.Anything).
			Return(domain.Role{ID: 11, Key: "auditor", Name: "Auditor"}, nil).Once()
		f.repo.On("SetRolePermissions", mock.Anything, 11, []string{"dashboard.view"}).Return(nil).Once()

		_, err := f.usecase.CreateRole(context.Background(), rbac.CreateRoleRequest{
			Key: "auditor", Name: "Auditor", Permissions: []string{"dashboard.view"},
		})
		require.NoError(t, err)
	})
}

func TestUpdateRole(t *testing.T) {
	t.Run("rejects empty name", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.usecase.UpdateRole(context.Background(), rbac.UpdateRoleRequest{ID: 3, Name: " "})
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
	})
}

func TestDeleteRole(t *testing.T) {
	t.Run("refuses to delete a role assigned to users", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("CountUsersWithRole", mock.Anything, 3).Return(5, nil).Once()

		err := f.usecase.DeleteRole(context.Background(), 3)
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeConflict, domErr.Type)
	})

	t.Run("deletes an unused role", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("CountUsersWithRole", mock.Anything, 10).Return(0, nil).Once()
		f.repo.On("DeleteRole", mock.Anything, 10).Return(nil).Once()

		require.NoError(t, f.usecase.DeleteRole(context.Background(), 10))
	})

	t.Run("system role deletion surfaces the repo NotFound", func(t *testing.T) {
		// Repo давхарга `DELETE ... AND is_system = false`-ээр system role-ийг
		// хамгаалдаг — 0 мөр устгавал NotFound буцаана; usecase түүнийг
		// DomainError хэвээр нь дамжуулна.
		f := newFixture(t)
		f.repo.On("CountUsersWithRole", mock.Anything, 1).Return(0, nil).Once()
		f.repo.On("DeleteRole", mock.Anything, 1).
			Return(apperror.NotFound("role not found or is a system role")).Once()

		err := f.usecase.DeleteRole(context.Background(), 1)
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeNotFound, domErr.Type)
	})
}

func TestSetRolePermissions(t *testing.T) {
	t.Run("unknown role is rejected before writing", func(t *testing.T) {
		f := newFixture(t)
		f.repo.On("GetRole", mock.Anything, 99).
			Return(domain.Role{}, apperror.NotFound("role not found")).Once()

		err := f.usecase.SetRolePermissions(context.Background(), 99, []string{"dashboard.view"})
		require.Error(t, err)
		var domErr *apperror.DomainError
		require.True(t, errors.As(err, &domErr))
		assert.Equal(t, apperror.ErrTypeNotFound, domErr.Type)
	})
}

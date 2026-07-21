//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	postgresrepo "template/internal/datasources/repositories/postgres/users"
	"template/internal/test/testenv"
)

// fixture нь тестүүдэд зориулж боломжийн өгөгдмөл утгуудтай UserDomain-г
// бүтээдэг. Дуудагч зөвхөн өөрийн хувилбарт хамаатай зүйлсийг дарж бичнэ.
func fixture(email string) *domain.User {
	return &domain.User{
		Username:  "user_" + email,
		Email:     email,
		Password:  "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		RoleID:    domain.RoleUser,
		CreatedAt: time.Now().UTC(),
	}
}

func TestRepo_StoreAndGetByEmail(t *testing.T) {
	db := testenv.StartPostgres(t)
	repo := postgresrepo.NewUserRepository(db)
	ctx := context.Background()

	stored, err := repo.Store(ctx, fixture("alice@example.com"))
	require.NoError(t, err)
	assert.NotEmpty(t, stored.ID, "INSERT … RETURNING must populate the id")
	assert.Equal(t, "alice@example.com", stored.Email)
	assert.False(t, stored.Active, "new users start inactive")

	got, err := repo.GetByEmail(ctx, &domain.User{Email: "alice@example.com"})
	require.NoError(t, err)
	assert.Equal(t, stored.ID, got.ID)
}

func TestRepo_StoreDuplicateEmail_ReturnsConflict(t *testing.T) {
	db := testenv.StartPostgres(t)
	repo := postgresrepo.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.Store(ctx, fixture("dup@example.com"))
	require.NoError(t, err)

	// Ижил email, өөр username — email дээрх хэсэгчилсэн давтагдашгүй
	// индекс үүн дээр одоо ч ажиллах ёстой.
	second := fixture("dup@example.com")
	second.Username = "another"
	_, err = repo.Store(ctx, second)
	require.Error(t, err)

	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr), "expected typed *apperror.DomainError, got %T", err)
	assert.Equal(t, apperror.ErrTypeConflict, domErr.Type)
}

func TestRepo_GetByEmail_NotFound(t *testing.T) {
	db := testenv.StartPostgres(t)
	repo := postgresrepo.NewUserRepository(db)

	_, err := repo.GetByEmail(context.Background(), &domain.User{Email: "nobody@example.com"})
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeNotFound, domErr.Type)
}

func TestRepo_GetByID_RoundTrip(t *testing.T) {
	db := testenv.StartPostgres(t)
	repo := postgresrepo.NewUserRepository(db)
	ctx := context.Background()

	stored, err := repo.Store(ctx, fixture("byid@example.com"))
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, stored.ID)
	require.NoError(t, err)
	assert.Equal(t, stored.Email, got.Email)
}

func TestRepo_SoftDelete_HidesFromQueries(t *testing.T) {
	db := testenv.StartPostgres(t)
	repo := postgresrepo.NewUserRepository(db)
	ctx := context.Background()

	stored, err := repo.Store(ctx, fixture("gone@example.com"))
	require.NoError(t, err)

	require.NoError(t, repo.SoftDelete(ctx, stored.ID))

	// Өгөгдмөл query-үүд (GetByEmail / GetByID) нь deleted_at IS NULL
	// дээр шүүх ёстой — мөр нь хүснэгтэд байгаа боловч нэвтрэх / хайх
	// замуудад харагдахгүй байх ёстой.
	_, err = repo.GetByEmail(ctx, &domain.User{Email: "gone@example.com"})
	require.Error(t, err)
	_, err = repo.GetByID(ctx, stored.ID)
	require.Error(t, err)

	// Аль хэдийн устгагдсан мөрийг дахин устгах нь чимээгүй амжилттай
	// болохын оронд (энэ нь дээд урсгал дахь алдааг нуух болно) NotFound
	// мэдээлэх ёстой.
	err = repo.SoftDelete(ctx, stored.ID)
	var domErr *apperror.DomainError
	require.True(t, errors.As(err, &domErr))
	assert.Equal(t, apperror.ErrTypeNotFound, domErr.Type)
}

func TestRepo_SoftDelete_AllowsReregistration(t *testing.T) {
	db := testenv.StartPostgres(t)
	repo := postgresrepo.NewUserRepository(db)
	ctx := context.Background()

	stored, err := repo.Store(ctx, fixture("recycle@example.com"))
	require.NoError(t, err)
	require.NoError(t, repo.SoftDelete(ctx, stored.ID))

	// email дээрх хэсэгчилсэн давтагдашгүй индекс нь WHERE deleted_at IS
	// NULL тул soft delete-ийн дараа ижил email-г дахин ашиглах боломжтой
	// байх ёстой.
	_, err = repo.Store(ctx, fixture("recycle@example.com"))
	require.NoError(t, err, "email should be reusable after soft delete")
}

func TestRepo_List_FiltersAndPagination(t *testing.T) {
	db := testenv.StartPostgres(t)
	repo := postgresrepo.NewUserRepository(db)
	ctx := context.Background()

	// Role болон идэвхтэй төлөвүүдийн холимог seed хий.
	for i, email := range []string{"a@x.com", "b@x.com", "c@x.com"} {
		u := fixture(email)
		if i == 0 {
			u.RoleID = domain.RoleAdmin // админ
		}
		_, err := repo.Store(ctx, u)
		require.NoError(t, err)
	}
	// ActiveOnly шүүлтэд тааруулах зүйл байхын тулд нэг хэрэглэгчийг
	// идэвхжүүл.
	all, err := repo.List(ctx, repointerface.UserListFilter{}, 0, 10)
	require.NoError(t, err)
	require.Len(t, all, 3)

	require.NoError(t, repo.ChangeActiveUser(ctx, &domain.User{ID: all[0].ID, Active: true}))

	t.Run("filter by role", func(t *testing.T) {
		got, err := repo.List(ctx, repointerface.UserListFilter{RoleID: domain.RoleAdmin}, 0, 10)
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("filter active only", func(t *testing.T) {
		got, err := repo.List(ctx, repointerface.UserListFilter{ActiveOnly: true}, 0, 10)
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("pagination", func(t *testing.T) {
		page1, err := repo.List(ctx, repointerface.UserListFilter{}, 0, 2)
		require.NoError(t, err)
		page2, err := repo.List(ctx, repointerface.UserListFilter{}, 2, 2)
		require.NoError(t, err)
		assert.Len(t, page1, 2)
		assert.Len(t, page2, 1)
		// Хуудсуудын хооронд ID-ууд давхцах ёсгүй.
		assert.NotEqual(t, page1[0].ID, page2[0].ID)
	})
}

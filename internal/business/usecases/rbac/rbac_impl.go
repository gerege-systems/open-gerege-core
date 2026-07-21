// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package rbac

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

const (
	adminRoleKey      = "admin"
	superAdminRoleKey = "superadmin"
)

// cacheTTL нь Resolve-ийн кэшийн нас. Бичих үед шууд invalidate хийдэг тул энэ нь
// зөвхөн ховор race-ийн (жишээ нь сервер сэргэх үеийн) хуучирсан бичлэгийг өөрөө
// эдгээх хамгаалалт.
const cacheTTL = 60 * time.Second

type cacheEntry struct {
	keys []string
	exp  time.Time
}

type usecase struct {
	repo repointerface.RBACRepository

	mu    sync.RWMutex
	cache map[int]cacheEntry // roleID -> resolved permission keys (+ expiry)
	now   func() time.Time
}

func NewUsecase(repo repointerface.RBACRepository) Usecase {
	return &usecase{repo: repo, cache: map[int]cacheEntry{}, now: time.Now}
}

func (u *usecase) invalidate(roleID int) {
	u.mu.Lock()
	delete(u.cache, roleID)
	u.mu.Unlock()
}

func (u *usecase) ListRoles(ctx context.Context) ([]RoleWithPerms, error) {
	roles, err := u.repo.ListRoles(ctx)
	if err != nil {
		return nil, mapRepoError(err, "list roles")
	}
	out := make([]RoleWithPerms, 0, len(roles))
	for _, r := range roles {
		keys, err := u.repo.GetRolePermissions(ctx, r.ID)
		if err != nil {
			return nil, mapRepoError(err, "get role permissions")
		}
		out = append(out, RoleWithPerms{Role: r, Permissions: keys})
	}
	return out, nil
}

func (u *usecase) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	perms, err := u.repo.ListPermissions(ctx)
	if err != nil {
		return nil, mapRepoError(err, "list permissions")
	}
	return perms, nil
}

func (u *usecase) CreateRole(ctx context.Context, req CreateRoleRequest) (domain.Role, error) {
	key := slugifyKey(req.Key, req.Name)
	if key == "" {
		return domain.Role{}, apperror.BadRequest("role key is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return domain.Role{}, apperror.BadRequest("role name is required")
	}
	role, err := u.repo.CreateRole(ctx, &domain.Role{Key: key, Name: req.Name, Description: req.Description})
	if err != nil {
		return domain.Role{}, mapRepoError(err, "create role")
	}
	if req.Permissions != nil {
		if err := u.repo.SetRolePermissions(ctx, role.ID, req.Permissions); err != nil {
			return domain.Role{}, mapRepoError(err, "set role permissions")
		}
	}
	u.invalidate(role.ID)
	return role, nil
}

func (u *usecase) UpdateRole(ctx context.Context, req UpdateRoleRequest) (domain.Role, error) {
	if strings.TrimSpace(req.Name) == "" {
		return domain.Role{}, apperror.BadRequest("role name is required")
	}
	role, err := u.repo.UpdateRole(ctx, &domain.Role{ID: req.ID, Name: req.Name, Description: req.Description})
	if err != nil {
		return domain.Role{}, mapRepoError(err, "update role")
	}
	if req.Permissions != nil {
		if err := u.repo.SetRolePermissions(ctx, req.ID, req.Permissions); err != nil {
			return domain.Role{}, mapRepoError(err, "set role permissions")
		}
	}
	u.invalidate(req.ID)
	return role, nil
}

func (u *usecase) DeleteRole(ctx context.Context, id int) error {
	// Ашиглагдаж буй эрхийг устгуулахгүй.
	n, err := u.repo.CountUsersWithRole(ctx, id)
	if err != nil {
		return mapRepoError(err, "count users with role")
	}
	if n > 0 {
		return apperror.Conflict("role is assigned to users")
	}
	if err := u.repo.DeleteRole(ctx, id); err != nil {
		return mapRepoError(err, "delete role")
	}
	u.invalidate(id)
	return nil
}

func (u *usecase) SetRolePermissions(ctx context.Context, roleID int, keys []string) error {
	if _, err := u.repo.GetRole(ctx, roleID); err != nil {
		return mapRepoError(err, "get role")
	}
	if err := u.repo.SetRolePermissions(ctx, roleID, keys); err != nil {
		return mapRepoError(err, "set role permissions")
	}
	u.invalidate(roleID)
	return nil
}

// Resolve нь нэг role-ийн эрхийн түлхүүрүүдийг буцаана (кэштэй). admin эрх нь
// каталогийн бүх эрхийг автоматаар авна (шинэ эрх нэмэгдсэн ч хамаарна).
func (u *usecase) Resolve(ctx context.Context, roleID int) ([]string, error) {
	u.mu.RLock()
	if cached, ok := u.cache[roleID]; ok && u.now().Before(cached.exp) {
		u.mu.RUnlock()
		return cached.keys, nil
	}
	u.mu.RUnlock()

	role, err := u.repo.GetRole(ctx, roleID)
	if err != nil {
		return nil, mapRepoError(err, "get role")
	}

	var keys []string
	// admin болон superadmin хоёулаа каталогийн БҮХ эрхэд auto-resolve хийгдэнэ
	// (super admin нь admin-аас дээгүүр зэрэглэл). Хүсэлтийн үед IsAdmin bypass
	// аль хэдийн хамардаг ч энэ нь Resolve-ийг шууд дуудагчдад ч зөв хэвээр
	// байлгана.
	if role.Key == adminRoleKey || role.Key == superAdminRoleKey {
		perms, err := u.repo.ListPermissions(ctx)
		if err != nil {
			return nil, mapRepoError(err, "list permissions")
		}
		for _, p := range perms {
			keys = append(keys, p.Key)
		}
	} else {
		keys, err = u.repo.GetRolePermissions(ctx, roleID)
		if err != nil {
			return nil, mapRepoError(err, "get role permissions")
		}
	}
	sort.Strings(keys)

	u.mu.Lock()
	u.cache[roleID] = cacheEntry{keys: keys, exp: u.now().Add(cacheTTL)}
	u.mu.Unlock()
	return keys, nil
}

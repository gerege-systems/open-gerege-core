// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"fmt"

	repointerface "template/internal/datasources/repositories/interface"
)

// List нь admin удирдлагад зориулж хэрэглэгчдийг хуудаслан буцаана. Кэш
// ашиглахгүй — admin жагсаалт нь үргэлж шинэ өгөгдөл харах ёстой.
func (uc *usecase) List(ctx context.Context, req ListRequest) (ListResponse, error) {
	list, err := uc.repo.List(ctx, repointerface.UserListFilter{
		RoleID:         req.RoleID,
		ActiveOnly:     req.ActiveOnly,
		IncludeDeleted: req.IncludeDeleted,
	}, req.Offset, req.Limit)
	if err != nil {
		return ListResponse{}, mapRepoError(err, "list users")
	}
	return ListResponse{Users: list}, nil
}

// UpdateRole нь хэрэглэгчийн role-г солино. Эхлээд GetByID-ээр оршихыг шалгаж,
// email-ийг авч (кэш цэвэрлэхэд) дараа нь role-г шинэчилнэ.
func (uc *usecase) UpdateRole(ctx context.Context, req UpdateRoleRequest) error {
	existing, err := uc.repo.GetByID(ctx, req.UserID)
	if err != nil {
		return mapRepoError(err, "get user by id")
	}
	if err := uc.repo.UpdateRole(ctx, req.UserID, req.RoleID); err != nil {
		return mapRepoError(err, "update role")
	}
	uc.ristrettoCache.Del(fmt.Sprintf("user/%s", existing.Email))
	return nil
}

// SetActive нь хэрэглэгчийг идэвхжүүлэх/идэвхгүй болгоно.
func (uc *usecase) SetActive(ctx context.Context, req SetActiveRequest) error {
	existing, err := uc.repo.GetByID(ctx, req.UserID)
	if err != nil {
		return mapRepoError(err, "get user by id")
	}
	existing.Active = req.Active
	if err := uc.repo.ChangeActiveUser(ctx, &existing); err != nil {
		return mapRepoError(err, "set active")
	}
	uc.ristrettoCache.Del(fmt.Sprintf("user/%s", existing.Email))
	return nil
}

// Delete нь хэрэглэгчийг зөөлөн устгана (deleted_at).
func (uc *usecase) Delete(ctx context.Context, req DeleteRequest) error {
	existing, err := uc.repo.GetByID(ctx, req.UserID)
	if err != nil {
		return mapRepoError(err, "get user by id")
	}
	if err := uc.repo.SoftDelete(ctx, req.UserID); err != nil {
		return mapRepoError(err, "soft delete")
	}
	uc.ristrettoCache.Del(fmt.Sprintf("user/%s", existing.Email))
	return nil
}

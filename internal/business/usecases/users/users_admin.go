// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
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

// ListAdmins нь админ түвшний бүх бүртгэлийг (super admin + admin) буцаана.
// Кэш ашиглахгүй — жагсаалт үргэлж шинэ өгөгдөл харах ёстой.
func (uc *usecase) ListAdmins(ctx context.Context) (ListResponse, error) {
	list, err := uc.repo.ListAdmins(ctx)
	if err != nil {
		return ListResponse{}, mapRepoError(err, "list admins")
	}
	return ListResponse{Users: list}, nil
}

// UpdateRole нь хэрэглэгчийн role-г солино. Эхлээд GetByID-ээр оршихыг шалгаж,
// email-ийг авч (кэш цэвэрлэхэд) дараа нь role-г шинэчилнэ.
//
// Хамгаалалт (privilege-escalation): super admin зэрэглэлийг энэ замаар ХЭЗЭЭ Ч
// оноож болохгүй (зөвхөн bootstrap/DB), мөн super admin бүртгэлийг энэ замаар
// өөрчилж болохгүй — эс бөгөөс users.manage эрхтэй энгийн admin өөр бүртгэлийг
// super admin болгож эрх нэмэгдүүлэх, эсвэл super admin-г буулгах боломжтой болно.
func (uc *usecase) UpdateRole(ctx context.Context, req UpdateRoleRequest) error {
	if req.RoleID == domain.RoleSuperAdmin {
		return apperror.Forbidden("cannot assign the super admin role")
	}
	existing, err := uc.repo.GetByID(ctx, req.UserID)
	if err != nil {
		return mapRepoError(err, "get user by id")
	}
	if existing.IsSuperAdmin() {
		return apperror.Forbidden("cannot modify a super admin account")
	}
	if err := uc.repo.UpdateRole(ctx, req.UserID, req.RoleID); err != nil {
		return mapRepoError(err, "update role")
	}
	uc.ristrettoCache.Del(fmt.Sprintf("user/%s", existing.Email))
	return nil
}

// SetActive нь хэрэглэгчийг идэвхжүүлэх/идэвхгүй болгоно. Super admin бүртгэлийг
// энэ замаар идэвхгүй болгож болохгүй (доорх хамгаалалт).
func (uc *usecase) SetActive(ctx context.Context, req SetActiveRequest) error {
	existing, err := uc.repo.GetByID(ctx, req.UserID)
	if err != nil {
		return mapRepoError(err, "get user by id")
	}
	if existing.IsSuperAdmin() {
		return apperror.Forbidden("cannot modify a super admin account")
	}
	existing.Active = req.Active
	if err := uc.repo.ChangeActiveUser(ctx, &existing); err != nil {
		return mapRepoError(err, "set active")
	}
	uc.ristrettoCache.Del(fmt.Sprintf("user/%s", existing.Email))
	return nil
}

// Delete нь хэрэглэгчийг зөөлөн устгана (deleted_at). Super admin бүртгэлийг
// энэ замаар устгаж болохгүй (доорх хамгаалалт).
func (uc *usecase) Delete(ctx context.Context, req DeleteRequest) error {
	existing, err := uc.repo.GetByID(ctx, req.UserID)
	if err != nil {
		return mapRepoError(err, "get user by id")
	}
	if existing.IsSuperAdmin() {
		return apperror.Forbidden("cannot modify a super admin account")
	}
	if err := uc.repo.SoftDelete(ctx, req.UserID); err != nil {
		return mapRepoError(err, "soft delete")
	}
	uc.ristrettoCache.Del(fmt.Sprintf("user/%s", existing.Email))
	return nil
}

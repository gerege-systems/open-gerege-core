// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package superadmin

import (
	"context"
	"errors"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/audit"
	"template/internal/business/usecases/users"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/logger"
)

// Audit action-ууд (hash-chained audit log). category нь бүгд "superadmin".
const (
	actionCreateAdmin  = "superadmin.create_admin"
	actionGrantAdmin   = "superadmin.grant_admin"
	actionRevokeAdmin  = "superadmin.revoke_admin"
	actionCreateInvite = "superadmin.create_invite"
	actionDeleteInvite = "superadmin.delete_invite"
	auditCategory      = "superadmin"
)

// usecase нь users use case (кэш-зөв мутациуд), audit log болон superadmin
// урилгын repo-оос хамаарна. ListAdmins/Store/UpdateRole/SetActive-ийг users
// давхаргаар дуудсанаар кэш цэвэрлэлт болон domain баталгаажуулалтыг
// давхардуулахгүй дахин ашиглана.
type usecase struct {
	usersUC  users.Usecase
	auditUC  audit.Usecase
	invites  repointerface.SuperadminInviteRepository
	platform AccessModeStore
}

// NewUsecase нь super admin use case-ийг үүсгэнэ. invitesRepo нь nil байж болно
// (урилгын endpoint-ууд тухайн үед "тохируулаагүй" алдаа буцаана). platform нь
// платформын хандалтын горим (public|private) store.
func NewUsecase(usersUC users.Usecase, auditUC audit.Usecase, invitesRepo repointerface.SuperadminInviteRepository, platform AccessModeStore) Usecase {
	return &usecase{usersUC: usersUC, auditUC: auditUC, invites: invitesRepo, platform: platform}
}

// GetAccessMode нь платформын хандалтын горимыг буцаана (public|private).
func (uc *usecase) GetAccessMode(ctx context.Context) (string, error) {
	return uc.platform.GetAccessMode(ctx)
}

// SetAccessMode нь платформын хандалтын горимыг тохируулна (validation нь store-д).
func (uc *usecase) SetAccessMode(ctx context.Context, mode string) error {
	return uc.platform.SetAccessMode(ctx, mode)
}

func (uc *usecase) ListAdmins(ctx context.Context) (ListAdminsResponse, error) {
	res, err := uc.usersUC.ListAdmins(ctx)
	if err != nil {
		return ListAdminsResponse{}, err
	}
	return ListAdminsResponse{Admins: res.Users}, nil
}

// CreateAdmin нь шинэ, идэвхтэй admin бүртгэл үүсгэнэ. users.Store нь идэвхгүй
// (active=false) мөр оруулдаг тул дараа нь SetActive-ээр идэвхжүүлнэ.
func (uc *usecase) CreateAdmin(ctx context.Context, req CreateAdminRequest) (CreateAdminResponse, error) {
	stored, err := uc.usersUC.Store(ctx, users.StoreRequest{User: &domain.User{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		FirstNameEn: req.FirstNameEn,
		LastNameEn:  req.LastNameEn,
		RoleID:      domain.RoleAdmin,
	}})
	if err != nil {
		return CreateAdminResponse{}, err
	}
	// Шинэ админ шууд ажиллах боломжтой байх ёстой тул идэвхжүүлнэ.
	if err := uc.usersUC.SetActive(ctx, users.SetActiveRequest{UserID: stored.User.ID, Active: true}); err != nil {
		return CreateAdminResponse{}, err
	}
	stored.User.Active = true

	uc.record(ctx, actionCreateAdmin, stored.User.ID, map[string]any{
		"email":    stored.User.Email,
		"username": stored.User.Username,
	})
	return CreateAdminResponse{User: stored.User}, nil
}

// GrantAdmin нь байгаа хэрэглэгчид admin эрх олгоно.
func (uc *usecase) GrantAdmin(ctx context.Context, req GrantAdminRequest) error {
	existing, err := uc.usersUC.GetByID(ctx, users.GetByIDRequest{ID: req.UserID})
	if err != nil {
		return err
	}
	// Аль хэдийн админ түвшний бол дахин олгох нь утгагүй (idempotent conflict).
	if existing.User.IsAdmin() {
		return apperror.Conflict("user is already an admin")
	}
	// CallerRoleID — энэ usecase нь route түвшинд RequireSuperAdmin-аар
	// хамгаалагдсан тул дуудагч нь super admin. Admin эрхийг зөвхөн super admin
	// олгодог шалгалтыг (users.UpdateRole) давахад шаардлагатай.
	if err := uc.usersUC.UpdateRole(ctx, users.UpdateRoleRequest{
		UserID: req.UserID, RoleID: domain.RoleAdmin, CallerRoleID: domain.RoleSuperAdmin,
	}); err != nil {
		return err
	}
	uc.record(ctx, actionGrantAdmin, req.UserID, map[string]any{
		"email": existing.User.Email,
	})
	return nil
}

// AddAdminByRegister нь регистрийн дугаараар БАЙГАА хэрэглэгчийг admin болгоно.
// Тухайн регистрээр DAN-д хэрэглэгч байхгүй бол татгалзана (шинэ хэрэглэгч
// үүсгэхгүй) — тэр хүн эхлээд eID-ээр нэвтэрсэн байх ёстой.
func (uc *usecase) AddAdminByRegister(ctx context.Context, req AddAdminByRegisterRequest) (CreateAdminResponse, error) {
	register := strings.ToUpper(strings.TrimSpace(req.Register))
	if register == "" {
		return CreateAdminResponse{}, apperror.BadRequest("register is required")
	}
	found, err := uc.usersUC.GetByNationalID(ctx, users.GetByNationalIDRequest{NationalID: register})
	if err != nil {
		var domErr *apperror.DomainError
		if errors.As(err, &domErr) && domErr.Type == apperror.ErrTypeNotFound {
			return CreateAdminResponse{}, apperror.NotFound("this register is not registered in DAN — the person must sign in via eID first")
		}
		return CreateAdminResponse{}, err
	}
	if found.User.IsAdmin() {
		return CreateAdminResponse{}, apperror.Conflict("user is already an admin")
	}
	if err := uc.GrantAdmin(ctx, GrantAdminRequest{UserID: found.User.ID}); err != nil {
		return CreateAdminResponse{}, err
	}
	found.User.RoleID = domain.RoleAdmin
	return CreateAdminResponse{User: found.User}, nil
}

// LookupByRegister нь регистрийн дугаараар DAN-д БАЙГАА хэрэглэгчийг олж буцаана
// (preview — эрх олгохгүй). Байхгүй бол NotFound.
func (uc *usecase) LookupByRegister(ctx context.Context, register string) (CreateAdminResponse, error) {
	register = strings.ToUpper(strings.TrimSpace(register))
	if register == "" {
		return CreateAdminResponse{}, apperror.BadRequest("register is required")
	}
	found, err := uc.usersUC.GetByNationalID(ctx, users.GetByNationalIDRequest{NationalID: register})
	if err != nil {
		var domErr *apperror.DomainError
		if errors.As(err, &domErr) && domErr.Type == apperror.ErrTypeNotFound {
			return CreateAdminResponse{}, apperror.NotFound("this register is not registered in DAN — the person must sign in via eID first")
		}
		return CreateAdminResponse{}, err
	}
	return CreateAdminResponse{User: found.User}, nil
}

// RevokeAdmin нь admin эрхийг хасч, энгийн хэрэглэгч болгоно.
func (uc *usecase) RevokeAdmin(ctx context.Context, req RevokeAdminRequest) error {
	// Lockout-аас сэргийлэх: super admin өөрийгөө хасаж болохгүй.
	if req.UserID == req.ActorID {
		return apperror.Forbidden("you cannot revoke your own access")
	}
	existing, err := uc.usersUC.GetByID(ctx, users.GetByIDRequest{ID: req.UserID})
	if err != nil {
		return err
	}
	// Зорилтот нь яг RoleAdmin байх ёстой. super admin-г API-аар хасахгүй
	// (зөвхөн DB/bootstrap); энгийн хэрэглэгчийг "хасах" нь утгагүй.
	if existing.User.IsSuperAdmin() {
		return apperror.Forbidden("a super admin cannot be revoked")
	}
	if existing.User.RoleID != domain.RoleAdmin {
		return apperror.BadRequest("user is not an admin")
	}
	// Дуудагч нь super admin (RequireSuperAdmin) — admin бүртгэлийг өөрчлөх
	// шалгалтыг давахад шаардлагатай.
	if err := uc.usersUC.UpdateRole(ctx, users.UpdateRoleRequest{
		UserID: req.UserID, RoleID: domain.RoleUser, CallerRoleID: domain.RoleSuperAdmin,
	}); err != nil {
		return err
	}
	uc.record(ctx, actionRevokeAdmin, req.UserID, map[string]any{
		"email": existing.User.Email,
	})
	return nil
}

// record нь audit үйл явдлыг best-effort (non-fatal) бичнэ — actor-г хүсэлтийн
// RLS context-оос audit давхарга өөрөө уншина. Бичих алдаа нь үндсэн үйлдлийг
// бүтэлгүйтүүлэхгүй (existing flow-ийн адил), зөвхөн warning бичнэ.
func (uc *usecase) record(ctx context.Context, action, target string, metadata map[string]any) {
	if err := uc.auditUC.RecordEvent(ctx, action, auditCategory, target, metadata); err != nil {
		logger.WarnWithContext(ctx, "superadmin audit event bичих амжилтгүй", logger.Fields{
			"action": action, "target": target, "error": err.Error(),
		})
	}
}

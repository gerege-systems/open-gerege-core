// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package org

import (
	"context"
	"fmt"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

// usecase нь хамаарлууд болон method хоорондын төлөвийг агуулдаг. Нэг зан төлөв
// өөрчлөгдөхөд PR-ийн diff нарийн хэвээр үлдэхийн тулд method бүр өөрийн файлд
// байрладаг.
type usecase struct {
	repo repointerface.OrgRepository
}

// NewUsecase нь байгууллагын use case-ийг үүсгэнэ.
func NewUsecase(repo repointerface.OrgRepository) Usecase {
	return &usecase{repo: repo}
}

// mapRepoError нь repository-ээс буцсан DomainError төрлүүдийг хадгалж, харин
// түүхий алдаануудыг форматтай дотоод алдаагаар боодог. Үүнгүйгээр дээд урсгал
// дахь errors.As(err, *DomainError) амжилтгүй болно.
func mapRepoError(err error, op string) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*apperror.DomainError); ok {
		return err
	}
	return apperror.InternalCause(fmt.Errorf("%s: %w", op, err))
}

// requireManager нь дуудагч тухайн байгууллагад owner/admin гишүүн эсэхийг
// шалгана — гишүүн нэмэх/хасах/дүр солих эрхийн нэгдсэн хаалга. Гишүүн биш бол
// apperror.Forbidden; owner/admin биш бол apperror.Forbidden буцна. Амжилттай
// бол дуудагчийн гишүүнчлэлийг буцаана — owner-only дүрмүүд (owner дүр олгох
// г.м.) дуудагчийн дүрийг шаарддаг.
func (uc *usecase) requireManager(ctx context.Context, orgID, callerID string) (domain.OrganizationMembership, error) {
	m, err := uc.repo.GetMembership(ctx, orgID, callerID)
	if err != nil {
		if _, ok := err.(*apperror.DomainError); ok {
			// Гишүүн биш дуудагч руу "байхгүй" гэдгийг илчлэхгүйгээр
			// Forbidden буцаана.
			return domain.OrganizationMembership{}, apperror.Forbidden("you are not allowed to manage this organization")
		}
		return domain.OrganizationMembership{}, mapRepoError(err, "get membership")
	}
	if !domain.CanManageMembers(m.Role) {
		return domain.OrganizationMembership{}, apperror.Forbidden("you are not allowed to manage this organization")
	}
	return m, nil
}

// requireMember нь дуудагч тухайн байгууллагын гишүүн эсэхийг шалгана (унших
// эрх). Гишүүн биш бол apperror.Forbidden.
func (uc *usecase) requireMember(ctx context.Context, orgID, callerID string) error {
	if _, err := uc.repo.GetMembership(ctx, orgID, callerID); err != nil {
		if _, ok := err.(*apperror.DomainError); ok {
			return apperror.Forbidden("you are not a member of this organization")
		}
		return mapRepoError(err, "get membership")
	}
	return nil
}

// CreateOrganization нь шинэ байгууллага үүсгэж, дуудагчийг owner болгоно.
func (uc *usecase) CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (OrganizationResponse, error) {
	regNo := strings.TrimSpace(req.RegNo)
	name := strings.TrimSpace(req.Name)
	if regNo == "" {
		return OrganizationResponse{}, apperror.BadRequest("registration number is required")
	}
	if name == "" {
		return OrganizationResponse{}, apperror.BadRequest("organization name is required")
	}
	org := &domain.Organization{
		RegNo:     regNo,
		Name:      name,
		NameLatin: strings.TrimSpace(req.NameLatin),
		CreatedBy: req.CallerID,
	}
	stored, err := uc.repo.CreateOrg(ctx, org)
	if err != nil {
		return OrganizationResponse{}, mapRepoError(err, "create organization")
	}
	return OrganizationResponse{Organization: stored}, nil
}

// ListMyOrganizations нь дуудагч гишүүн болсон байгууллагуудыг буцаана.
func (uc *usecase) ListMyOrganizations(ctx context.Context, req ListMyOrganizationsRequest) (ListOrganizationsResponse, error) {
	list, err := uc.repo.ListOrgsForUser(ctx, req.CallerID)
	if err != nil {
		return ListOrganizationsResponse{}, mapRepoError(err, "list organizations")
	}
	return ListOrganizationsResponse{Organizations: list}, nil
}

// GetOrganization нь нэг байгууллагыг буцаана. RLS нь энгийн хэрэглэгчид зөвхөн
// гишүүн болсон org-оо харуулдаг тул repo-гийн NotFound нь "байхгүй эсвэл хандах
// эрхгүй"-г хоёуланг хамарна.
func (uc *usecase) GetOrganization(ctx context.Context, req GetOrganizationRequest) (OrganizationResponse, error) {
	org, err := uc.repo.GetOrgByID(ctx, req.OrgID)
	if err != nil {
		return OrganizationResponse{}, mapRepoError(err, "get organization")
	}
	return OrganizationResponse{Organization: org}, nil
}

// LookupByRegNo нь бүртгэлийн дугаараар байгууллагыг хайна.
func (uc *usecase) LookupByRegNo(ctx context.Context, req LookupByRegNoRequest) (OrganizationResponse, error) {
	regNo := strings.TrimSpace(req.RegNo)
	if regNo == "" {
		return OrganizationResponse{}, apperror.BadRequest("registration number is required")
	}
	org, err := uc.repo.GetOrgByRegNo(ctx, regNo)
	if err != nil {
		return OrganizationResponse{}, mapRepoError(err, "lookup organization")
	}
	return OrganizationResponse{Organization: org}, nil
}

// ListMembers нь байгууллагын гишүүдийг буцаана (дуудагч гишүүн байх ёстой).
func (uc *usecase) ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error) {
	if err := uc.requireMember(ctx, req.OrgID, req.CallerID); err != nil {
		return ListMembersResponse{}, err
	}
	members, err := uc.repo.ListMembers(ctx, req.OrgID)
	if err != nil {
		return ListMembersResponse{}, mapRepoError(err, "list members")
	}
	return ListMembersResponse{Members: members}, nil
}

// AddMember нь гишүүн нэмнэ (дуудагч owner/admin байх ёстой).
func (uc *usecase) AddMember(ctx context.Context, req AddMemberRequest) (MembershipResponse, error) {
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = domain.OrgRoleMember
	}
	if !domain.IsValidOrgRole(role) {
		return MembershipResponse{}, apperror.BadRequest("invalid membership role")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return MembershipResponse{}, apperror.BadRequest("user id is required")
	}
	caller, err := uc.requireManager(ctx, req.OrgID, req.CallerID)
	if err != nil {
		return MembershipResponse{}, err
	}
	// owner дүрийг зөвхөн owner олгоно — org admin өөрөөсөө дээш дүр
	// (өөрт нь эсвэл бусдад) олгож эрх ахиулахаас сэргийлнэ.
	if role == domain.OrgRoleOwner && caller.Role != domain.OrgRoleOwner {
		return MembershipResponse{}, apperror.Forbidden("only the owner can grant the owner role")
	}
	m, err := uc.repo.AddMember(ctx, &domain.OrganizationMembership{
		OrgID:  req.OrgID,
		UserID: req.UserID,
		Role:   role,
	})
	if err != nil {
		return MembershipResponse{}, mapRepoError(err, "add member")
	}
	return MembershipResponse{Membership: m}, nil
}

// UpdateMemberRole нь гишүүний дүрийг солино (дуудагч owner/admin байх ёстой).
// Owner-ийн дүрд хоёр нэмэлт хамгаалалт бий: owner дүрийг зөвхөн owner олгоно,
// мөн owner-ийн дүрийг өөрчилж болохгүй — эс бөгөөс org admin owner-ыг member
// болгож бууруулаад дараа нь RemoveMember-ээр хасч, "owner-ыг хасахгүй"
// хамгаалалтыг тойрч гарна.
func (uc *usecase) UpdateMemberRole(ctx context.Context, req UpdateMemberRoleRequest) error {
	role := strings.TrimSpace(req.Role)
	if !domain.IsValidOrgRole(role) {
		return apperror.BadRequest("invalid membership role")
	}
	caller, err := uc.requireManager(ctx, req.OrgID, req.CallerID)
	if err != nil {
		return err
	}
	if role == domain.OrgRoleOwner && caller.Role != domain.OrgRoleOwner {
		return apperror.Forbidden("only the owner can grant the owner role")
	}
	target, err := uc.repo.GetMembership(ctx, req.OrgID, req.UserID)
	if err != nil {
		return mapRepoError(err, "get target membership")
	}
	if target.Role == domain.OrgRoleOwner {
		return apperror.BadRequest("the organization owner's role cannot be changed")
	}
	if err := uc.repo.UpdateMemberRole(ctx, req.OrgID, req.UserID, role); err != nil {
		return mapRepoError(err, "update member role")
	}
	return nil
}

// RemoveMember нь гишүүнийг хасна (дуудагч owner/admin байх ёстой). Owner-ийг
// хасахаас сэргийлнэ — байгууллага эзэнгүй үлдэхээс хамгаална.
func (uc *usecase) RemoveMember(ctx context.Context, req RemoveMemberRequest) error {
	if _, err := uc.requireManager(ctx, req.OrgID, req.CallerID); err != nil {
		return err
	}
	target, err := uc.repo.GetMembership(ctx, req.OrgID, req.UserID)
	if err != nil {
		return mapRepoError(err, "get target membership")
	}
	if target.Role == domain.OrgRoleOwner {
		return apperror.BadRequest("the organization owner cannot be removed")
	}
	if err := uc.repo.RemoveMember(ctx, req.OrgID, req.UserID); err != nil {
		return mapRepoError(err, "remove member")
	}
	return nil
}

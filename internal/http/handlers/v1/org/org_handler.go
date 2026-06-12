// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package org нь /v1/org/* HTTP endpoint-уудыг үйлчилнэ — байгууллага үүсгэх,
// жагсаах, харах, дугаараар хайх, гишүүн удирдах. Бүх endpoint нэвтрэлт
// шаардана; эрх олголт (owner/admin эсэх) нь usecase давхаргад хэрэгждэг.
package org

import (
	"context"
	"net/http"

	audituc "template/internal/business/usecases/audit"
	orguc "template/internal/business/usecases/org"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

// Handler нь org-домэйн endpoint-уудыг үйлчилнэ. Зөвхөн org.Usecase руу
// дууддаг — хэзээ ч repository эсвэл DB руу шууд дууддаггүй.
//
// auditUC нь persisted hash-chained audit log (org үүсгэх, гишүүн нэмэх/хасах
// үед best-effort бичлэг). nil байж болно — тэр үед audit бичлэг алгасагдана.
type Handler struct {
	usecase orguc.Usecase
	auditUC audituc.Usecase
}

func NewHandler(usecase orguc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// NewHandlerWithAudit нь audit use case-ийг тарьж handler үүсгэнэ.
func NewHandlerWithAudit(usecase orguc.Usecase, auditUC audituc.Usecase) Handler {
	return Handler{usecase: usecase, auditUC: auditUC}
}

// recordAudit нь org үйл явдлыг persisted audit log руу best-effort бичнэ —
// амжилтгүй болсон ч HTTP урсгалыг эвдэхгүй (зөвхөн log).
func (h Handler) recordAudit(ctx context.Context, action, target string, metadata map[string]any) {
	if h.auditUC == nil {
		return
	}
	if err := h.auditUC.RecordEvent(ctx, action, "org", target, metadata); err != nil {
		logger.ErrorWithContext(ctx, "org: persisted audit write failed (non-fatal)", logger.Fields{
			"action": action,
			"error":  err.Error(),
		})
	}
}

// CreateOrganization godoc
// @Summary      Шинэ байгууллага үүсгэх
// @Description  Нэвтэрсэн хэрэглэгч шинэ байгууллага бүртгэнэ; үүсгэгч автоматаар owner гишүүн болно.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      requests.CreateOrgRequest  true  "Байгууллагын мэдээлэл"
// @Success      201  {object}  v1.BaseResponse{data=responses.OrgResponse}  "Created organization"
// @Failure      400  {object}  v1.BaseResponse  "Invalid request body"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      409  {object}  v1.BaseResponse  "Registration number already exists"
// @Router       /v1/org [post]
func (h Handler) CreateOrganization(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	var req requests.CreateOrgRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	resp, err := h.usecase.CreateOrganization(r.Context(), orguc.CreateOrganizationRequest{
		CallerID: user.ID, RegNo: req.RegNo, Name: req.Name, NameLatin: req.NameLatin,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	h.recordAudit(r.Context(), "org.create", resp.Organization.ID, map[string]any{
		"reg_no": resp.Organization.RegNo,
	})
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "organization created successfully", responses.FromOrg(resp.Organization))
}

// ListMyOrganizations godoc
// @Summary      Өөрийн байгууллагуудыг жагсаах
// @Description  Нэвтэрсэн хэрэглэгч гишүүн болсон бүх байгууллагыг буцаана.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=[]responses.OrgResponse}  "Organizations"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Router       /v1/org [get]
func (h Handler) ListMyOrganizations(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	resp, err := h.usecase.ListMyOrganizations(r.Context(), orguc.ListMyOrganizationsRequest{CallerID: user.ID})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "organizations fetched successfully", responses.ToOrgList(resp.Organizations))
}

// GetOrganization godoc
// @Summary      Нэг байгууллагыг харах
// @Description  ID-аар байгууллагыг буцаана; дуудагч заавал гишүүн (эсвэл admin) байх ёстой.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Байгууллагын ID (uuid)"
// @Success      200  {object}  v1.BaseResponse{data=responses.OrgResponse}  "Organization"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      404  {object}  v1.BaseResponse  "Organization not found or not accessible"
// @Router       /v1/org/{id} [get]
func (h Handler) GetOrganization(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	resp, err := h.usecase.GetOrganization(r.Context(), orguc.GetOrganizationRequest{
		CallerID: user.ID, OrgID: chi.URLParam(r, "id"),
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "organization fetched successfully", responses.FromOrg(resp.Organization))
}

// LookupByRegNo godoc
// @Summary      Бүртгэлийн дугаараар хайх
// @Description  Улсын бүртгэлийн дугаараар байгууллагыг хайна (зөвхөн дуудагчид харагдах org).
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        regNo  path      string  true  "Бүртгэлийн дугаар"
// @Success      200  {object}  v1.BaseResponse{data=responses.OrgResponse}  "Organization"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      404  {object}  v1.BaseResponse  "Organization not found"
// @Router       /v1/org/lookup/{regNo} [get]
func (h Handler) LookupByRegNo(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	resp, err := h.usecase.LookupByRegNo(r.Context(), orguc.LookupByRegNoRequest{
		CallerID: user.ID, RegNo: chi.URLParam(r, "regNo"),
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "organization fetched successfully", responses.FromOrg(resp.Organization))
}

// ListMembers godoc
// @Summary      Байгууллагын гишүүдийг жагсаах
// @Description  Тухайн байгууллагын бүх гишүүнийг буцаана; дуудагч гишүүн байх ёстой.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Байгууллагын ID (uuid)"
// @Success      200  {object}  v1.BaseResponse{data=[]responses.OrgMemberResponse}  "Members"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a member"
// @Router       /v1/org/{id}/members [get]
func (h Handler) ListMembers(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	resp, err := h.usecase.ListMembers(r.Context(), orguc.ListMembersRequest{
		CallerID: user.ID, OrgID: chi.URLParam(r, "id"),
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "members fetched successfully", responses.ToOrgMemberList(resp.Members))
}

// AddMember godoc
// @Summary      Гишүүн нэмэх
// @Description  Байгууллагад гишүүн нэмнэ; дуудагч owner/admin байх ёстой.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                      true  "Байгууллагын ID (uuid)"
// @Param        payload  body      requests.AddMemberRequest   true  "Гишүүний мэдээлэл"
// @Success      201  {object}  v1.BaseResponse{data=responses.OrgMemberResponse}  "Membership"
// @Failure      400  {object}  v1.BaseResponse  "Invalid request body"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not allowed to manage members"
// @Failure      409  {object}  v1.BaseResponse  "Already a member"
// @Router       /v1/org/{id}/members [post]
func (h Handler) AddMember(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	var req requests.AddMemberRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	resp, err := h.usecase.AddMember(r.Context(), orguc.AddMemberRequest{
		CallerID: user.ID, OrgID: chi.URLParam(r, "id"), UserID: req.UserID, Role: req.Role,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	h.recordAudit(r.Context(), "org.member.add", chi.URLParam(r, "id"), map[string]any{
		"member_user_id": resp.Membership.UserID,
		"role":           resp.Membership.Role,
	})
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "member added successfully", responses.FromOrgMember(resp.Membership))
}

// UpdateMemberRole godoc
// @Summary      Гишүүний дүр солих
// @Description  Гишүүний дүрийг (owner/admin/member) солино; дуудагч owner/admin байх ёстой.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      string                            true  "Байгууллагын ID (uuid)"
// @Param        userID   path      string                            true  "Гишүүн хэрэглэгчийн ID (uuid)"
// @Param        payload  body      requests.UpdateMemberRoleRequest  true  "Шинэ дүр"
// @Success      200  {object}  v1.BaseResponse  "Role updated"
// @Failure      400  {object}  v1.BaseResponse  "Invalid request body"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not allowed to manage members"
// @Failure      404  {object}  v1.BaseResponse  "Membership not found"
// @Router       /v1/org/{id}/members/{userID} [put]
func (h Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	var req requests.UpdateMemberRoleRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.UpdateMemberRole(r.Context(), orguc.UpdateMemberRoleRequest{
		CallerID: user.ID, OrgID: chi.URLParam(r, "id"), UserID: chi.URLParam(r, "userID"), Role: req.Role,
	}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "member role updated successfully", nil)
}

// RemoveMember godoc
// @Summary      Гишүүн хасах
// @Description  Байгууллагаас гишүүнийг хасна; дуудагч owner/admin байх ёстой. Owner-ийг хасах боломжгүй.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id      path      string  true  "Байгууллагын ID (uuid)"
// @Param        userID  path      string  true  "Гишүүн хэрэглэгчийн ID (uuid)"
// @Success      200  {object}  v1.BaseResponse  "Member removed"
// @Failure      400  {object}  v1.BaseResponse  "Cannot remove owner"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not allowed to manage members"
// @Failure      404  {object}  v1.BaseResponse  "Membership not found"
// @Router       /v1/org/{id}/members/{userID} [delete]
func (h Handler) RemoveMember(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	if err := h.usecase.RemoveMember(r.Context(), orguc.RemoveMemberRequest{
		CallerID: user.ID, OrgID: chi.URLParam(r, "id"), UserID: chi.URLParam(r, "userID"),
	}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	h.recordAudit(r.Context(), "org.member.remove", chi.URLParam(r, "id"), map[string]any{
		"member_user_id": chi.URLParam(r, "userID"),
	})
	return v1.NewSuccessResponse(w, r, http.StatusOK, "member removed successfully", nil)
}

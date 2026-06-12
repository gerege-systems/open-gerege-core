// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package org нь байгууллага болон гишүүнчлэлийн use case-уудыг хариуцна:
// байгууллага үүсгэх, өөрийн байгууллагуудыг жагсаах, дугаараар хайх, гишүүн
// нэмэх/хасах/дүр солих. Эрх олголт (owner/admin эсэх) нь энэ давхаргад
// хэрэгждэг — RLS зөвхөн мөрийн харагдах байдлыг хариуцна.
package org

import (
	"context"

	"template/internal/business/domain"
)

// Usecase нь оролтын хил (input boundary) юм. Method бүр Request struct авч,
// (буцаах өгөгдөлтэй үед) Response struct буцаадаг тул талбар нэмэх нь
// хувилбаруудын хооронд буцах нийцтэй хэвээр үлддэг.
//
// CallerID нь Request бүрд орсон — нэвтэрсэн хэрэглэгчийн UUID. Эрх олголтын
// шийдвэрүүд (гишүүн эсэх, owner/admin эсэх) үүнд тулгуурлана.
type Usecase interface {
	// CreateOrganization нь шинэ байгууллага үүсгэж, дуудагчийг owner гишүүн
	// болгоно.
	CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (OrganizationResponse, error)
	// ListMyOrganizations нь дуудагч гишүүн болсон байгууллагуудыг буцаана.
	ListMyOrganizations(ctx context.Context, req ListMyOrganizationsRequest) (ListOrganizationsResponse, error)
	// GetOrganization нь нэг байгууллагыг буцаана. Дуудагч заавал гишүүн (эсвэл
	// admin) байх ёстой — эс бөгөөс apperror.NotFound/Forbidden.
	GetOrganization(ctx context.Context, req GetOrganizationRequest) (OrganizationResponse, error)
	// LookupByRegNo нь бүртгэлийн дугаараар байгууллагыг хайна.
	LookupByRegNo(ctx context.Context, req LookupByRegNoRequest) (OrganizationResponse, error)
	// ListMembers нь байгууллагын гишүүдийг буцаана (дуудагч гишүүн байх ёстой).
	ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error)
	// AddMember нь гишүүн нэмнэ (дуудагч owner/admin байх ёстой).
	AddMember(ctx context.Context, req AddMemberRequest) (MembershipResponse, error)
	// UpdateMemberRole нь гишүүний дүрийг солино (дуудагч owner/admin байх ёстой).
	UpdateMemberRole(ctx context.Context, req UpdateMemberRoleRequest) error
	// RemoveMember нь гишүүнийг хасна (дуудагч owner/admin байх ёстой).
	RemoveMember(ctx context.Context, req RemoveMemberRequest) error
}

// Usecase-ийн хилд зориулсан Request / Response төрлүүд.
type (
	CreateOrganizationRequest struct {
		CallerID  string
		RegNo     string
		Name      string
		NameLatin string
	}

	ListMyOrganizationsRequest struct {
		CallerID string
	}

	GetOrganizationRequest struct {
		CallerID string
		OrgID    string
	}

	LookupByRegNoRequest struct {
		CallerID string
		RegNo    string
	}

	ListMembersRequest struct {
		CallerID string
		OrgID    string
	}

	AddMemberRequest struct {
		CallerID string
		OrgID    string
		UserID   string
		Role     string
	}

	UpdateMemberRoleRequest struct {
		CallerID string
		OrgID    string
		UserID   string
		Role     string
	}

	RemoveMemberRequest struct {
		CallerID string
		OrgID    string
		UserID   string
	}

	OrganizationResponse struct {
		Organization domain.Organization
	}

	ListOrganizationsResponse struct {
		Organizations []domain.Organization
	}

	MembershipResponse struct {
		Membership domain.OrganizationMembership
	}

	ListMembersResponse struct {
		Members []domain.OrganizationMembership
	}
)

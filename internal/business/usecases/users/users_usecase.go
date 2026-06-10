// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package users нь хэрэглэгчийн identity-ийн CRUD-ийг хариуцдаг: үүсгэх, хайх,
// идэвхжүүлэх, зөөлөн устгалт болон нууц үг эргүүлэх.
package users

import (
	"context"

	"template/internal/business/domain"
)

// Usecase нь оролтын хил (input boundary) юм. Method бүр Request struct авч,
// (буцаах өгөгдөлтэй үед) Response struct буцаадаг тул талбар нэмэх нь
// хувилбаруудын хооронд буцах нийцтэй (backward-compatible) хэвээр үлддэг.
type Usecase interface {
	// Store нь шинэ User (нормчилсон email, hash хийсэн нууц үг) үүсгэж,
	// хадгална; DB-ийн үүсгэсэн ID-г оруулсан оруулсан мөрийг буцаана.
	Store(ctx context.Context, req StoreRequest) (StoreResponse, error)
	// GetByEmail нь өгөгдсөн email-тэй хэрэглэгчийг буцаана; кэш-эхэлсэн
	// (cache-first) хайлт бөгөөд алдалт (miss) дээр singleflight-аар нэгтгэдэг.
	GetByEmail(ctx context.Context, req GetByEmailRequest) (GetByEmailResponse, error)
	// GetByID нь өгөгдсөн primary key-тэй хэрэглэгчийг буцаана; кэшийг алгасна.
	GetByID(ctx context.Context, req GetByIDRequest) (GetByIDResponse, error)
	// Activate нь хэрэглэгчийн active флагийг хувиргана (OTP-баталгаажуулах урсгалаас дуудагдана).
	Activate(ctx context.Context, req ActivateRequest) error
	// UpdatePassword нь хэрэглэгчийн нууц үгийг (дуудагч аль хэдийн
	// domain.User.ChangePassword-аар hash хийсэн) сольж, password_changed_at-ийг тэмдэглэнэ.
	UpdatePassword(ctx context.Context, req UpdatePasswordRequest) error

	// List нь admin удирдлагад зориулж хэрэглэгчдийг хуудаслан буцаана.
	List(ctx context.Context, req ListRequest) (ListResponse, error)
	// UpdateRole нь хэрэглэгчийн role-г солино (admin удирдлага).
	UpdateRole(ctx context.Context, req UpdateRoleRequest) error
	// SetActive нь хэрэглэгчийг идэвхжүүлэх/идэвхгүй болгоно (admin удирдлага).
	SetActive(ctx context.Context, req SetActiveRequest) error
	// Delete нь хэрэглэгчийг зөөлөн устгана (admin удирдлага).
	Delete(ctx context.Context, req DeleteRequest) error
}

// Usecase-ийн хилд зориулсан Request / Response төрлүүд. Struct-д талбар нэмэх
// нь дуудагчдыг эвддэггүй, харин method-ийн гарын үсэгт (signature) параметр
// нэмэх нь эвддэг — Uncle Bob-ийн "Input/Output Boundary" зөвлөмжийг бодит
// байдлаар хэрэгжүүлсэн нь.
type (
	StoreRequest struct {
		User *domain.User
	}
	StoreResponse struct {
		User domain.User
	}

	GetByEmailRequest struct {
		Email string
	}
	GetByEmailResponse struct {
		User domain.User
	}

	GetByIDRequest struct {
		ID string
	}
	GetByIDResponse struct {
		User domain.User
	}

	ActivateRequest struct {
		UserID string
	}

	UpdatePasswordRequest struct {
		User *domain.User
	}

	ListRequest struct {
		RoleID         int
		ActiveOnly     bool
		IncludeDeleted bool
		Offset         int
		Limit          int
	}
	ListResponse struct {
		Users []domain.User
	}

	UpdateRoleRequest struct {
		UserID string
		RoleID int
	}

	SetActiveRequest struct {
		UserID string
		Active bool
	}

	DeleteRequest struct {
		UserID string
	}
)

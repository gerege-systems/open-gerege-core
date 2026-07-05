// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package superadmin нь super admin-ий "админуудыг удирдах" use case давхарга
// юм: админ түвшний бүртгэлүүдийг жагсаах, шинэ админ үүсгэх, байгаа хэрэглэгчид
// админ эрх олгох/хасах. Бүх мутаци нь hash-chained audit log-д бичигдэнэ.
//
// Зохион байгуулалтын дүрэм (least-privilege / зэрэглэлийн шатлал):
//   - Зөвхөн super admin (RequireSuperAdmin gate) энэ давхаргад хүрнэ.
//   - Энэ давхарга ЗӨВХӨН admin (RoleAdmin) зэрэглэлийг л үүсгэж/олгож/хасна;
//     super admin зэрэглэлийг API-аар хэзээ ч үүсгэдэггүй (bootstrap/DB-ээр л).
//   - Өөрийгөө хасаж болохгүй, super admin-г хасаж болохгүй (lockout-аас сэргийлэх).
package superadmin

import (
	"context"

	"template/internal/business/domain"
)

// Usecase нь оролтын хил (input boundary). Method бүр Request struct авч,
// (буцаах өгөгдөлтэй үед) Response struct буцаадаг тул талбар нэмэх нь
// хувилбаруудын хооронд буцаж нийцтэй хэвээр үлддэг.
type Usecase interface {
	// ListAdmins нь админ түвшний бүх бүртгэлийг (super admin + admin) буцаана.
	ListAdmins(ctx context.Context) (ListAdminsResponse, error)
	// CreateAdmin нь шинэ, идэвхтэй admin бүртгэл (нэр/и-мэйл/нууц үг) үүсгэнэ.
	// Давхардсан и-мэйл нь apperror.Conflict болж гарна.
	CreateAdmin(ctx context.Context, req CreateAdminRequest) (CreateAdminResponse, error)
	// GrantAdmin нь байгаа хэрэглэгчид admin эрх олгоно (RoleAdmin болгоно).
	// Аль хэдийн админ (admin/super admin) бол apperror.Conflict.
	GrantAdmin(ctx context.Context, req GrantAdminRequest) error
	// RevokeAdmin нь admin-ийн эрхийг хасч, энгийн хэрэглэгч (RoleUser) болгоно.
	// Зорилтот нь яг RoleAdmin байх ёстой (super admin-г хасахгүй) бөгөөд
	// дуудагч өөрийгөө хасаж болохгүй.
	RevokeAdmin(ctx context.Context, req RevokeAdminRequest) error
}

// Request / Response төрлүүд (Input/Output Boundary).
type (
	ListAdminsResponse struct {
		Admins []domain.User
	}

	CreateAdminRequest struct {
		Username    string
		Email       string
		Password    string
		FirstName   string
		LastName    string
		FirstNameEn string
		LastNameEn  string
	}
	CreateAdminResponse struct {
		User domain.User
	}

	GrantAdminRequest struct {
		UserID string
	}

	RevokeAdminRequest struct {
		// UserID нь эрхийг нь хасах хэрэглэгч.
		UserID string
		// ActorID нь үйлдлийг гүйцэтгэж буй super admin — өөрийгөө хасахаас
		// сэргийлэх (lockout guard) шалгалтад ашиглагдана.
		ActorID string
	}
)

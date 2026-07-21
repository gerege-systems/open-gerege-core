// Government Template Platform V3.0
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
	// AddAdminByRegister нь регистрийн дугаараар DAN-д (eID-ээр нэвтэрч)
	// БАЙГАА хэрэглэгчийг admin болгоно. Core (үндэсний бүртгэл) руу ХАНДАХГҮЙ —
	// зөвхөн local DAN хэрэглэгчийг лавлана. Тухайн регистрээр DAN-д бүртгэлтэй
	// хэрэглэгч байхгүй бол apperror.NotFound ("эхлээд eID-ээр нэвтэрсэн байх
	// шаардлагатай") — шинэ хэрэглэгч ҮҮСГЭХГҮЙ. Промоушн хийсэн хэрэглэгчийг буцаана.
	AddAdminByRegister(ctx context.Context, req AddAdminByRegisterRequest) (CreateAdminResponse, error)
	// LookupByRegister нь регистрийн дугаараар DAN-д БАЙГАА хэрэглэгчийг олж
	// буцаана (эрх олгохоос ӨМНӨХ preview — нэр/эрхийг харуулна). Промоушн хийхгүй.
	// Байхгүй бол apperror.NotFound.
	LookupByRegister(ctx context.Context, register string) (CreateAdminResponse, error)
	// RevokeAdmin нь admin-ийн эрхийг хасч, энгийн хэрэглэгч (RoleUser) болгоно.
	// Зорилтот нь яг RoleAdmin байх ёстой (super admin-г хасахгүй) бөгөөд
	// дуудагч өөрийгөө хасаж болохгүй.
	RevokeAdmin(ctx context.Context, req RevokeAdminRequest) error

	// ListInvites нь super admin болох урилгуудыг (хүлээгдэж буй + ашигласан)
	// буцаана.
	ListInvites(ctx context.Context) (ListInvitesResponse, error)
	// CreateInvite нь и-мэйлийг super admin болох allow-list-д нэмнэ. Урьсан
	// хүн (и-мэйл) нь урилгад тэмдэглэгдэнэ. Давхардсан и-мэйл →
	// apperror.Conflict. АНХААР: энэ нь super admin эрхийг ШУУД олгодоггүй —
	// урьсан хүн /auth/superadmin/onboard шидтэнг (Google + eID + и-мэйл OTP +
	// TOTP) бүрэн давж байж л super admin болно.
	CreateInvite(ctx context.Context, req CreateInviteRequest) (CreateInviteResponse, error)
	// DeleteInvite нь урилгыг цуцална (хараахан ашиглаагүй бол дахин
	// бүртгүүлэх боломжгүй болно). Байхгүй бол apperror.NotFound.
	DeleteInvite(ctx context.Context, req DeleteInviteRequest) error

	// GetAccessMode нь платформын хандалтын горим (public|private)-ыг буцаана.
	GetAccessMode(ctx context.Context) (string, error)
	// SetAccessMode нь платформын хандалтын горимыг тохируулна. public: хэн ч
	// Government SSO-оор нэвтэрнэ; private: зөвхөн админаас урьдчилан бүртгэсэн
	// хэрэглэгч. Буруу утга → apperror.BadRequest.
	SetAccessMode(ctx context.Context, mode string) error
}

// AccessModeStore нь платформын хандалтын горимыг унших/бичих
// (postgres/platformsettings) хамгийн бага хараат байдал.
type AccessModeStore interface {
	GetAccessMode(ctx context.Context) (string, error)
	SetAccessMode(ctx context.Context, mode string) error
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
	AddAdminByRegisterRequest struct {
		// Register нь иргэний регистрийн дугаар (national_id).
		Register string
	}

	RevokeAdminRequest struct {
		// UserID нь эрхийг нь хасах хэрэглэгч.
		UserID string
		// ActorID нь үйлдлийг гүйцэтгэж буй super admin — өөрийгөө хасахаас
		// сэргийлэх (lockout guard) шалгалтад ашиглагдана.
		ActorID string
	}

	ListInvitesResponse struct {
		Invites []domain.SuperadminInvite
	}

	CreateInviteRequest struct {
		// Email нь урих хаяг (нормчлогдоно).
		Email string
		// ActorEmail нь урьж буй super admin-ий и-мэйл (invited_by).
		ActorEmail string
	}
	CreateInviteResponse struct {
		Invite domain.SuperadminInvite
	}

	DeleteInviteRequest struct {
		Email string
	}
)

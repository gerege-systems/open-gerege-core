// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package _interface нь repositories давхарга дахь домэйн бүрийн
// gateway хийсвэрлэлийг агуулна. Package-ийн нэр "_interface" байгаа
// шалтгаан нь "interface" нь Go-гийн нөөц түлхүүр үг бөгөөд шууд
// identifier болгон ашиглах боломжгүй; эхэнд тавьсан доогуур зураас
// нь үзэл баримтлалын утгыг өөрчлөхгүйгээр түүнийг хүчинтэй identifier
// болгон үлдээдэг.
//
// Тодорхой adapter-ууд (postgres/, ирээдүйн mongo/, г.м.) эдгээр
// interface-үүдийг хэрэгжүүлдэг бөгөөд энэ package-ийн ах дүүс болж
// оршино. Usecase давхарга нь зөвхөн энэ package-аас хамаардаг —
// хэзээ ч тодорхой adapter-аас хамаардаггүй — тиймээс хадгалалтын
// engine-г солих нь business код руу нэвчдэггүй.
package _interface

import (
	"context"

	"template/internal/business/domain"
)

// UserListFilter нь UserRepository.List() үр дүнг нарийсгана. Талбар
// бүр сонголттой; хоосон утга нь "энэ хэмжээст шүүлтгүй" гэсэн үг.
// Домэйн угтвартай (UserListFilter, ирээдүйн ProductListFilter) тул
// олон шүүлтийн төрөл энэ хуваалцсан package-д мөргөлдөөнгүйгээр
// зэрэгцэн оршиж чадна.
type UserListFilter struct {
	RoleID         int  // 0 = аль ч role
	ActiveOnly     bool // true = зөвхөн active=true хэрэглэгчид
	IncludeDeleted bool // false (өгөгдмөл) = WHERE deleted_at IS NULL
}

// UserRepository нь хэрэглэгчдийг ачаалах болон хадгалах gateway юм.
type UserRepository interface {
	// Store нь хэрэглэгчийг оруулж, хадгалагдсан мөрийг нэг round-trip-д
	// буцаадаг тул дуудагчдад дараагийн GetByEmail хэрэггүй (амжилтгүй
	// бол INSERT-г өнчрүүлэх байсан). Давхардсан username/email нь
	// apperror.Conflict болж гарна.
	Store(ctx context.Context, in *domain.User) (domain.User, error)
	// GetByEmail нь soft-delete хийгдсэн мөрүүдийг хасч, email-ээр
	// хэрэглэгчийг хайна. Тохирох мөр байхгүй үед apperror.NotFound-г
	// буцаана.
	GetByEmail(ctx context.Context, in *domain.User) (out domain.User, err error)
	// GetByID нь soft-delete хийгдсэн мөрүүдийг хасч, primary key-ээр
	// хэрэглэгчийг хайна. Тохирох мөр байхгүй үед apperror.NotFound-г
	// буцаана.
	GetByID(ctx context.Context, id string) (domain.User, error)
	// List нь filter-т тохирох хэрэглэгчдийг offset/limit-ээр хуудаслан
	// буцаана. Limit нь сервер талд хатуу хязгаарлагдсан тул буруу
	// ажиллаж буй дуудагч бүх хүснэгтийг татаж чадахгүй.
	List(ctx context.Context, filter UserListFilter, offset, limit int) ([]domain.User, error)
	// ChangeActiveUser нь active flag-г сольдог (OTP-verify урсгалд
	// ашиглагддаг) ба updated_at-г тэмдэглэнэ. Soft-delete хийгдсэн
	// мөрүүд дээр no-op.
	ChangeActiveUser(ctx context.Context, in *domain.User) (err error)
	// UpdatePassword нь bcrypt hash-г сольж, password_changed_at +
	// updated_at-г тэмдэглэнэ. Хэрэглэгч байхгүй/soft-delete хийгдсэн бол
	// apperror.NotFound-г буцаана.
	UpdatePassword(ctx context.Context, in *domain.User) error
	// SoftDelete нь deleted_at = NOW() гэж тогтоодог тул мөр нь
	// audit/сэргээх зорилгоор хүснэгтэд хэвээр үлддэг боловч өгөгдмөл
	// query-үүдтэй таарахаа болино. Мөр байхгүй эсвэл аль хэдийн устгагдсан
	// бол apperror.NotFound-г буцаана.
	SoftDelete(ctx context.Context, id string) error
	// UpdateRole нь хэрэглэгчийн role_id-г солино (admin удирдлага). Мөр
	// байхгүй/soft-delete хийгдсэн бол apperror.NotFound буцаана.
	UpdateRole(ctx context.Context, id string, roleID int) error
}

// RBACRepository нь динамик role-ууд болон тэдгээрийн эрхийг (role↔permission)
// хадгалах/уншихыг хариуцна. Permission каталог нь код дотор тодорхойлогддог тул
// энд зөвхөн уншина (ListPermissions нь seed хийгдсэн каталогийг буцаана).
type RBACRepository interface {
	ListRoles(ctx context.Context) ([]domain.Role, error)
	GetRole(ctx context.Context, id int) (domain.Role, error)
	CreateRole(ctx context.Context, in *domain.Role) (domain.Role, error)
	UpdateRole(ctx context.Context, in *domain.Role) (domain.Role, error)
	DeleteRole(ctx context.Context, id int) error
	// CountUsersWithRole нь тухайн role-д оноогдсон (soft-delete хийгдээгүй)
	// хэрэглэгчдийн тоог буцаана — ашиглагдаж буй role-ийг устгуулахгүйн тулд.
	CountUsersWithRole(ctx context.Context, roleID int) (int, error)
	ListPermissions(ctx context.Context) ([]domain.Permission, error)
	GetRolePermissions(ctx context.Context, roleID int) ([]string, error)
	SetRolePermissions(ctx context.Context, roleID int, keys []string) error
}

// AIRepository нь AI туслахын тохируулдаг prompt давхаргууд болон мэдлэгийн
// санг (knowledge base) хадгалах/уншихыг хариуцна. Suurь (base) дүрэм кодод
// хатуу бичигдсэн тул эндээс зөвхөн scope/instructions давхарга уншигдана.
type AIRepository interface {
	// ListPrompts нь тохируулдаг бүх prompt давхаргыг буцаана.
	ListPrompts(ctx context.Context) ([]domain.AIPrompt, error)
	// SetPrompt нь нэг давхаргын агуулгыг солино. Танигдаагүй key дээр
	// apperror.NotFound буцаана (зөвшөөрөгдсөн key-үүд migration-д seed
	// хийгддэг — INSERT хийдэггүй).
	SetPrompt(ctx context.Context, key, content string) error
	// SearchKnowledge нь мэдлэгийн сангаас query-д тохирох бичлэгүүдийг
	// буцаана (title/content ILIKE + tag тэнцэл). AI-ийн search_knowledge
	// tool үүгээр ажилладаг.
	SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.AIKnowledge, error)
}

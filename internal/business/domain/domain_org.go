// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// Гишүүнчлэлийн (membership) дүрийн танигчид. Эдгээр нь string тогтмол тул
// DB-ийн organization_memberships.role баганатай шууд таарна. Код доторх эрх
// олголтын шалгалтууд (CanManageMembers г.м.) нь эдгээр тогтмолыг ашиглах
// бөгөөд мөр шууд бичихгүй — нэг газар тодорхойлсноор бүх дуудагч дагана.
const (
	// OrgRoleOwner нь байгууллагыг үүсгэгч. Бүх эрхтэй: гишүүн нэмэх/хасах,
	// дүр солих, байгууллагыг удирдах. Үүсгэгч автоматаар owner болно.
	OrgRoleOwner = "owner"
	// OrgRoleAdmin нь гишүүдийг удирдах эрхтэй (owner-той ойролцоо) боловч
	// зөвхөн business дүрмээр owner-ээс ялгагдана (доорх CanManageMembers-г үз).
	OrgRoleAdmin = "admin"
	// OrgRoleMember нь энгийн гишүүн — байгууллагыг харна, удирдлагын эрхгүй.
	OrgRoleMember = "member"
)

// Organization нь байгууллагын domain entity юм. RegNo нь улсын бүртгэлийн
// дугаар (case-insensitive давтагдашгүй); NameLatin нь латин (галиглсан) нэр.
// CreatedBy нь үүсгэгч хэрэглэгчийн UUID — тэр автоматаар owner гишүүн болно.
type Organization struct {
	ID        string
	RegNo     string
	Name      string
	NameLatin string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt *time.Time
}

// OrganizationMembership нь хэрэглэгч ↔ байгууллагын холбоос бөгөөд тухайн
// хэрэглэгчийн уг байгууллага доторх дүрийг (Role) агуулна. (OrgID, UserID)
// хосоор давтагдашгүй (composite primary key).
type OrganizationMembership struct {
	OrgID     string
	UserID    string
	Role      string
	CreatedAt time.Time
}

// IsValidOrgRole нь өгөгдсөн дүр нь танигдсан гишүүнчлэлийн дүр эсэхийг
// мэдээлнэ — transport давхарга оролтыг баталгаажуулахад ашиглана.
func IsValidOrgRole(role string) bool {
	switch role {
	case OrgRoleOwner, OrgRoleAdmin, OrgRoleMember:
		return true
	default:
		return false
	}
}

// CanManageMembers нь тухайн дүр нь гишүүн нэмэх/хасах/дүр солих эрхтэй эсэхийг
// мэдээлнэ. owner болон admin удирдаж чадна; энгийн member чадахгүй. Дүрэм нэг
// газар байрлахын тулд (дуудах газруудад нүцгэн харьцуулалт хийхгүй) method
// болгосон.
func CanManageMembers(role string) bool {
	return role == OrgRoleOwner || role == OrgRoleAdmin
}

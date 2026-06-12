// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package records

import "time"

// Organizations нь organizations хүснэгтийн pgx record юм. `db` tag-ууд нь
// snake_case schema руу буудаг бөгөөд pgx.RowToStructByName тэдгээрээр
// баганануудыг талбаруудтай тааруулдаг. Nullable баганануудыг *time.Time-ээр
// илэрхийлсэн тул NULL нь nil pointer болж буудаг.
//
// GORM-ийн автомат soft-delete байхгүй тул repository давхарга нь DeletedAt-г
// шүүхдээ query бүрт `deleted_at IS NULL`-г ИЛ-ээр нэмэх ёстой.
type Organizations struct {
	Id        string     `db:"id"`
	RegNo     string     `db:"reg_no"`
	Name      string     `db:"name"`
	NameLatin string     `db:"name_latin"`
	CreatedBy string     `db:"created_by"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// OrgColumns нь SELECT/RETURNING-д ашиглах баганануудын жагсаалт — query-уудыг
// тогтвортой байлгахаар нэг эх сурвалжид төвлөрүүлэв.
const OrgColumns = "id, reg_no, name, name_latin, created_by, created_at, updated_at, deleted_at"

// OrganizationMemberships нь organization_memberships хүснэгтийн pgx record.
type OrganizationMemberships struct {
	OrgID     string    `db:"org_id"`
	UserID    string    `db:"user_id"`
	Role      string    `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

// OrgMembershipColumns нь membership-ийн баганануудын жагсаалт.
const OrgMembershipColumns = "org_id, user_id, role, created_at"

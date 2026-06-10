// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package records

import (
	"time"
)

// Users нь users хүснэгтийн pgx record юм. `db` tag-ууд нь snake_case
// schema руу буудаг бөгөөд pgx.RowToStructByName тэдгээрээр баганануудыг
// талбаруудтай тааруулдаг. Нэмж болохуйц (nullable) баганануудыг
// *time.Time-ээр илэрхийлсэн тул NULL нь nil pointer болж буудаг.
//
// GORM-ийн автомат soft-delete (gorm.DeletedAt) байхгүй болсон тул
// repository давхарга нь DeletedAt-г шүүхдээ query бүрт `deleted_at IS
// NULL`-г ИЛ-ээр нэмэх ёстой.
type Users struct {
	Id                string     `db:"id"`
	Username          string     `db:"username"`
	Email             string     `db:"email"`
	Password          string     `db:"password"`
	Active            bool       `db:"active"`
	RoleId            int        `db:"role_id"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         *time.Time `db:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at"`
	PasswordChangedAt *time.Time `db:"password_changed_at"`
}

// UserColumns нь SELECT/RETURNING-д ашиглах баганануудын жагсаалт —
// pgx.RowToStructByName нь нэрээр тааруулдаг тул query-уудыг тогтвортой
// байлгахаар нэг эх сурвалжид төвлөрүүлэв.
const UserColumns = "id, username, email, password, active, role_id, created_at, updated_at, deleted_at, password_changed_at"

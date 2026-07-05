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
	Id          string `db:"id"`
	Username    string `db:"username"`
	FirstName   string `db:"first_name"`
	LastName    string `db:"last_name"`
	FirstNameEn string `db:"first_name_en"`
	LastNameEn  string `db:"last_name_en"`
	// Email/Password нь eID хэрэглэгчдэд NULL байж болох тул (migration 12-д
	// NOT NULL-ийг хассан) *string — NULL нь nil pointer болж буудаг.
	Email    *string `db:"email"`
	Password *string `db:"password"`
	Active   bool    `db:"active"`
	RoleId   int     `db:"role_id"`
	// eID identity баганууд (migration 12) — нууц үгээр бүртгүүлсэн
	// хэрэглэгчдэд NULL тул *string.
	NationalID          *string    `db:"national_id"`
	CivilID             *string    `db:"civil_id"`
	KYCLevel            *string    `db:"kyc_level"`
	DocumentNumber      *string    `db:"document_number"`
	CertSerial          *string    `db:"cert_serial"`
	CertNotBefore       *time.Time `db:"cert_not_before"`
	CertNotAfter        *time.Time `db:"cert_not_after"`
	CertIssuer          *string    `db:"cert_issuer"`
	CertKeyType         *string    `db:"cert_key_type"`
	GoogleSub           *string    `db:"google_sub"`
	GoogleEmail         *string    `db:"google_email"`
	GoogleEmailVerified bool       `db:"google_email_verified"`
	GoogleName          *string    `db:"google_name"`
	GooglePicture       *string    `db:"google_picture"`
	GoogleLinkedAt      *time.Time `db:"google_linked_at"`
	CreatedAt           time.Time  `db:"created_at"`
	UpdatedAt           *time.Time `db:"updated_at"`
	DeletedAt           *time.Time `db:"deleted_at"`
	PasswordChangedAt   *time.Time `db:"password_changed_at"`
}

// UserColumns нь SELECT/RETURNING-д ашиглах баганануудын жагсаалт —
// pgx.RowToStructByName нь нэрээр тааруулдаг тул query-уудыг тогтвортой
// байлгахаар нэг эх сурвалжид төвлөрүүлэв.
const UserColumns = "id, username, first_name, last_name, first_name_en, last_name_en, email, password, active, role_id, national_id, civil_id, kyc_level, document_number, cert_serial, cert_not_before, cert_not_after, cert_issuer, cert_key_type, google_sub, google_email, google_email_verified, google_name, google_picture, google_linked_at, created_at, updated_at, deleted_at, password_changed_at"

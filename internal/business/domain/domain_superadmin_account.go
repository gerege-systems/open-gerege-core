// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// SuperadminAccount нь super admin-ы бүртгэлийн satellite мөр (superadmin_accounts).
// Super admin нь users-д role_id=1 мөр хэвээр (google_sub-аар түлхүүрлэсэн, civil_id
// users-д NULL) боловч eID баталгаа (civil_id/national_id), MFA (TOTP secret), email
// баталгаажуулалт, урилга/onboarding metadata нь энд тусад нь хадгалагдана.
type SuperadminAccount struct {
	UserID        string
	CivilID       string
	NationalID    string
	EmailVerified bool
	MFAEnabled    bool
	// TOTPSecret нь AES-GCM ciphertext (usecase давхаргад шифрлэгдсэн) — DB-д ил
	// текст хэзээ ч хадгалагдахгүй.
	TOTPSecret  string
	InvitedBy   string
	OnboardedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

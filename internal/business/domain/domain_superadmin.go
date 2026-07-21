// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import (
	"strings"
	"time"
)

// RecoveryCode нь хэрэглэгчийн 2FA нөөц код (user_recovery_codes) юм. Энгийн
// текст код нь ЗӨВХӨН үүсгэх агшинд оршиж, хэрэглэгчид нэг удаа харагдана;
// DB-д зөвхөн CodeHash (SHA-256) хадгалагдана. UsedAt тэмдэглэгдсэн код
// дахин хэрэглэгдэхгүй (нэг удаагийн).
type RecoveryCode struct {
	ID        string
	UserID    string
	CodeHash  string
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Used нь кодыг аль хэдийн хэрэглэсэн эсэхийг мэдээлнэ.
func (c RecoveryCode) Used() bool { return c.UsedAt != nil }

// SuperadminInvite нь superadmin болох эрхтэй и-мэйлийн урилга
// (superadmin_invites) юм — onboarding нь урилгагүй и-мэйлээр эхэлж чадахгүй
// (allow-list). AcceptedAt тэмдэглэгдсэн урилга дахин хэрэглэгдэхгүй.
type SuperadminInvite struct {
	Email      string
	InvitedBy  string
	CreatedAt  time.Time
	AcceptedAt *time.Time
}

// Accepted нь урилгыг аль хэдийн ашигласан эсэхийг мэдээлнэ.
func (i SuperadminInvite) Accepted() bool { return i.AcceptedAt != nil }

// NormalizeInviteEmail нь урилгын и-мэйлийг каноник хэлбэрт (жижиг үсэг,
// зайгүй) буулгана — superadmin_invites.email нь primary key тул хадгалах ба
// хайх талдаа ижил нормчлолыг хэрэглэнэ.
func NormalizeInviteEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

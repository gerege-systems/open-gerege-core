// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package _interface

import (
	"context"

	"template/internal/business/domain"
)

// SSOTokenRepository нь иргэний dgov-SSO OAuth токенуудыг (шифрлэсэн) хадгалах
// gateway. SSO eID proxy-г дуудахад ашиглагдана. Adapter нь токенуудыг AES-GCM-
// ээр шифрлэж/тайлж, зөвхөн шифр текстийг DB-д байлгана.
type SSOTokenRepository interface {
	// Upsert нь тухайн хэрэглэгчийн токенуудыг хадгална (байвал дарж бичнэ).
	Upsert(ctx context.Context, userID string, tok domain.SSOToken) error
	// Get нь хадгалагдсан токенуудыг (тайлсан) буцаана; байхгүй бол
	// domain.ErrSSOTokenNotFound.
	Get(ctx context.Context, userID string) (domain.SSOToken, error)
}

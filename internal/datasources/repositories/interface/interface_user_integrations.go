// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package _interface

import (
	"context"

	"template/internal/business/domain"
)

// UserIntegrationRepository нь хэрэглэгчийн OAuth интеграцийн токены gateway.
// Бүх method хэрэглэгч-тус-бүрийн RLS-ээр хамгаалагдсан (withRLS).
type UserIntegrationRepository interface {
	// Upsert нь (user_id, provider)-оор давхцвал токеныг шинэчилнэ, эс бөгөөс
	// шинээр оруулна; үр дүнгийн мөрийг буцаана.
	Upsert(ctx context.Context, in *domain.UserIntegration) (domain.UserIntegration, error)
	// ListByUser нь тухайн хэрэглэгчийн бүх холболтыг буцаана.
	ListByUser(ctx context.Context, userID string) ([]domain.UserIntegration, error)
	// DeleteByUserAndProvider нь нэг холболтыг устгана (idempotent).
	DeleteByUserAndProvider(ctx context.Context, userID, provider string) error
}

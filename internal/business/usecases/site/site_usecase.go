// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package site нь сайтын нийтийн харагдацын default-ыг (accent · font · style ·
// theme) удирдана. GetAppearance нь landing зэрэг нийтийн хуудсанд (auth-гүй)
// уншигддаг тул богино TTL кэштэй; SetAppearance нь админ өөрчлөлт хийхэд
// кэшийг шууд хүчингүй болгоно.
package site

import (
	"context"

	"template/internal/business/domain"
)

type Usecase interface {
	// GetAppearance нь харагдацын default-ыг буцаана.
	GetAppearance(ctx context.Context) (domain.SiteAppearance, error)
	// SetAppearance нь харагдацын default-ыг баталгаажуулаад шинэчилнэ.
	SetAppearance(ctx context.Context, a domain.SiteAppearance) error
}

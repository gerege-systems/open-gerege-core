// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package site

import (
	"context"
	"sync"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

// appearanceCacheTTL нь GetAppearance-ийн уншилтын кэшийн нас. Landing бүр
// уншдаг тул богино кэш DB-г хамгаална; админ өөрчлөлт кэшийг шууд цэвэрлэнэ.
const appearanceCacheTTL = time.Minute

type usecase struct {
	repo  repointerface.SiteRepository
	cache struct {
		mu       sync.RWMutex
		value    domain.SiteAppearance
		loadedAt time.Time
		valid    bool
	}
}

func NewUsecase(repo repointerface.SiteRepository) Usecase {
	return &usecase{repo: repo}
}

func (uc *usecase) GetAppearance(ctx context.Context) (domain.SiteAppearance, error) {
	uc.cache.mu.RLock()
	if uc.cache.valid && time.Since(uc.cache.loadedAt) < appearanceCacheTTL {
		v := uc.cache.value
		uc.cache.mu.RUnlock()
		return v, nil
	}
	uc.cache.mu.RUnlock()

	a, err := uc.repo.GetAppearance(ctx)
	if err != nil {
		return domain.SiteAppearance{}, apperror.InternalCause(err)
	}

	uc.cache.mu.Lock()
	uc.cache.value = a
	uc.cache.loadedAt = time.Now()
	uc.cache.valid = true
	uc.cache.mu.Unlock()
	return a, nil
}

func (uc *usecase) SetAppearance(ctx context.Context, a domain.SiteAppearance) error {
	if !domain.ValidSiteAccent(a.Accent) {
		return apperror.BadRequest("invalid accent (preset name or #rrggbb hex)")
	}
	if !domain.SiteFonts[a.Font] {
		return apperror.BadRequest("invalid font")
	}
	if !domain.SiteStyles[a.Style] {
		return apperror.BadRequest("invalid style")
	}
	if !domain.SiteThemes[a.Theme] {
		return apperror.BadRequest("invalid theme")
	}
	if err := uc.repo.SetAppearance(ctx, a); err != nil {
		return err
	}
	// Кэшийг хүчингүй болгоно — дараагийн уншилт DB-ээс шинэчилнэ.
	uc.cache.mu.Lock()
	uc.cache.valid = false
	uc.cache.mu.Unlock()
	return nil
}

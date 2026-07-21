// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package theme

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

const activeCacheTTL = time.Minute

type usecase struct {
	repo  repointerface.ThemeRepository
	cache struct {
		mu       sync.RWMutex
		value    domain.Theme
		loadedAt time.Time
		valid    bool
	}
}

func NewUsecase(repo repointerface.ThemeRepository) Usecase {
	return &usecase{repo: repo}
}

func (uc *usecase) invalidate() {
	uc.cache.mu.Lock()
	uc.cache.valid = false
	uc.cache.mu.Unlock()
}

// validate нь нэр болон config JSONB-ийг шалгана.
func (uc *usecase) validate(name string, config json.RawMessage) error {
	if strings.TrimSpace(name) == "" {
		return apperror.BadRequest("theme name is required")
	}
	if len(name) > 80 {
		return apperror.BadRequest("theme name too long (max 80)")
	}
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if err := domain.ValidateThemeConfig(config); err != nil {
		return apperror.BadRequest(err.Error())
	}
	return nil
}

func (uc *usecase) List(ctx context.Context) ([]domain.Theme, error) {
	list, err := uc.repo.ListThemes(ctx)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	return list, nil
}

func (uc *usecase) Get(ctx context.Context, id string) (domain.Theme, error) {
	return uc.repo.GetTheme(ctx, id)
}

func (uc *usecase) GetActive(ctx context.Context) (domain.Theme, error) {
	uc.cache.mu.RLock()
	if uc.cache.valid && time.Since(uc.cache.loadedAt) < activeCacheTTL {
		v := uc.cache.value
		uc.cache.mu.RUnlock()
		return v, nil
	}
	uc.cache.mu.RUnlock()

	t, err := uc.repo.GetActiveTheme(ctx)
	if err != nil {
		return domain.Theme{}, err
	}
	uc.cache.mu.Lock()
	uc.cache.value = t
	uc.cache.loadedAt = time.Now()
	uc.cache.valid = true
	uc.cache.mu.Unlock()
	return t, nil
}

func (uc *usecase) Create(ctx context.Context, name string, config json.RawMessage) (domain.Theme, error) {
	if err := uc.validate(name, config); err != nil {
		return domain.Theme{}, err
	}
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	t, err := uc.repo.CreateTheme(ctx, strings.TrimSpace(name), config)
	if err != nil {
		return domain.Theme{}, apperror.InternalCause(err)
	}
	return t, nil
}

func (uc *usecase) Update(ctx context.Context, id, name string, config json.RawMessage) error {
	if err := uc.validate(name, config); err != nil {
		return err
	}
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if err := uc.repo.UpdateTheme(ctx, id, strings.TrimSpace(name), config); err != nil {
		return err
	}
	uc.invalidate() // идэвхтэй theme засагдсан байж болзошгүй
	return nil
}

func (uc *usecase) Delete(ctx context.Context, id string) error {
	// Идэвхтэй theme-ийг устгахыг хориглоно — landing-ийг эх сурвалжгүй болгоно.
	t, err := uc.repo.GetTheme(ctx, id)
	if err != nil {
		return err
	}
	if t.IsActive {
		return apperror.BadRequest("cannot delete the active theme; activate another first")
	}
	if err := uc.repo.DeleteTheme(ctx, id); err != nil {
		return err
	}
	uc.invalidate()
	return nil
}

func (uc *usecase) SetActive(ctx context.Context, id string) error {
	if err := uc.repo.SetActive(ctx, id); err != nil {
		return err
	}
	uc.invalidate()
	return nil
}

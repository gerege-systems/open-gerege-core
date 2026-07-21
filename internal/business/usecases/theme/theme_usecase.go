// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package theme нь landing-ийн нэрлэсэн загваруудыг (themes) удирдана — CRUD,
// идэвхтэй (default) сонголт. Идэвхтэй theme-ийг нэвтрээгүй зочны landing SSR-д
// уншдаг тул богино TTL кэштэй; аливаа бичих үйлдэл кэшийг хүчингүй болгоно.
package theme

import (
	"context"
	"encoding/json"

	"template/internal/business/domain"
)

type Usecase interface {
	List(ctx context.Context) ([]domain.Theme, error)
	Get(ctx context.Context, id string) (domain.Theme, error)
	// GetActive нь идэвхтэй theme-ийг буцаана (landing SSR).
	GetActive(ctx context.Context) (domain.Theme, error)
	Create(ctx context.Context, name string, config json.RawMessage) (domain.Theme, error)
	Update(ctx context.Context, id, name string, config json.RawMessage) error
	Delete(ctx context.Context, id string) error
	// SetActive нь тухайн theme-ийг идэвхтэй (default) болгоно.
	SetActive(ctx context.Context, id string) error
}

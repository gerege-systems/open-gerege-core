// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package platform нь модулийн lifecycle-ийн usecase: модулиудын төлвийг
// жагсаах, business модулийг restart-гүйгээр асаах/унтраах. Дүрмүүд нь
// kernel/module Registry-д (core хамгаалалт, хамаарлын дараалал) — энэ
// давхарга нь түүнийг DB persistence + audit-тэй холбоно.
package platform

import (
	"context"
	"fmt"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	audituc "github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
)

// Store нь модулийн төлвийн persistence (platform_modules хүснэгт).
type Store interface {
	ListDisabled(ctx context.Context) ([]string, error)
	SetEnabled(ctx context.Context, id string, enabled bool) error
}

// Usecase — модулийн lifecycle-ийн үйлдлүүд.
type Usecase interface {
	// List нь бүх модулийн одоогийн төлвийг буцаана.
	List(ctx context.Context) []module.Status
	// SetEnabled нь business модулийг асааж/унтраана: registry дүрэм
	// шалгана → DB-д хадгална → audit бичнэ. Registry-ийн алдаа (core
	// модуль, хамаарал г.м.) хэрэглэгчид ил буцна.
	SetEnabled(ctx context.Context, id string, enabled bool) error
}

type usecase struct {
	registry *module.Registry
	store    Store
	audit    audituc.Usecase
}

// NewUsecase нь platform usecase үүсгэнэ. store nil бол persistence-гүй
// (зөвхөн in-memory) ажиллана — тест болон нимгэн орчинд.
func NewUsecase(registry *module.Registry, store Store, audit audituc.Usecase) Usecase {
	return &usecase{registry: registry, store: store, audit: audit}
}

func (u *usecase) List(_ context.Context) []module.Status {
	return u.registry.List()
}

func (u *usecase) SetEnabled(ctx context.Context, id string, enabled bool) error {
	// 1) Registry — бизнес дүрмийн цорын ганц эх сурвалж. Дүрмийн зөрчил
	// (core модуль, хамаарал, бүртгэлгүй ID) нь хэрэглэгчийн алдаа тул
	// BadRequest болж ил буцна.
	var err error
	if enabled {
		err = u.registry.Enable(id)
	} else {
		err = u.registry.Disable(id)
	}
	if err != nil {
		return apperror.BadRequest(err.Error())
	}

	// 2) Persistence — restart-ын дараа төлөв хадгалагдана.
	if u.store != nil {
		if err := u.store.SetEnabled(ctx, id, enabled); err != nil {
			return fmt.Errorf("модулийн төлөв хадгалах: %w", err)
		}
	}

	// 3) Audit — hash-chain бүртгэлд админы үйлдэл үлдэнэ. Бусад flow-той
	// ижил best-effort: audit унасан ч үйлдэл амжилттай хэвээр (эс бөгөөс
	// audit-ийн түр саатал модулийн удирдлагыг гацаана).
	if u.audit != nil {
		action := "module.disable"
		if enabled {
			action = "module.enable"
		}
		if err := u.audit.RecordEvent(ctx, action, "platform", id, map[string]any{
			"enabled": enabled,
		}); err != nil {
			logger.Warn("модулийн audit бичигдсэнгүй", logger.Fields{
				"module": id, "error": err.Error(),
			})
		}
	}
	return nil
}

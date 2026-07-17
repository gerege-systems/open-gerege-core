// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package landing нь нүүр хуудасны (landing + auth бүрхүүл) ажиллаж байх үед
// тохируулдаг харагдацыг (өнгө, фонт, хэмжээ, текст mn/en, товч/цэс) удирдана.
// Схемийг frontend эзэмшдэг тул usecase нь тохиргоог ихэвчлэн opaque JSON-оор
// авч явна; зөвхөн хадгалахын өмнө (a) хүчинтэй JSON объект, (b) хэмжээний
// хязгаар, (c) rawCss ариутгал гэсэн гурван шалгалт хийнэ.
//
// Уншилт нь НИЙТИЙН (нэвтрээгүй зочид нүүр татна) тул GetConfig хэзээ ч алдаа
// буцаадаггүй — DB унасан ч богино кэш эсвэл хоосон объект руу fail-open хийж,
// frontend-ийн өгөгдмөл (DEFAULT_LANDING_CONFIG) дүүргэнэ. ai_prompts-ийн
// prompt кэштэй адил TTL-тэй кэш; SetConfig кэшийг шууд хүчингүй болгоно.
package landing

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"template/internal/apperror"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/logger"
)

// configCacheTTL — тохиргоог хүсэлт бүрд DB-ээс уншихгүйн тулд богино кэш;
// SetConfig нэн даруй хүчингүй болгодог тул нэг instance дээр өөрчлөлт шууд,
// бусад instance дээр TTL-ээр үйлчилнэ.
const configCacheTTL = time.Minute

// maxConfigBytes — ирж буй тохиргооны баримтын хамгийн их хэмжээ (JSONB
// хавагнахаас сэргийлнэ). 64 KiB нь текст + theme + цэсэнд элбэг.
const maxConfigBytes = 64 * 1024

// maxRawCSSBytes — advanced CSS override-ийн таазт хэмжээ.
const maxRawCSSBytes = 20 * 1024

// emptyConfig — DB огт байхгүй үеийн эцсийн fallback (frontend өгөгдмөлөөр
// дүүргэнэ).
var emptyConfig = json.RawMessage(`{}`)

type Usecase interface {
	// GetConfig нь одоогийн нүүрний тохиргооны JSON баримтыг буцаана. НИЙТИЙН
	// уншилт тул алдаа буцаадаггүй — DB алдаа үед кэш/хоосон объект руу
	// fail-open хийнэ.
	GetConfig(ctx context.Context) json.RawMessage
	// SetConfig нь тохиргоог бүхэлд нь солино: хүчинтэй JSON объект + хэмжээ
	// шалгаж, rawCss-ийг ариутгаад хадгалж, кэшийг хүчингүй болгоно.
	SetConfig(ctx context.Context, config json.RawMessage) error
}

type usecase struct {
	repo  repointerface.LandingConfigRepository
	cache configCache
}

type configCache struct {
	mu        sync.Mutex
	fetchedAt time.Time
	value     json.RawMessage
}

func NewUsecase(repo repointerface.LandingConfigRepository) Usecase {
	return &usecase{repo: repo}
}

func (uc *usecase) GetConfig(ctx context.Context) json.RawMessage {
	if uc.repo == nil {
		return emptyConfig
	}
	uc.cache.mu.Lock()
	defer uc.cache.mu.Unlock()
	if uc.cache.value != nil && time.Since(uc.cache.fetchedAt) < configCacheTTL {
		return uc.cache.value
	}
	cfg, err := uc.repo.GetConfig(ctx)
	if err != nil {
		logger.ErrorWithContext(ctx, "landing: failed to load config (using fallback)", logger.Fields{
			"error": err.Error(),
		})
		if uc.cache.value != nil {
			return uc.cache.value // хуучирсан кэшээр үргэлжилнэ
		}
		return emptyConfig
	}
	uc.cache.value = cfg.Config
	uc.cache.fetchedAt = time.Now()
	return uc.cache.value
}

func (uc *usecase) SetConfig(ctx context.Context, config json.RawMessage) error {
	if uc.repo == nil {
		return apperror.Internal("landing config storage not configured")
	}
	if len(config) > maxConfigBytes {
		return apperror.BadRequest("landing config too large")
	}
	// Хүчинтэй JSON ОБЪЕКТ эсэхийг шалгана (массив/мөр/тоо биш).
	var doc map[string]any
	if err := json.Unmarshal(config, &doc); err != nil {
		return apperror.BadRequest("landing config must be a JSON object")
	}
	// rawCss талбарыг ариутгаж буцаан тавина (task: XSS/breakout-аас сэргийлэх;
	// зохион байгуулалт эвдэх нь админы сонголт — зөвшөөрөгдсөн).
	if raw, ok := doc["rawCss"].(string); ok {
		doc["rawCss"] = sanitizeRawCSS(raw)
	}
	clean, err := json.Marshal(doc)
	if err != nil {
		return apperror.InternalCause(err)
	}
	if err := uc.repo.SetConfig(ctx, clean); err != nil {
		return err
	}
	// Кэшийг хүчингүй болгож өөрчлөлтийг нэн даруй үйлчилнэ.
	uc.cache.mu.Lock()
	uc.cache.value = nil
	uc.cache.mu.Unlock()
	return nil
}

// sanitizeRawCSS нь админы advanced CSS override-оос <style> блокоос гарах
// эсвэл скрипт тарих векторуудыг устгана. CSS нь .signin-shell дотор
// байрлуулагдах тул гол эрсдэл нь </style> таслалт; мөн @import (гадаад
// нөөц), expression()/javascript: (хуучин browser), script/comment таслал.
func sanitizeRawCSS(css string) string {
	css = strings.ReplaceAll(css, "\x00", "")
	for _, tok := range []string{"</style", "<!--", "-->", "</script", "<script", "@import", "expression(", "javascript:"} {
		css = stripFold(css, tok)
	}
	if len(css) > maxRawCSSBytes {
		css = css[:maxRawCSSBytes]
	}
	return css
}

// stripFold нь sub-ийн бүх тохиолдлыг (том/жижиг үсэг үл харгалзан) s-ээс хасна.
func stripFold(s, sub string) string {
	if sub == "" {
		return s
	}
	ls, lsub := strings.ToLower(s), strings.ToLower(sub)
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(ls[i:], lsub)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+j])
		i += j + len(sub)
	}
	return b.String()
}

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package language

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gerege-systems/public-gerege-core/core/apperror"
	"github.com/gerege-systems/public-gerege-core/core/business/domain"
	repointerface "github.com/gerege-systems/public-gerege-core/core/datasources/repositories/interface"
	"github.com/gerege-systems/public-gerege-core/pkg/gemini"
)

const (
	// Идэвхтэй хэл ба dictionary-г хуудас бүрийн ачаалалт уншдаг тул богино
	// TTL кэштэй (themes-ийн идэвхтэй theme-тэй ижил зарчим).
	listCacheTTL = time.Minute
	dictCacheTTL = time.Minute

	// Нэг Gemini дуудалтад өгөх түлхүүрийн тоо. Хэт том багц нь гаралтын
	// токены хязгаарт мөргөж таслагдана; хэт жижиг нь дуудалтын тоог өсгөнө.
	translateBatchSize = 60

	// Хариунд буцаах анхааруулгын дээд тоо — админд ойлгомжтой байх хэрээр.
	maxWarnings = 20
)

type dictEntry struct {
	value    map[string]string
	loadedAt time.Time
}

type usecase struct {
	repo       repointerface.LanguageRepository
	translator Translator

	mu sync.RWMutex
	// Идэвхтэй хэлний кэш.
	enabled      []domain.Language
	enabledAt    time.Time
	enabledValid bool
	// Хэл тус бүрийн dictionary кэш.
	dicts map[string]dictEntry
}

// NewUsecase нь хэлний usecase үүсгэнэ. translator нь nil байж болно — тэр
// тохиолдолд автомат орчуулга тохируулагдаагүй гэж хариулна.
func NewUsecase(repo repointerface.LanguageRepository, translator Translator) Usecase {
	return &usecase{repo: repo, translator: translator, dicts: make(map[string]dictEntry)}
}

// invalidate нь бүх кэшийг хүчингүй болгоно. Хэл идэвхжих/унтрахад аль ч
// dictionary-ийн хүчинтэй байдал өөрчлөгдөж болзошгүй тул бүгдийг цэвэрлэнэ.
func (uc *usecase) invalidate() {
	uc.mu.Lock()
	uc.enabledValid = false
	uc.dicts = make(map[string]dictEntry)
	uc.mu.Unlock()
}

func (uc *usecase) List(ctx context.Context) ([]domain.Language, error) {
	list, err := uc.repo.ListLanguages(ctx)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	return list, nil
}

func (uc *usecase) ListEnabled(ctx context.Context) ([]domain.Language, error) {
	uc.mu.RLock()
	if uc.enabledValid && time.Since(uc.enabledAt) < listCacheTTL {
		cached := uc.enabled
		uc.mu.RUnlock()
		return cached, nil
	}
	uc.mu.RUnlock()

	list, err := uc.repo.ListEnabledLanguages(ctx)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	uc.mu.Lock()
	uc.enabled = list
	uc.enabledAt = time.Now()
	uc.enabledValid = true
	uc.mu.Unlock()
	return list, nil
}

func (uc *usecase) Get(ctx context.Context, code string) (domain.Language, error) {
	if err := domain.ValidateLanguageCode(code); err != nil {
		return domain.Language{}, apperror.BadRequest(err.Error())
	}
	return uc.repo.GetLanguage(ctx, code)
}

// Create нь шинэ хэлийг ҮРГЭЛЖ унтраалттай үүсгэнэ — орчуулга нь хоосон байхад
// идэвхжвэл хэрэглэгч бүтэн хуудас түлхүүрийн нэр харах болно. Админ орчуулгыг
// бөглөсний дараа Update-ээр идэвхжүүлнэ.
func (uc *usecase) Create(ctx context.Context, code, label, locale string) (domain.Language, error) {
	code = strings.TrimSpace(code)
	label = strings.TrimSpace(label)
	locale = strings.TrimSpace(locale)
	if locale == "" {
		locale = code
	}
	if err := domain.ValidateLanguage(code, label, locale); err != nil {
		return domain.Language{}, apperror.BadRequest(err.Error())
	}

	created, err := uc.repo.CreateLanguage(ctx, domain.Language{
		Code:      code,
		Label:     label,
		Locale:    locale,
		Enabled:   false,
		SortOrder: 100,
	})
	if err != nil {
		return domain.Language{}, err
	}
	uc.invalidate()
	return created, nil
}

func (uc *usecase) Update(ctx context.Context, code string, patch domain.LanguagePatch) error {
	if err := domain.ValidateLanguageCode(code); err != nil {
		return apperror.BadRequest(err.Error())
	}
	if patch.Label != nil {
		trimmed := strings.TrimSpace(*patch.Label)
		if trimmed == "" {
			return apperror.BadRequest("language label is required")
		}
		if len(trimmed) > 80 {
			return apperror.BadRequest("language label too long (max 80)")
		}
		patch.Label = &trimmed
	}
	if patch.Locale != nil {
		trimmed := strings.TrimSpace(*patch.Locale)
		if err := domain.ValidateLanguage(code, "x", trimmed); err != nil {
			return apperror.BadRequest(err.Error())
		}
		patch.Locale = &trimmed
	}

	if err := uc.repo.UpdateLanguage(ctx, code, patch); err != nil {
		return err
	}
	uc.invalidate()
	return nil
}

func (uc *usecase) Delete(ctx context.Context, code string) error {
	lang, err := uc.Get(ctx, code)
	if err != nil {
		return err
	}
	// Багцлагдсан хэлийг устгаж болохгүй: түүний утга аппын кодод байдаг тул
	// мөрийг устгах нь зөвхөн будлиан үүсгэнэ (апп хэвээр орчуулгыг харуулна).
	if lang.IsBuiltin {
		return apperror.BadRequest("built-in language cannot be deleted; disable it instead")
	}
	if err := uc.repo.DeleteLanguage(ctx, code); err != nil {
		return err
	}
	uc.invalidate()
	return nil
}

func (uc *usecase) Dictionary(ctx context.Context, code string) (map[string]string, error) {
	if err := domain.ValidateLanguageCode(code); err != nil {
		return nil, apperror.BadRequest(err.Error())
	}

	uc.mu.RLock()
	if entry, ok := uc.dicts[code]; ok && time.Since(entry.loadedAt) < dictCacheTTL {
		cached := entry.value
		uc.mu.RUnlock()
		return cached, nil
	}
	uc.mu.RUnlock()

	dict, err := uc.repo.GetDictionary(ctx, code)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	uc.mu.Lock()
	uc.dicts[code] = dictEntry{value: dict, loadedAt: time.Now()}
	uc.mu.Unlock()
	return dict, nil
}

func (uc *usecase) PutTranslations(ctx context.Context, code string, entries map[string]string) error {
	if _, err := uc.Get(ctx, code); err != nil {
		return err
	}
	if err := domain.ValidateDictionary(entries); err != nil {
		return apperror.BadRequest(err.Error())
	}
	if err := uc.repo.UpsertTranslations(ctx, code, entries, domain.TranslationSourceManual); err != nil {
		return apperror.InternalCause(err)
	}
	uc.invalidate()
	return nil
}

func (uc *usecase) AutoTranslate(ctx context.Context, req AutoTranslateRequest) (AutoTranslateResult, error) {
	if uc.translator == nil {
		return AutoTranslateResult{}, apperror.BadRequest("AI translation is not configured")
	}
	lang, err := uc.Get(ctx, req.Code)
	if err != nil {
		return AutoTranslateResult{}, err
	}
	if err := domain.ValidateDictionary(req.Base); err != nil {
		return AutoTranslateResult{}, apperror.BadRequest(err.Error())
	}
	baseLang := strings.TrimSpace(req.BaseLang)
	if baseLang == "" {
		baseLang = "mn"
	}
	if baseLang == lang.Code {
		return AutoTranslateResult{}, apperror.BadRequest("source and target language are the same")
	}

	existing, err := uc.repo.GetDictionary(ctx, lang.Code)
	if err != nil {
		return AutoTranslateResult{}, apperror.InternalCause(err)
	}
	sources, err := uc.repo.GetTranslationSources(ctx, lang.Code)
	if err != nil {
		return AutoTranslateResult{}, apperror.InternalCause(err)
	}

	pending, result := selectPending(req.Base, existing, sources, req.Overwrite)
	if len(pending) == 0 {
		return result, nil
	}

	translated := make(map[string]string, len(pending))
	for _, batch := range chunkKeys(pending, translateBatchSize) {
		items := make(map[string]string, len(batch))
		for _, key := range batch {
			items[key] = req.Base[key]
		}

		out, err := uc.translator.TranslateBatch(ctx, TranslateBatchRequest{
			SourceLang:  baseLang,
			TargetLang:  lang.Code,
			TargetLabel: lang.Label,
			Items:       items,
		})
		if err != nil {
			// Тохируулаагүй бол цааш үргэлжлүүлэх утгагүй — шууд буцна.
			if errors.Is(err, gemini.ErrNotConfigured) {
				return AutoTranslateResult{}, apperror.BadRequest("AI translation is not configured")
			}
			// Контекст цуцлагдсан бол мөн зогсооно (хүсэлт тасарсан).
			if ctx.Err() != nil {
				return AutoTranslateResult{}, apperror.InternalCause(ctx.Err())
			}
			// Нэг багц унасан нь бусдыг зогсоох шалтгаан биш — хэсэгчилсэн
			// орчуулга ч ашигтай. Тоолж, анхааруулга үлдээгээд үргэлжилнэ.
			result.Failed += len(batch)
			result.addWarning(fmt.Sprintf("batch of %d keys failed: %v", len(batch), err))
			continue
		}

		for _, key := range batch {
			value := strings.TrimSpace(out[key])
			if value == "" {
				result.Failed++
				continue
			}
			// Байрлуулагч ({name}) алдагдсан орчуулга UI-д хоосон нүх үлдээдэг
			// тул суулгахгүй — эх утга нь дутуугаас дээр.
			if !domain.PlaceholdersPreserved(req.Base[key], value) {
				result.Failed++
				result.addWarning(fmt.Sprintf("%s: placeholders changed, skipped", key))
				continue
			}
			if len(value) > domain.LanguageValueMaxBytes {
				result.Failed++
				result.addWarning(fmt.Sprintf("%s: translation too long, skipped", key))
				continue
			}
			translated[key] = value
		}
	}

	if len(translated) > 0 {
		if err := uc.repo.UpsertTranslations(ctx, lang.Code, translated, domain.TranslationSourceAI); err != nil {
			return AutoTranslateResult{}, apperror.InternalCause(err)
		}
		uc.invalidate()
	}
	result.Translated = len(translated)
	return result, nil
}

// addWarning нь анхааруулгыг дээд хязгаар хүртэл нэмнэ.
func (r *AutoTranslateResult) addWarning(msg string) {
	if len(r.Warnings) < maxWarnings {
		r.Warnings = append(r.Warnings, msg)
	}
}

// selectPending нь орчуулах шаардлагатай түлхүүрүүдийг эрэмбэлж буцаана.
//
//   - overwrite=false — зөвхөн утгагүй түлхүүр.
//   - overwrite=true  — гараар засагдсанаас бусад бүх түлхүүр.
//
// Эрэмбэлсэн нь багцлалтыг тогтвортой болгож, дахин ажиллуулахад ижил бүлэг
// үүсгэнэ (кэш/дибаг хялбар).
func selectPending(base, existing, sources map[string]string, overwrite bool) ([]string, AutoTranslateResult) {
	var result AutoTranslateResult
	pending := make([]string, 0, len(base))

	for key, text := range base {
		if strings.TrimSpace(text) == "" {
			result.Skipped++
			continue
		}
		current, has := existing[key]
		hasValue := has && strings.TrimSpace(current) != ""

		switch {
		case !hasValue:
			pending = append(pending, key)
		case overwrite && sources[key] != domain.TranslationSourceManual:
			pending = append(pending, key)
		default:
			result.Skipped++
		}
	}

	sort.Strings(pending)
	return pending, result
}

// chunkKeys нь түлхүүрүүдийг тогтмол хэмжээтэй багц болгон хуваана.
func chunkKeys(keys []string, size int) [][]string {
	if size <= 0 {
		size = len(keys)
	}
	out := make([][]string, 0, (len(keys)+size-1)/size)
	for start := 0; start < len(keys); start += size {
		end := start + size
		if end > len(keys) {
			end = len(keys)
		}
		out = append(out, keys[start:end])
	}
	return out
}

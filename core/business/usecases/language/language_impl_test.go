// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package language_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/language"
	"github.com/gerege-systems/open-gerege-core/pkg/gemini"
)

// fakeRepo нь LanguageRepository-ийн санах ойн хэрэгжилт. Mockery-гүй — зан
// төлөв нь энгийн тул гараар бичсэн нь тестийг уншихад ойлгомжтой.
type fakeRepo struct {
	langs   map[string]domain.Language
	dict    map[string]map[string]string // code → key → value
	sources map[string]map[string]string // code → key → source
	upserts int                          // UpsertTranslations дуудагдсан тоо
	failGet bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		langs: map[string]domain.Language{
			"mn": {Code: "mn", Label: "Монгол", Locale: "mn-MN", Enabled: true, IsBuiltin: true, SortOrder: 10},
			"ja": {Code: "ja", Label: "日本語", Locale: "ja-JP", Enabled: false, SortOrder: 100},
		},
		dict:    map[string]map[string]string{},
		sources: map[string]map[string]string{},
	}
}

func (r *fakeRepo) ListLanguages(context.Context) ([]domain.Language, error) {
	out := make([]domain.Language, 0, len(r.langs))
	for _, l := range r.langs {
		out = append(out, l)
	}
	return out, nil
}

func (r *fakeRepo) ListEnabledLanguages(context.Context) ([]domain.Language, error) {
	out := make([]domain.Language, 0, len(r.langs))
	for _, l := range r.langs {
		if l.Enabled {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *fakeRepo) GetLanguage(_ context.Context, code string) (domain.Language, error) {
	if r.failGet {
		return domain.Language{}, errors.New("boom")
	}
	l, ok := r.langs[code]
	if !ok {
		return domain.Language{}, apperror.NotFound("language not found")
	}
	return l, nil
}

func (r *fakeRepo) CreateLanguage(_ context.Context, lang domain.Language) (domain.Language, error) {
	if _, exists := r.langs[lang.Code]; exists {
		return domain.Language{}, apperror.Conflict("language already exists")
	}
	r.langs[lang.Code] = lang
	return lang, nil
}

func (r *fakeRepo) UpdateLanguage(_ context.Context, code string, patch domain.LanguagePatch) error {
	l, ok := r.langs[code]
	if !ok {
		return apperror.NotFound("language not found")
	}
	if patch.Label != nil {
		l.Label = *patch.Label
	}
	if patch.Locale != nil {
		l.Locale = *patch.Locale
	}
	if patch.Enabled != nil {
		l.Enabled = *patch.Enabled
	}
	if patch.SortOrder != nil {
		l.SortOrder = *patch.SortOrder
	}
	r.langs[code] = l
	return nil
}

func (r *fakeRepo) DeleteLanguage(_ context.Context, code string) error {
	if _, ok := r.langs[code]; !ok {
		return apperror.NotFound("language not found")
	}
	delete(r.langs, code)
	return nil
}

func (r *fakeRepo) GetDictionary(_ context.Context, code string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range r.dict[code] {
		out[k] = v
	}
	return out, nil
}

func (r *fakeRepo) GetTranslationSources(_ context.Context, code string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range r.sources[code] {
		out[k] = v
	}
	return out, nil
}

func (r *fakeRepo) UpsertTranslations(_ context.Context, code string, entries map[string]string, source string) error {
	r.upserts++
	if r.dict[code] == nil {
		r.dict[code] = map[string]string{}
		r.sources[code] = map[string]string{}
	}
	for k, v := range entries {
		r.dict[code][k] = v
		r.sources[code][k] = source
	}
	return nil
}

// fakeTranslator нь эх мөрийг "<<текст>>" болгож "орчуулна" — утга нь хамаагүй,
// зан төлөв (аль түлхүүр илгээгдсэн, хэдэн багц болсон) л чухал.
type fakeTranslator struct {
	batches   [][]string
	transform func(key, text string) (string, bool)
	err       error
}

func (t *fakeTranslator) TranslateBatch(_ context.Context, req language.TranslateBatchRequest) (map[string]string, error) {
	keys := make([]string, 0, len(req.Items))
	for k := range req.Items {
		keys = append(keys, k)
	}
	t.batches = append(t.batches, keys)
	if t.err != nil {
		return nil, t.err
	}
	out := map[string]string{}
	for k, v := range req.Items {
		if t.transform != nil {
			if val, ok := t.transform(k, v); ok {
				out[k] = val
			}
			continue
		}
		out[k] = "<<" + v + ">>"
	}
	return out, nil
}

func TestCreateStartsDisabled(t *testing.T) {
	repo := newFakeRepo()
	uc := language.NewUsecase(repo, nil)

	created, err := uc.Create(context.Background(), "ko", "한국어", "ko-KR")
	require.NoError(t, err)
	assert.Equal(t, "ko", created.Code)
	// Орчуулга хоосон байхад идэвхжвэл хэрэглэгч түлхүүрийн нэр харна.
	assert.False(t, created.Enabled, "шинэ хэл унтраалттай эхлэх ёстой")
}

func TestCreateRejectsBadCode(t *testing.T) {
	uc := language.NewUsecase(newFakeRepo(), nil)
	for _, code := range []string{"", "K", "toolongcode", "en_US", "../etc"} {
		_, err := uc.Create(context.Background(), code, "X", "en-US")
		assert.Error(t, err, "код %q татгалзагдах ёстой", code)
	}
}

func TestCreateDefaultsLocaleToCode(t *testing.T) {
	repo := newFakeRepo()
	uc := language.NewUsecase(repo, nil)

	created, err := uc.Create(context.Background(), "de", "Deutsch", "")
	require.NoError(t, err)
	assert.Equal(t, "de", created.Locale)
}

func TestDeleteBuiltinBlocked(t *testing.T) {
	repo := newFakeRepo()
	uc := language.NewUsecase(repo, nil)

	err := uc.Delete(context.Background(), "mn")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "built-in")
	assert.Contains(t, repo.langs, "mn", "багцлагдсан хэл устаагүй байх ёстой")
}

func TestDeleteCustomLanguage(t *testing.T) {
	repo := newFakeRepo()
	uc := language.NewUsecase(repo, nil)

	require.NoError(t, uc.Delete(context.Background(), "ja"))
	assert.NotContains(t, repo.langs, "ja")
}

func TestAutoTranslateFillsOnlyMissing(t *testing.T) {
	repo := newFakeRepo()
	repo.dict["ja"] = map[string]string{"nav.home": "ホーム"}
	repo.sources["ja"] = map[string]string{"nav.home": domain.TranslationSourceAI}
	tr := &fakeTranslator{}
	uc := language.NewUsecase(repo, tr)

	res, err := uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{
		Code: "ja",
		Base: map[string]string{"nav.home": "Нүүр", "nav.users": "Хэрэглэгчид"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Translated)
	assert.Equal(t, 1, res.Skipped, "аль хэдийн утгатай түлхүүр алгасагдана")
	assert.Equal(t, "ホーム", repo.dict["ja"]["nav.home"], "байгаа утга хэвээр")
	assert.Equal(t, "<<Хэрэглэгчид>>", repo.dict["ja"]["nav.users"])
}

func TestAutoTranslateOverwriteSparesManual(t *testing.T) {
	repo := newFakeRepo()
	repo.dict["ja"] = map[string]string{"a": "AI утга", "b": "Гар утга"}
	repo.sources["ja"] = map[string]string{
		"a": domain.TranslationSourceAI,
		"b": domain.TranslationSourceManual,
	}
	uc := language.NewUsecase(repo, &fakeTranslator{})

	res, err := uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{
		Code:      "ja",
		Base:      map[string]string{"a": "Нэг", "b": "Хоёр"},
		Overwrite: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Translated)
	assert.Equal(t, "<<Нэг>>", repo.dict["ja"]["a"], "AI утга дахин үүснэ")
	assert.Equal(t, "Гар утга", repo.dict["ja"]["b"], "гар засвар хэзээ ч дарагдахгүй")
}

func TestAutoTranslateRejectsBrokenPlaceholders(t *testing.T) {
	repo := newFakeRepo()
	tr := &fakeTranslator{transform: func(_, text string) (string, bool) {
		// Байрлуулагчийг "орчуулж" гээсэн загвар — яг энэ алдааг барих ёстой.
		return strings.ReplaceAll(text, "{name}", "名前"), true
	}}
	uc := language.NewUsecase(repo, tr)

	res, err := uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{
		Code: "ja",
		Base: map[string]string{"greet": "Сайн уу, {name}", "plain": "Хадгалах"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Translated)
	assert.Equal(t, 1, res.Failed)
	assert.NotContains(t, repo.dict["ja"], "greet", "эвдэрсэн байрлуулагчтай утга суухгүй")
	assert.Contains(t, repo.dict["ja"], "plain")
	assert.NotEmpty(t, res.Warnings)
}

func TestAutoTranslateBatchesLargeDictionary(t *testing.T) {
	repo := newFakeRepo()
	tr := &fakeTranslator{}
	uc := language.NewUsecase(repo, tr)

	base := make(map[string]string, 130)
	for i := 0; i < 130; i++ {
		base[fmt.Sprintf("key.%03d", i)] = fmt.Sprintf("утга %d", i)
	}
	res, err := uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{Code: "ja", Base: base})
	require.NoError(t, err)
	assert.Equal(t, 130, res.Translated)
	// 130 түлхүүр / 60 = 3 багц (60 + 60 + 10).
	assert.Len(t, tr.batches, 3)
}

func TestAutoTranslateContinuesAfterBatchFailure(t *testing.T) {
	repo := newFakeRepo()
	tr := &fakeTranslator{err: errors.New("upstream 500")}
	uc := language.NewUsecase(repo, tr)

	res, err := uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{
		Code: "ja",
		Base: map[string]string{"a": "Нэг", "b": "Хоёр"},
	})
	require.NoError(t, err, "нэг багц унасан нь бүх үйлдлийг унагаахгүй")
	assert.Equal(t, 0, res.Translated)
	assert.Equal(t, 2, res.Failed)
	assert.NotEmpty(t, res.Warnings)
	assert.Zero(t, repo.upserts, "бичих зүйлгүй бол DB хөндөхгүй")
}

func TestAutoTranslateNotConfigured(t *testing.T) {
	uc := language.NewUsecase(newFakeRepo(), nil)
	_, err := uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{
		Code: "ja", Base: map[string]string{"a": "Нэг"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")

	// Translator байгаа ч API key дутуу үед мөн ойлгомжтой алдаа өгнө.
	uc = language.NewUsecase(newFakeRepo(), &fakeTranslator{err: gemini.ErrNotConfigured})
	_, err = uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{
		Code: "ja", Base: map[string]string{"a": "Нэг"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestAutoTranslateRejectsSameLanguage(t *testing.T) {
	uc := language.NewUsecase(newFakeRepo(), &fakeTranslator{})
	_, err := uc.AutoTranslate(context.Background(), language.AutoTranslateRequest{
		Code: "mn", Base: map[string]string{"a": "Нэг"}, BaseLang: "mn",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same")
}

func TestDictionaryCacheInvalidatedOnWrite(t *testing.T) {
	repo := newFakeRepo()
	uc := language.NewUsecase(repo, nil)
	ctx := context.Background()

	first, err := uc.Dictionary(ctx, "ja")
	require.NoError(t, err)
	assert.Empty(t, first)

	require.NoError(t, uc.PutTranslations(ctx, "ja", map[string]string{"nav.home": "ホーム"}))

	// Кэш хүчингүй болсон тул шинэ утга шууд харагдана (TTL хүлээхгүй).
	second, err := uc.Dictionary(ctx, "ja")
	require.NoError(t, err)
	assert.Equal(t, "ホーム", second["nav.home"])
}

func TestPutTranslationsMarksManual(t *testing.T) {
	repo := newFakeRepo()
	uc := language.NewUsecase(repo, nil)

	require.NoError(t, uc.PutTranslations(context.Background(), "ja", map[string]string{"a": "あ"}))
	assert.Equal(t, domain.TranslationSourceManual, repo.sources["ja"]["a"])
}

func TestPutTranslationsUnknownLanguage(t *testing.T) {
	uc := language.NewUsecase(newFakeRepo(), nil)
	err := uc.PutTranslations(context.Background(), "xx", map[string]string{"a": "b"})
	require.Error(t, err)
}

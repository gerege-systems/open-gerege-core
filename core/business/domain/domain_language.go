// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Language нь аппын интерфейсийн хэл. Түлхүүрийн жагсаалт нь аппын өөрийнх
// (frontend-д багцлагдсан dictionary) бөгөөд платформ нь зөвхөн УТГЫГ хадгална.
// Тиймээс platform-core нь тухайн аппын түлхүүрийг мэдэхгүй ерөнхий хэвээр.
type Language struct {
	Code      string     `db:"code"`
	Label     string     `db:"label"`
	Locale    string     `db:"locale"`
	Enabled   bool       `db:"enabled"`
	IsBuiltin bool       `db:"is_builtin"`
	SortOrder int        `db:"sort_order"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

// Translation нь нэг хэлний нэг түлхүүрийн утга.
type Translation struct {
	LangCode  string    `db:"lang_code"`
	Key       string    `db:"key"`
	Value     string    `db:"value"`
	Source    string    `db:"source"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Орчуулгын утга хаанаас ирсэн бэ. AI-аар үүсгэснийг дараа нь гараар засвал
// TranslationSourceManual болох ба дахин автомат орчуулга үүнийг дарж бичихгүй.
const (
	TranslationSourceManual = "manual"
	TranslationSourceAI     = "ai"
	TranslationSourceImport = "import"
)

// TranslationSources нь зөвшөөрөгдсөн source утгууд.
var TranslationSources = map[string]bool{
	TranslationSourceManual: true,
	TranslationSourceAI:     true,
	TranslationSourceImport: true,
}

// LanguagePatch нь хэлний хэсэгчилсэн шинэчлэлт — nil талбар "хэвээр".
// Code болон IsBuiltin өөрчлөгдөхгүй (code нь түлхүүр, builtin нь кодын баримт).
type LanguagePatch struct {
	Label     *string
	Locale    *string
	Enabled   *bool
	SortOrder *int
}

// Хязгаарууд — DoS ба санамсаргүй асар том хүсэлтээс хамгаална.
const (
	// LanguageDictionaryMaxKeys нь нэг хүсэлтээр бичих түлхүүрийн дээд тоо.
	// Лавлах хэрэгжилтийн (template) dictionary ~830 түлхүүртэй.
	LanguageDictionaryMaxKeys = 5000
	// LanguageKeyMaxBytes нь нэг түлхүүрийн нэрийн дээд урт.
	LanguageKeyMaxBytes = 200
	// LanguageValueMaxBytes нь нэг орчуулгын утгын дээд урт.
	LanguageValueMaxBytes = 8 * 1024
	// LanguageDictionaryMaxBytes нь нэг хүсэлтийн бүх утгын нийлбэр дээд хэмжээ.
	LanguageDictionaryMaxBytes = 2 * 1024 * 1024
)

// BCP-47-ийн практик дэд олонлог: 'mn', 'en-US', 'zh-Hans', 'zh-Hans-CN'.
// Бүрэн BCP-47 (extension, private-use) шаардлагагүй тул зориуд нарийсгав —
// код нь URL зам болон cookie-д ордог.
var languageCodeRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8}){0,2}$`)

// languageLocaleRe нь Intl locale — код шиг боловч бүс заавал том үсгээр
// бичигдэх шаардлагагүй тул ижил дүрмээр шалгана.
var languageLocaleRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8}){0,2}$`)

// placeholderRe нь орчуулгад хадгалагдах ёстой байрлуулагчийг олно: {0}, {name}.
// Загвар нь frontend-ийн dictionary-тэй нийцнэ; AI орчуулга эдгээрийг
// орчуулж/гээж болзошгүй тул тусад нь шалгана.
var placeholderRe = regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)

// ValidateLanguageCode нь хэлний кодыг шалгана.
func ValidateLanguageCode(code string) error {
	if code == "" {
		return fmt.Errorf("language code is required")
	}
	if !languageCodeRe.MatchString(code) {
		return fmt.Errorf("invalid language code %q (expected BCP-47 like 'mn', 'en-US', 'zh-Hans')", code)
	}
	return nil
}

// ValidateLanguage нь хэлний бүх талбарыг шалгана (үүсгэх үед).
func ValidateLanguage(code, label, locale string) error {
	if err := ValidateLanguageCode(code); err != nil {
		return err
	}
	if strings.TrimSpace(label) == "" {
		return fmt.Errorf("language label is required")
	}
	if len(label) > 80 {
		return fmt.Errorf("language label too long (max 80)")
	}
	if !languageLocaleRe.MatchString(locale) {
		return fmt.Errorf("invalid locale %q (expected like 'mn-MN')", locale)
	}
	return nil
}

// ValidateDictionary нь бичих гэж буй орчуулгын багцыг шалгана: түлхүүрийн тоо,
// нэр/утгын урт, нийт хэмжээ.
func ValidateDictionary(entries map[string]string) error {
	if len(entries) == 0 {
		return fmt.Errorf("dictionary is empty")
	}
	if len(entries) > LanguageDictionaryMaxKeys {
		return fmt.Errorf("too many keys (%d, max %d)", len(entries), LanguageDictionaryMaxKeys)
	}
	total := 0
	for key, value := range entries {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("empty translation key")
		}
		if len(key) > LanguageKeyMaxBytes {
			return fmt.Errorf("translation key too long (%q, max %d bytes)", key[:32], LanguageKeyMaxBytes)
		}
		if len(value) > LanguageValueMaxBytes {
			return fmt.Errorf("translation value for %q too long (max %d bytes)", key, LanguageValueMaxBytes)
		}
		total += len(key) + len(value)
		if total > LanguageDictionaryMaxBytes {
			return fmt.Errorf("dictionary too large (max %d bytes)", LanguageDictionaryMaxBytes)
		}
	}
	return nil
}

// Placeholders нь мөр доторх байрлуулагчдыг эрэмбэлсэн, давхардалгүй жагсаалт
// болгож буцаана.
func Placeholders(s string) []string {
	found := placeholderRe.FindAllString(s, -1)
	if len(found) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(found))
	out := make([]string, 0, len(found))
	for _, p := range found {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// PlaceholdersPreserved нь орчуулга эх мөрийн байрлуулагчдыг бүрэн хадгалсан
// эсэхийг шалгана. AI орчуулга {name}-ийг орчуулж эсвэл гээж болзошгүй бөгөөд
// тэр нь UI-д хоосон нүх үлдээдэг тул суулгахаас өмнө барина.
func PlaceholdersPreserved(source, translated string) bool {
	want := Placeholders(source)
	if len(want) == 0 {
		return true
	}
	got := Placeholders(translated)
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

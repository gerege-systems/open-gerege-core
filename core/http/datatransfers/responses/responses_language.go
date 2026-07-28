// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"github.com/gerege-systems/platform-core/core/business/domain"
	languageuc "github.com/gerege-systems/platform-core/core/business/usecases/language"
)

// LanguageResponse нь интерфейсийн хэлийг frontend-д буцаах хэлбэр.
type LanguageResponse struct {
	Code      string     `json:"code"`
	Label     string     `json:"label"`
	Locale    string     `json:"locale"`
	Enabled   bool       `json:"enabled"`
	IsBuiltin bool       `json:"is_builtin"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

func ToLanguage(l domain.Language) LanguageResponse {
	return LanguageResponse{
		Code:      l.Code,
		Label:     l.Label,
		Locale:    l.Locale,
		Enabled:   l.Enabled,
		IsBuiltin: l.IsBuiltin,
		SortOrder: l.SortOrder,
		CreatedAt: l.CreatedAt,
		UpdatedAt: l.UpdatedAt,
	}
}

func ToLanguageList(list []domain.Language) []LanguageResponse {
	out := make([]LanguageResponse, 0, len(list))
	for _, l := range list {
		out = append(out, ToLanguage(l))
	}
	return out
}

// LanguageDictionaryResponse нь нэг хэлний бүх орчуулга. Entries хоосон байж
// болно — тэр тохиолдолд апп багцлагдсан утгаа хэрэглэнэ.
type LanguageDictionaryResponse struct {
	Code    string            `json:"code"`
	Entries map[string]string `json:"entries"`
}

func ToLanguageDictionary(code string, entries map[string]string) LanguageDictionaryResponse {
	if entries == nil {
		entries = map[string]string{}
	}
	return LanguageDictionaryResponse{Code: code, Entries: entries}
}

// LanguageAutoTranslateResponse нь автомат орчуулгын хураангуй.
type LanguageAutoTranslateResponse struct {
	Translated int      `json:"translated"`
	Skipped    int      `json:"skipped"`
	Failed     int      `json:"failed"`
	Warnings   []string `json:"warnings,omitempty"`
}

func ToLanguageAutoTranslate(r languageuc.AutoTranslateResult) LanguageAutoTranslateResponse {
	return LanguageAutoTranslateResponse{
		Translated: r.Translated,
		Skipped:    r.Skipped,
		Failed:     r.Failed,
		Warnings:   r.Warnings,
	}
}

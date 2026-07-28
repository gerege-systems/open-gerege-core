// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package _interface

import (
	"context"

	"github.com/gerege-systems/public-gerege-core/core/business/domain"
)

// LanguageRepository нь интерфейсийн хэл (languages) ба тэдгээрийн орчуулга
// (translations) хүснэгтийг удирдана. Нийтийн config тул RLS-гүй — унших нь
// нээлттэй, бичих нь route түвшинд super admin-аар хаагдана.
type LanguageRepository interface {
	// ListLanguages нь бүх хэлийг эрэмбээр буцаана (админ).
	ListLanguages(ctx context.Context) ([]domain.Language, error)
	// ListEnabledLanguages нь зөвхөн идэвхтэй хэлийг буцаана (нийтийн).
	ListEnabledLanguages(ctx context.Context) ([]domain.Language, error)
	// GetLanguage нь кодоор нэг хэл буцаана; олдохгүй бол apperror.NotFound.
	GetLanguage(ctx context.Context, code string) (domain.Language, error)
	// CreateLanguage нь шинэ хэл үүсгэж, үүсгэсэн мөрийг буцаана. Код давхардвал
	// apperror.Conflict.
	CreateLanguage(ctx context.Context, lang domain.Language) (domain.Language, error)
	// UpdateLanguage нь хэсэгчилсэн шинэчлэлт хийнэ (nil талбар хэвээр).
	UpdateLanguage(ctx context.Context, code string, patch domain.LanguagePatch) error
	// DeleteLanguage нь хэлийг устгана; орчуулга нь CASCADE-аар хамт устана.
	// Builtin хэлийг устгахыг usecase хориглоно.
	DeleteLanguage(ctx context.Context, code string) error

	// GetDictionary нь тухайн хэлний бүх түлхүүр→утга газрын зургийг буцаана.
	GetDictionary(ctx context.Context, code string) (map[string]string, error)
	// GetTranslationSources нь түлхүүр→source газрын зураг. Автомат орчуулга нь
	// гараар засагдсан ('manual') утгыг дарж бичихгүйн тулд ашиглана.
	GetTranslationSources(ctx context.Context, code string) (map[string]string, error)
	// UpsertTranslations нь багц орчуулгыг нэг хүсэлтээр бичнэ (insert эсвэл update).
	UpsertTranslations(ctx context.Context, code string, entries map[string]string, source string) error
}

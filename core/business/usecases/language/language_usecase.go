// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package language нь интерфейсийн хэлийг удирдана — хэл нэмэх/хасах,
// идэвхжүүлэх, орчуулгыг гараар эсвэл Gemini-ээр бөглөх.
//
// ХАРИУЦЛАГЫН ХУВААРЬ: түлхүүрийн ЖАГСААЛТ нь аппын өөрийнх (frontend-д
// багцлагдсан dictionary). Платформ нь тэдгээрийн УТГЫГ л хадгална. Иймд
// автомат орчуулгын үед апп эх хэлний түлхүүр→текст-ээ хамт илгээнэ; платформ
// аль ч аппын түлхүүрийг урьдчилан мэдэхгүй ерөнхий хэвээр үлдэнэ.
//
// Идэвхтэй хэлийн жагсаалт болон dictionary-г хуудас бүрийн ачаалалт уншдаг тул
// богино TTL кэштэй; бичих үйлдэл кэшийг хүчингүй болгоно.
package language

import (
	"context"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
)

// Translator нь багц орчуулгын гадаад үйлчилгээ (үйлдвэрлэлд Gemini).
// Интерфейс болгосон нь usecase-ийг сүлжээнээс салгаж, тестэд хуурамч
// хэрэгжилт оруулах боломж олгоно.
type Translator interface {
	// TranslateBatch нь түлхүүр→эх текст газрын зургийг зорилтот хэл рүү
	// орчуулж, ижил түлхүүртэй газрын зураг буцаана. Хэсэгчилсэн үр дүн
	// зөвшөөрөгдөнө — дутуу түлхүүрийг дуудагч алгасна.
	TranslateBatch(ctx context.Context, req TranslateBatchRequest) (map[string]string, error)
}

// TranslateBatchRequest нь нэг багц орчуулгын оролт.
type TranslateBatchRequest struct {
	// SourceLang / TargetLang нь хэлний код; Label нь prompt-д ойлгомжтой
	// байлгах зорилготой хүний нэр ('日本語').
	SourceLang  string
	TargetLang  string
	TargetLabel string
	// Items нь түлхүүр→эх текст.
	Items map[string]string
}

// AutoTranslateRequest нь хэлийг "суулгах" хүсэлт — аппын эх dictionary-г
// зорилтот хэл рүү бүтэн орчуулна.
type AutoTranslateRequest struct {
	Code string
	// Base нь эх хэлний түлхүүр→текст (аппаас ирнэ).
	Base map[string]string
	// BaseLang нь Base-ийн хэлний код (хоосон бол "mn").
	BaseLang string
	// Overwrite нь аль хэдийн AI-аар орчуулагдсан утгыг ДАХИН үүсгэх эсэх.
	// Гараар засагдсан ('manual') утгыг хэзээ ч дарж бичихгүй — хүний засвар
	// автомат орчуулгаас үргэлж давуу.
	Overwrite bool
}

// AutoTranslateResult нь автомат орчуулгын хураангуй.
type AutoTranslateResult struct {
	// Translated нь шинээр бичигдсэн түлхүүрийн тоо.
	Translated int
	// Skipped нь аль хэдийн орчуулагдсан (эсвэл гараар засагдсан) тул
	// хөндөөгүй түлхүүрийн тоо.
	Skipped int
	// Failed нь model буцаагаагүй эсвэл шалгалт даваагүй түлхүүрийн тоо.
	Failed int
	// Warnings нь хүнд харуулах анхааруулга (жишээ нь байрлуулагч алдагдсан).
	// Хэмжээг хязгаарласан — бүх алдааг жагсаахгүй.
	Warnings []string
}

type Usecase interface {
	// List нь бүх хэлийг буцаана (super admin).
	List(ctx context.Context) ([]domain.Language, error)
	// ListEnabled нь идэвхтэй хэлийг буцаана (нийтийн, кэштэй).
	ListEnabled(ctx context.Context) ([]domain.Language, error)
	Get(ctx context.Context, code string) (domain.Language, error)
	Create(ctx context.Context, code, label, locale string) (domain.Language, error)
	Update(ctx context.Context, code string, patch domain.LanguagePatch) error
	// Delete нь хэлийг устгана. Багцлагдсан (builtin) хэлийг устгахгүй —
	// зөвхөн унтраана.
	Delete(ctx context.Context, code string) error
	// Dictionary нь тухайн хэлний түлхүүр→утга газрын зураг (нийтийн, кэштэй).
	Dictionary(ctx context.Context, code string) (map[string]string, error)
	// PutTranslations нь гараар оруулсан орчуулгыг хадгална.
	PutTranslations(ctx context.Context, code string, entries map[string]string) error
	// AutoTranslate нь дутуу түлхүүрийг Gemini-ээр бөглөнө.
	AutoTranslate(ctx context.Context, req AutoTranslateRequest) (AutoTranslateResult, error)
}

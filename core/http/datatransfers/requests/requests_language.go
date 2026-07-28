// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// LanguageCreateRequest нь шинэ хэл нэмэх super admin хүсэлт. Шинэ хэл нь
// ҮРГЭЛЖ унтраалттай үүснэ — орчуулга бөглөсний дараа PATCH-аар идэвхжүүлнэ.
type LanguageCreateRequest struct {
	// Code нь BCP-47 хэлний код: 'ja', 'en-US', 'zh-Hans'.
	Code string `json:"code" validate:"required,max=32"`
	// Label нь хэлний эх нэр (сонгогчид харагдана): '日本語'.
	Label string `json:"label" validate:"required,max=80"`
	// Locale нь Intl форматлах locale ('ja-JP'). Хоосон бол Code-ыг хэрэглэнэ.
	Locale string `json:"locale" validate:"max=32"`
}

// LanguageUpdateRequest нь хэсэгчилсэн шинэчлэлт — заагаагүй талбар хэвээр.
// Заагаагүй/заасныг ялгахын тулд заавал заагч (pointer) төрөл.
type LanguageUpdateRequest struct {
	Label     *string `json:"label" validate:"omitempty,max=80"`
	Locale    *string `json:"locale" validate:"omitempty,max=32"`
	Enabled   *bool   `json:"enabled"`
	SortOrder *int    `json:"sort_order"`
}

// LanguageTranslationsRequest нь орчуулгыг гараар (эсвэл JSON багцаар) бичих
// хүсэлт. Түлхүүр нь аппынх тул чөлөөт газрын зураг — нарийн шалгалт domain-д
// (ValidateDictionary: тоо, урт, нийт хэмжээ).
type LanguageTranslationsRequest struct {
	Entries map[string]string `json:"entries" validate:"required"`
}

// LanguageAutoTranslateRequest нь хэлийг "суулгах" хүсэлт — аппын эх хэлний
// dictionary-г илгээж, Gemini-ээр дутуу түлхүүрийг бөглүүлнэ.
//
// Base-ыг апп илгээдэг нь санаатай: түлхүүрийн жагсаалт нь аппын өөрийнх тул
// платформ түүнийг урьдчилан мэдэхгүй ерөнхий хэвээр үлдэнэ.
type LanguageAutoTranslateRequest struct {
	// Base нь эх хэлний түлхүүр→текст.
	Base map[string]string `json:"base" validate:"required"`
	// BaseLang нь Base-ийн хэлний код. Хоосон бол 'mn'.
	BaseLang string `json:"base_lang" validate:"max=32"`
	// Overwrite нь өмнө нь AI-аар үүсгэсэн утгыг дахин үүсгэх эсэх. Гараар
	// засагдсан утга ХЭЗЭЭ Ч дарагдахгүй.
	Overwrite bool `json:"overwrite"`
}

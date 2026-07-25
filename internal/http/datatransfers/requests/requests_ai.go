// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// AIAudio нь base64 кодлогдсон оролтын дуу. Mime нь browser MediaRecorder-ийн
// гаргадаг түгээмэл audio төрлүүдээр хязгаарлагдана; Data нь ~700 KB base64
// (~520 KB түүхий, opus-аар ~30 секунд) — глобал body хязгаарт багтана.
type AIAudio struct {
	Mime string `json:"mime" validate:"required,oneof=audio/webm audio/ogg audio/wav audio/mpeg audio/mp3 audio/mp4 audio/m4a audio/aac audio/flac"`
	Data string `json:"data" validate:"required,base64,max=716800"`
}

// AIChatTurn нь өмнөх харилцааны нэг ээлж. role: "user" | "model".
type AIChatTurn struct {
	Role string `json:"role" validate:"required,oneof=user model"`
	Text string `json:"text" validate:"required,max=4000"`
}

// AIChatRequest нь POST /ai/chat-ийн body. Message эсвэл Audio-гийн ядаж нэг
// нь шаардлагатай (handler шалгана) — дуут мессежийг AI шууд ойлгоно.
// History нь сонголттой — frontend өмнөх ээлжүүдээ дамжуулж харилцааг
// үргэлжлүүлнэ (сервер талд чат төлөв хадгалдаггүй, stateless).
type AIChatRequest struct {
	Message string       `json:"message" validate:"omitempty,max=4000"`
	Audio   *AIAudio     `json:"audio" validate:"omitempty"`
	History []AIChatTurn `json:"history" validate:"omitempty,max=20,dive"`
	// Lang нь frontend-ийн UI хэл — туслах ЭНЭ хэлээр хариулна. Хоосон бол
	// сервер өгөгдмөл (mn) хэрэглэнэ; цагаан жагсаалтаас гадуур утга орохгүй.
	Lang string `json:"lang" validate:"omitempty,oneof=mn en zh ru"`
}

// AIPublicChatRequest нь POST /public/ai/chat-ийн body — нүүр хуудасны
// нээлттэй (нэвтрэлтгүй) чат виджет. Нэвтэрсэн чатаас ЯЛГААТАЙ нь:
//   - audio байхгүй (нэргүй гадаргуугаас 700 KB upload авахгүй),
//   - мессеж ба ээлж бүр 1000 тэмдэгт, түүх 6 ээлж — Gemini-ийн зардлыг
//     нэргүй хэрэглэгчийн хувьд хатуу хязгаарлана.
type AIPublicChatRequest struct {
	// Message нь audio байхгүй үед заавал (handler шалгана) — push-to-talk
	// горимд зөвхөн audio ирнэ.
	Message string             `json:"message" validate:"omitempty,max=1000"`
	Audio   *AIPublicAudio     `json:"audio" validate:"omitempty"`
	History []AIPublicChatTurn `json:"history" validate:"omitempty,max=6,dive"`
	Lang    string             `json:"lang" validate:"omitempty,oneof=mn en zh ru"`
}

// AIPublicAudio нь нээлттэй чатын push-to-talk бичлэг. Нэвтэрсэн чатын
// AIAudio-гаас ЯЛГААТАЙ нь хэмжээ: ~250 KB base64 (~15 секунд opus) —
// нэргүй гадаргуугаас ирэх upload-ыг богино байлгана.
type AIPublicAudio struct {
	Mime string `json:"mime" validate:"required,oneof=audio/webm audio/ogg audio/wav audio/mpeg audio/mp3 audio/mp4 audio/m4a audio/aac audio/flac"`
	Data string `json:"data" validate:"required,base64,max=256000"`
}

// AIPublicTTSRequest нь нээлттэй чатын «сонсох» товч — AI-ийн хариултыг
// дуут болгоно. Текстийн хязгаар нэвтэрсэн TTS-ээс (2000) богино: нэргүй
// гадаргуу дээр нэг дуудалтын зардлыг таазлана.
type AIPublicTTSRequest struct {
	Text string `json:"text" validate:"required,max=800"`
}

// AIPublicChatTurn нь нээлттэй чатын нэг ээлж — AIChatTurn-тэй ижил боловч
// текстийн хязгаар богино (нэргүй гадаргуугийн зардлын хамгаалалт).
type AIPublicChatTurn struct {
	Role string `json:"role" validate:"required,oneof=user model"`
	Text string `json:"text" validate:"required,max=1000"`
}

// AISTTRequest нь POST /ai/stt-ийн body — audio-г текст болгоно.
type AISTTRequest struct {
	Audio AIAudio `json:"audio" validate:"required"`
}

// AITTSRequest нь POST /ai/tts-ийн body — текстийг яриа болгоно.
type AITTSRequest struct {
	Text  string `json:"text" validate:"required,max=2000"`
	Voice string `json:"voice" validate:"omitempty,alphanum,max=40"`
}

// AIPromptUpdateRequest нь PUT /admin/ai/prompts/{key}-ийн body. Хоосон
// content зөвшөөрөгдөнө (давхаргыг цэвэрлэх) — scope хоосон бол env/default
// fallback хэрэглэгдэнэ.
type AIPromptUpdateRequest struct {
	Content string `json:"content" validate:"max=4000"`
}

// AITranslateRequest нь POST /ai/translate-ийн body. Text эсвэл Audio-гийн
// ядаж нэг нь шаардлагатай (handler шалгана); Speak үнэн бол орчуулгын
// дуут (TTS) хувилбар хамт ирнэ. Live орчуулга = frontend жижиг audio
// chunk-уудыг энэ endpoint руу дараалан илгээх урсгал.
type AITranslateRequest struct {
	Text       string   `json:"text" validate:"omitempty,max=4000"`
	Audio      *AIAudio `json:"audio" validate:"omitempty"`
	TargetLang string   `json:"target_lang" validate:"required,min=2,max=20"`
	Speak      bool     `json:"speak"`
}

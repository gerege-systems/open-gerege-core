// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"template/internal/apperror"
	"template/internal/constants"
	"template/pkg/gemini"
	"template/pkg/logger"
)

// speechError нь Gemini-ийн алдааг ангилна: хугацаа хэтэрсэн / холболт тасарсан
// зэрэг ТҮР ЗУУРЫН саатлыг 503 болгож, бусдыг 500-аар үлдээнэ. TTS нь урт
// текст дээр 10-20 секунд зарцуулдаг тул deadline-д мөргөх нь бодит бөгөөд
// дахин оролдоход эдгэрдэг тохиолдол — «дотоод алдаа» гэж хэлэх нь буруу.
func speechError(op string, err error) error {
	wrapped := fmt.Errorf("%s: %w", op, err)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		errors.Is(err, gemini.ErrUnavailable) {
		return apperror.UnavailableCause(wrapped)
	}
	return apperror.InternalCause(wrapped)
}

// Дуу хоолойн боломжууд: Transcribe (STT) / Speak (TTS) / Translate.
// Чатаас ялгаатай нь эдгээр нь fallback мессеж буцаадаггүй — алдааг шууд
// error болгож өгнө (дуудагч UI өөрөө "дахин оролд" гэж харуулна).

// sttInstruction нь STT-ийн тогтмол дүрэм — зөвхөн сонссоноо буцаана.
const sttInstruction = "Чи яриа-текст (STT) хөрвүүлэгч. Өгсөн audio-д сонсогдсон яриаг " +
	"яг хэлсэн хэлээр нь, үг үсгийн алдаагүй, тайлбаргүйгээр зөвхөн текст болгон буцаа. " +
	"Яриа сонсогдохгүй бол хоосон мөр буцаа."

// PlatformVocabulary нь платформын чатад дуут мессеж хөрвүүлэхэд STT-д өгөх
// нэр томьёоны жагсаалт. Хэрэглэгчид ихэвчлэн эдгээр сэдвээр асуудаг тул
// ойролцоо дуудлагатай үгийн сонголтыг зөв тийш нь татна.
const PlatformVocabulary = "Gerege, платформ, eID, цахим үнэмлэх, нэвтрэх, бүртгэл, " +
	"нэвтрэлт, аюулгүй байдал, хамгаалалт, QR код, регистрийн дугаар, гарын үсэг, " +
	"тамга, байгууллага, төрийн үйлчилгээ, лавлагаа, токен, нууцлал, SSO, API"

func (uc *usecase) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	instruction := sttInstruction
	if req.Vocabulary != "" {
		instruction += " Энэ бичлэг нь дараах сэдвийн хүрээнд байх магадлалтай тул " +
			"ойролцоо дуудлагатай үг тааралдвал эдгээрийг илүүд үз: " + req.Vocabulary + "."
	}
	resp, err := uc.client.GenerateContent(ctx, gemini.Request{
		SystemInstruction: &gemini.Content{Parts: []gemini.Part{{Text: instruction}}},
		Contents: []gemini.Content{{
			Role: "user",
			Parts: []gemini.Part{
				{Text: "Энэ бичлэгийг текст болго."},
				{InlineData: &gemini.Blob{MimeType: req.Audio.Mime, Data: req.Audio.Data}},
			},
		}},
	})
	if err != nil {
		return TranscribeResult{}, speechError("ai transcribe", err)
	}
	return TranscribeResult{Text: resp.Text()}, nil
}

// speakAttempts — TTS дуудалт үе үе бүтэлгүйтдэг (доорх speakInstruction-ийг
// үзнэ үү) тул хэрэглэгчид алдаа өгөхийн өмнө хэдэн удаа дахин оролдоно.
// Нэг дуудалт 3-5 секунд тул AIRequestTimeout (50с)-д гурав багтана.
const speakAttempts = 3

// speakInstruction нь TTS model-д «унш, бүү хариул» гэдгийг ил хэлнэ.
//
// Яагаад хэрэгтэй вэ: зөвхөн текстийг дангаар нь илгээхэд model нь богино
// өгүүлбэр, ялангуяа асуултыг (ж: «eID гэж юу вэ?») УНШИХЫН оронд ХАРИУЛАХ
// гэж оролддог. Тэр үед API нь аудиогүй хариу, эсвэл шууд алдаа буцаадаг:
//
//	400 "Model tried to generate text, but it should only be used for TTS.
//	     Make sure your instructions are clear to only generate audio from a
//	     given text transcript."
//
// Заавар нь Google-ийн баримтжуулсан хэв маяг («Say: …») — model зөвхөн
// хоёр цэгийн дараах текстийг дуугаргана.
const speakInstruction = "Read the following text aloud in its own language, " +
	"exactly as written, with a natural and calm tone. Do not answer it, do not " +
	"translate it, do not add or omit anything — only speak this text:\n\n"

// wrapSpeakText нь зааврыг текстэд хавсаргана.
func wrapSpeakText(text string) string { return speakInstruction + text }

func (uc *usecase) Speak(ctx context.Context, req SpeakRequest) (SpeakResult, error) {
	voice := req.Voice
	if voice == "" {
		voice = uc.cfg.Voice
	}
	geminiReq := gemini.Request{
		Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: wrapSpeakText(req.Text)}}}},
		GenerationConfig: &gemini.GenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: &gemini.SpeechConfig{
				VoiceConfig: &gemini.VoiceConfig{
					PrebuiltVoiceConfig: &gemini.PrebuiltVoiceConfig{VoiceName: voice},
				},
			},
		},
	}

	var lastErr error
	for attempt := 1; attempt <= speakAttempts; attempt++ {
		resp, err := uc.ttsClient.GenerateContent(ctx, geminiReq)
		switch {
		case err == nil:
			if blob := resp.InlineAudio(); blob != nil {
				return toWAV(*blob)
			}
			lastErr = errors.New("no audio in response")
		case errors.Is(err, gemini.ErrNotConfigured), ctx.Err() != nil:
			// Тохиргооны алдаа / хугацаа дууссан — дахин оролдох нь утгагүй.
			return SpeakResult{}, speechError("ai speak", err)
		default:
			// Model «унших» биш «хариулах» горимд орсон үеийн 400 нь тогтмол
			// биш — дахин оролдоход эдгэрдэг тул энд ч гэсэн дахин оролдоно.
			lastErr = err
		}
		logger.WarnWithContext(ctx, "ai: tts attempt failed, retrying", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryAI,
			"attempt":                attempt,
			"attempts":               speakAttempts,
			"error":                  lastErr.Error(),
		})
	}
	// Бүх оролдлого бүтэлгүй — түр зуурын саатал (503), дотоод алдаа биш.
	return SpeakResult{}, apperror.UnavailableCause(
		fmt.Errorf("ai speak: %d attempts failed: %w", speakAttempts, lastErr))
}

// toWAV нь TTS-ийн түүхий PCM гаралтыг browser тоглуулж чадах WAV болгоно;
// model өөр контейнер форматтай буцаавал байгаагаар нь дамжуулна.
func toWAV(blob gemini.Blob) (SpeakResult, error) {
	if !strings.Contains(strings.ToLower(blob.MimeType), "l16") &&
		!strings.Contains(strings.ToLower(blob.MimeType), "pcm") {
		return SpeakResult{Mime: blob.MimeType, Data: blob.Data}, nil
	}
	pcm, err := base64.StdEncoding.DecodeString(blob.Data)
	if err != nil {
		return SpeakResult{}, apperror.InternalCause(fmt.Errorf("ai speak: decode pcm: %w", err))
	}
	wav := gemini.PCMToWAV(pcm, gemini.PCMRateFromMime(blob.MimeType))
	return SpeakResult{Mime: "audio/wav", Data: base64.StdEncoding.EncodeToString(wav)}, nil
}

// langNames нь түгээмэл хэлний кодыг хүний нэр рүү буулгана — prompt-д
// ойлгомжтой болгох зорилготой; жагсаалтад байхгүй кодыг байгаагаар нь өгнө.
var langNames = map[string]string{
	"mn": "Монгол",
	"en": "English",
	"ru": "Русский",
	"zh": "中文",
	"ja": "日本語",
	"ko": "한국어",
	"de": "Deutsch",
}

func (uc *usecase) Translate(ctx context.Context, req TranslateRequest) (TranslateResult, error) {
	sourceText := strings.TrimSpace(req.Text)

	// Audio оролттой бол эхлээд STT — хоёр алхамт pipeline нь structured
	// output шаардахгүй тул найдвартай.
	if sourceText == "" && req.Audio != nil {
		tr, err := uc.Transcribe(ctx, TranscribeRequest{Audio: *req.Audio})
		if err != nil {
			return TranslateResult{}, err
		}
		sourceText = strings.TrimSpace(tr.Text)
		if sourceText == "" {
			// Яриа илрээгүй chunk (чимээгүй хэсэг) — алдаа биш, хоосон үр дүн.
			return TranslateResult{}, nil
		}
	}
	if sourceText == "" {
		return TranslateResult{}, apperror.BadRequest("text or audio is required")
	}

	target := req.TargetLang
	if name, ok := langNames[strings.ToLower(target)]; ok {
		target = name
	}

	instruction := fmt.Sprintf(
		"Чи мэргэжлийн синхрон орчуулагч. Өгсөн текстийг %s хэл рүү орчуулж "+
			"ЗӨВХӨН орчуулсан текстийг буцаа — тайлбар, хашилт, оршил бүү нэм. "+
			"Текст аль хэдийн зорилтот хэл дээр байвал хэвээр нь буцаа.", target)

	resp, err := uc.client.GenerateContent(ctx, gemini.Request{
		SystemInstruction: &gemini.Content{Parts: []gemini.Part{{Text: instruction}}},
		Contents:          []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: sourceText}}}},
	})
	if err != nil {
		return TranslateResult{}, apperror.InternalCause(fmt.Errorf("ai translate: %w", err))
	}
	translated := strings.TrimSpace(resp.Text())
	if translated == "" {
		return TranslateResult{}, apperror.InternalCause(fmt.Errorf("ai translate: empty translation"))
	}

	result := TranslateResult{SourceText: sourceText, Translated: translated}
	if req.Speak {
		audio, speakErr := uc.Speak(ctx, SpeakRequest{Text: translated})
		if speakErr != nil {
			// Дуут гаралт нэмэлт боломж — TTS унавал орчуулгаа дуугүй буцаана.
			return result, nil
		}
		result.Audio = &audio
	}
	return result, nil
}

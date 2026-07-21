// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"template/internal/apperror"
	"template/pkg/gemini"
)

// Дуу хоолойн боломжууд: Transcribe (STT) / Speak (TTS) / Translate.
// Чатаас ялгаатай нь эдгээр нь fallback мессеж буцаадаггүй — алдааг шууд
// error болгож өгнө (дуудагч UI өөрөө "дахин оролд" гэж харуулна).

// sttInstruction нь STT-ийн тогтмол дүрэм — зөвхөн сонссоноо буцаана.
const sttInstruction = "Чи яриа-текст (STT) хөрвүүлэгч. Өгсөн audio-д сонсогдсон яриаг " +
	"яг хэлсэн хэлээр нь, үг үсгийн алдаагүй, тайлбаргүйгээр зөвхөн текст болгон буцаа. " +
	"Яриа сонсогдохгүй бол хоосон мөр буцаа."

func (uc *usecase) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	resp, err := uc.client.GenerateContent(ctx, gemini.Request{
		SystemInstruction: &gemini.Content{Parts: []gemini.Part{{Text: sttInstruction}}},
		Contents: []gemini.Content{{
			Role: "user",
			Parts: []gemini.Part{
				{Text: "Энэ бичлэгийг текст болго."},
				{InlineData: &gemini.Blob{MimeType: req.Audio.Mime, Data: req.Audio.Data}},
			},
		}},
	})
	if err != nil {
		return TranscribeResult{}, apperror.InternalCause(fmt.Errorf("ai transcribe: %w", err))
	}
	return TranscribeResult{Text: resp.Text()}, nil
}

func (uc *usecase) Speak(ctx context.Context, req SpeakRequest) (SpeakResult, error) {
	voice := req.Voice
	if voice == "" {
		voice = uc.cfg.Voice
	}
	resp, err := uc.ttsClient.GenerateContent(ctx, gemini.Request{
		Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: req.Text}}}},
		GenerationConfig: &gemini.GenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: &gemini.SpeechConfig{
				VoiceConfig: &gemini.VoiceConfig{
					PrebuiltVoiceConfig: &gemini.PrebuiltVoiceConfig{VoiceName: voice},
				},
			},
		},
	})
	if err != nil {
		return SpeakResult{}, apperror.InternalCause(fmt.Errorf("ai speak: %w", err))
	}
	blob := resp.InlineAudio()
	if blob == nil {
		return SpeakResult{}, apperror.InternalCause(fmt.Errorf("ai speak: no audio in response"))
	}
	return toWAV(*blob)
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

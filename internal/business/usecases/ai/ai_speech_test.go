// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/pkg/gemini"
)

func audioResponse(mime string, raw []byte) gemini.Response {
	return gemini.Response{Candidates: []gemini.Candidate{{
		Content: gemini.Content{Role: "model", Parts: []gemini.Part{{
			InlineData: &gemini.Blob{MimeType: mime, Data: base64.StdEncoding.EncodeToString(raw)},
		}}},
	}}}
}

var testAudio = Audio{Mime: "audio/webm", Data: base64.StdEncoding.EncodeToString([]byte("fake-opus"))}

func TestTranscribe(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("Сайн байна уу")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	res, err := uc.Transcribe(context.Background(), TranscribeRequest{Audio: testAudio})
	require.NoError(t, err)
	assert.Equal(t, "Сайн байна уу", res.Text)

	// Audio inline хэсэг хүсэлтэд явсан байх ёстой.
	req := gen.requests[0]
	require.Len(t, req.Contents, 1)
	var hasAudio bool
	for _, p := range req.Contents[0].Parts {
		if p.InlineData != nil && p.InlineData.MimeType == "audio/webm" {
			hasAudio = true
		}
	}
	assert.True(t, hasAudio)
}

func TestTranscribeError(t *testing.T) {
	gen := &fakeGenerator{errs: []error{errors.New("boom")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	_, err := uc.Transcribe(context.Background(), TranscribeRequest{Audio: testAudio})
	require.Error(t, err)
}

// Түр зуурын саатал (хугацаа хэтрэлт, Gemini боломжгүй) нь 500 биш 503 —
// дахин оролдвол болох алдааг «дотоод алдаа» гэж хэлэх нь буруу дохио.
func TestSpeakTransientFailureIsUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "хугацаа хэтэрсэн", err: fmt.Errorf("gemini: http: %w", context.DeadlineExceeded)},
		{name: "gemini боломжгүй", err: fmt.Errorf("%w: 3 attempts failed", gemini.ErrUnavailable)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tts := &fakeGenerator{errs: []error{tt.err}}
			uc := NewUsecase(&fakeGenerator{}, tts, nil, nil, Config{})

			_, err := uc.Speak(context.Background(), SpeakRequest{Text: "сайн уу"})
			require.Error(t, err)
			assert.True(t, apperror.Is(err, apperror.ErrTypeUnavailable),
				"ErrTypeUnavailable хүлээж байна, гарсан: %v", err)
		})
	}
}

// TTS-ийн алдаанууд (аудиогүй хариу, model «унших» биш «хариулах» горимд орсны
// 400 г.м.) нь тогтмол биш тул дахин оролдоод эцэст нь 503 болно — дуудагчид
// «дараа дахин оролд» гэсэн зөв дохио өгнө. Бодит шалтгаан логд үлдэнэ.
func TestSpeakFailureAfterRetriesIsUnavailable(t *testing.T) {
	tts := &fakeGenerator{errs: []error{errors.New("boom"), errors.New("boom"), errors.New("boom")}}
	uc := NewUsecase(&fakeGenerator{}, tts, nil, nil, Config{})

	_, err := uc.Speak(context.Background(), SpeakRequest{Text: "сайн уу"})
	require.Error(t, err)
	assert.True(t, apperror.Is(err, apperror.ErrTypeUnavailable))
	assert.Len(t, tts.requests, speakAttempts)
}

// STT дээр ангилал өөрчлөгдөөгүй — бодит алдаа 500 хэвээр.
func TestTranscribeOtherFailureStaysInternal(t *testing.T) {
	gen := &fakeGenerator{errs: []error{errors.New("boom")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	_, err := uc.Transcribe(context.Background(), TranscribeRequest{Audio: testAudio})
	require.Error(t, err)
	assert.True(t, apperror.Is(err, apperror.ErrTypeInternal))
}

// TTS-д илгээх текст нь «унш, бүү хариул» зааврыг агуулна — үүнгүй бол model
// богино асуултыг уншихын оронд хариулах гэж оролддог (бодит 400).
func TestSpeakSendsReadAloudInstruction(t *testing.T) {
	pcm := []byte{0x01, 0x02}
	tts := &fakeGenerator{responses: []gemini.Response{audioResponse("audio/L16;codec=pcm;rate=24000", pcm)}}
	uc := NewUsecase(&fakeGenerator{}, tts, nil, nil, Config{})

	_, err := uc.Speak(context.Background(), SpeakRequest{Text: "eID гэж юу вэ?"})
	require.NoError(t, err)

	sent := tts.requests[0].Contents[0].Parts[0].Text
	assert.Contains(t, sent, "Do not answer it")
	assert.Contains(t, sent, "eID гэж юу вэ?", "хэрэглэгчийн текст өөрчлөгдөхгүй")
}

func TestSpeakWrapsPCMAsWAV(t *testing.T) {
	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	tts := &fakeGenerator{responses: []gemini.Response{audioResponse("audio/L16;codec=pcm;rate=24000", pcm)}}
	uc := NewUsecase(&fakeGenerator{}, tts, nil, nil, Config{})

	res, err := uc.Speak(context.Background(), SpeakRequest{Text: "Сайн уу"})
	require.NoError(t, err)
	assert.Equal(t, "audio/wav", res.Mime)

	wav, err := base64.StdEncoding.DecodeString(res.Data)
	require.NoError(t, err)
	assert.Equal(t, "RIFF", string(wav[:4]))
	assert.Equal(t, pcm, wav[44:])

	// TTS хүсэлт AUDIO modality + voice-той явсан байх ёстой.
	req := tts.requests[0]
	require.NotNil(t, req.GenerationConfig)
	assert.Equal(t, []string{"AUDIO"}, req.GenerationConfig.ResponseModalities)
	assert.Equal(t, defaultVoice, req.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName)
}

// TTS model заримдаа 200 буцаагаад аудиогүй хариу өгдөг (бодит хэмжилт).
// Бүх оролдлого хоосон бол 503 — дахин оролдоход эдгэрдэг түр саатал.
func TestSpeakNoAudioInResponse(t *testing.T) {
	tts := &fakeGenerator{responses: []gemini.Response{
		textResponse("за"), textResponse("за"), textResponse("за"),
	}}
	uc := NewUsecase(&fakeGenerator{}, tts, nil, nil, Config{})

	_, err := uc.Speak(context.Background(), SpeakRequest{Text: "x"})
	require.Error(t, err)
	assert.True(t, apperror.Is(err, apperror.ErrTypeUnavailable))
	assert.Len(t, tts.requests, speakAttempts, "хоосон хариу бүрд дахин оролдоно")
}

// Эхний хариу хоосон байсан ч дараагийнх нь аудиотай бол хэрэглэгч алдаа
// хардаггүй — энэ л retry-ийн гол зорилго.
func TestSpeakRetriesUntilAudio(t *testing.T) {
	pcm := []byte{0x01, 0x02}
	tts := &fakeGenerator{responses: []gemini.Response{
		textResponse("за"),
		audioResponse("audio/L16;codec=pcm;rate=24000", pcm),
	}}
	uc := NewUsecase(&fakeGenerator{}, tts, nil, nil, Config{})

	res, err := uc.Speak(context.Background(), SpeakRequest{Text: "x"})
	require.NoError(t, err)
	assert.Equal(t, "audio/wav", res.Mime)
	assert.Len(t, tts.requests, 2)
}

func TestTranslateText(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("Hello")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	res, err := uc.Translate(context.Background(), TranslateRequest{Text: "Сайн уу", TargetLang: "en"})
	require.NoError(t, err)
	assert.Equal(t, "Сайн уу", res.SourceText)
	assert.Equal(t, "Hello", res.Translated)
	assert.Nil(t, res.Audio)

	// Зорилтот хэлний нэр instruction-д орсон байх ёстой.
	require.NotNil(t, gen.requests[0].SystemInstruction)
	assert.Contains(t, gen.requests[0].SystemInstruction.Parts[0].Text, "English")
}

func TestTranslateAudioPipeline(t *testing.T) {
	// 1-р дуудлага: STT → "Сайн байна уу", 2-р дуудлага: орчуулга → "Hello".
	gen := &fakeGenerator{responses: []gemini.Response{
		textResponse("Сайн байна уу"),
		textResponse("Hello"),
	}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	res, err := uc.Translate(context.Background(), TranslateRequest{Audio: &testAudio, TargetLang: "en"})
	require.NoError(t, err)
	assert.Equal(t, "Сайн байна уу", res.SourceText)
	assert.Equal(t, "Hello", res.Translated)
	assert.Equal(t, 2, gen.calls)
}

func TestTranslateSilentAudioReturnsEmpty(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	res, err := uc.Translate(context.Background(), TranslateRequest{Audio: &testAudio, TargetLang: "en"})
	require.NoError(t, err)
	assert.Empty(t, res.SourceText)
	assert.Empty(t, res.Translated)
}

func TestTranslateWithSpeak(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("Hello")}}
	tts := &fakeGenerator{responses: []gemini.Response{audioResponse("audio/L16;rate=24000", []byte{9, 9})}}
	uc := NewUsecase(gen, tts, nil, nil, Config{})

	res, err := uc.Translate(context.Background(), TranslateRequest{Text: "Сайн уу", TargetLang: "en", Speak: true})
	require.NoError(t, err)
	require.NotNil(t, res.Audio)
	assert.Equal(t, "audio/wav", res.Audio.Mime)
}

func TestTranslateSpeakFailureStillReturnsText(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("Hello")}}
	tts := &fakeGenerator{errs: []error{errors.New("tts down")}}
	uc := NewUsecase(gen, tts, nil, nil, Config{})

	res, err := uc.Translate(context.Background(), TranslateRequest{Text: "Сайн уу", TargetLang: "en", Speak: true})
	require.NoError(t, err)
	assert.Equal(t, "Hello", res.Translated)
	assert.Nil(t, res.Audio)
}

func TestRunWithAudioMessage(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("Дуут мессежийг сонслоо")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	res, err := uc.Run(context.Background(), RunRequest{Audio: &testAudio})
	require.NoError(t, err)
	assert.Equal(t, "Дуут мессежийг сонслоо", res.Reply)

	// Сүүлийн user content нь audio part агуулсан байх ёстой.
	contents := gen.requests[0].Contents
	last := contents[len(contents)-1]
	require.Len(t, last.Parts, 1)
	assert.NotNil(t, last.Parts[0].InlineData)
}

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestSpeakNoAudioInResponse(t *testing.T) {
	tts := &fakeGenerator{responses: []gemini.Response{textResponse("за")}}
	uc := NewUsecase(&fakeGenerator{}, tts, nil, nil, Config{})

	_, err := uc.Speak(context.Background(), SpeakRequest{Text: "x"})
	require.Error(t, err)
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

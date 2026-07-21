// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ai нь Gemini-д суурилсан AI pipeline-ийг хэрэгжүүлнэ:
//
//	хэрэглэгчийн асуулт (текст/audio) → Gemini (function calling) →
//	backend tool гүйцэтгэл → үр дүнг Gemini руу буцаах → эцсийн Монгол хариулт
//
// AI ямар tool дуудахаа ШИЙДНЭ, backend ГҮЙЦЭТГЭНЭ — model хэзээ ч өөрөө
// код ажиллуулахгүй. Чатын хувьд Gemini бүх оролдлогын дараа ч амжилтгүй
// бол хэрэглэгчид Монгол fallback мессеж буцаана (хүсэлт унагахгүй).
//
// Мөн дуу хоолойн боломжууд: Transcribe (STT), Speak (TTS), Translate
// (текст/audio → зорилтот хэл, сонголтоор дуут гаралт) — live орчуулгын
// urskhal нь frontend-ээс жижиг audio chunk-уудыг Translate руу дамжуулж
// бүтдэг.
package ai

import (
	"context"

	"template/internal/business/domain"
)

type Usecase interface {
	// Run нь нэг чат хүсэлтийг pipeline-аар бүрэн гүйцэтгэж эцсийн
	// хариултыг буцаана. Gemini-ийн түр зуурын алдааг fallback мессежээр
	// (Degraded=true) намжаана; зөвхөн тохиргооны алдааг error болгоно.
	Run(ctx context.Context, req RunRequest) (RunResult, error)

	// ListPrompts нь тохируулдаг prompt давхаргуудыг буцаана (админ UI).
	ListPrompts(ctx context.Context) ([]domain.AIPrompt, error)
	// SetPrompt нь нэг давхаргын агуулгыг сольж, кэшийг хүчингүй болгоно.
	SetPrompt(ctx context.Context, key, content string) error

	// Transcribe нь audio-г текст болгоно (STT).
	Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error)

	// Speak нь текстийг яриа болгоно (TTS) — browser-т шууд тоглуулах
	// боломжтой WAV буцаана.
	Speak(ctx context.Context, req SpeakRequest) (SpeakResult, error)

	// Translate нь текст эсвэл audio-г зорилтот хэл рүү орчуулна; Speak
	// үнэн бол орчуулгын дуут (TTS) хувилбарыг хамт буцаана.
	Translate(ctx context.Context, req TranslateRequest) (TranslateResult, error)
}

type (
	// Audio нь base64 кодлогдсон оролтын дуу (browser MediaRecorder chunk).
	Audio struct {
		Mime string
		Data string // base64
	}

	// Turn нь өмнөх харилцааны нэг ээлж. Role: "user" | "model".
	Turn struct {
		Role string
		Text string
	}

	RunRequest struct {
		Prompt  string
		Audio   *Audio // сонголттой — дуут мессеж (audio ойлголт)
		History []Turn
	}

	// Step нь pipeline-ийн гүйцэтгэсэн нэг tool дуудлагын ул мөр —
	// frontend "AI юу хийснийг" харуулахад ашиглаж болно.
	Step struct {
		Tool   string
		Args   map[string]any
		Result map[string]any
	}

	RunResult struct {
		Reply string
		Steps []Step
		// Degraded нь Gemini амжилтгүй болж fallback мессеж буцаасныг заана.
		Degraded bool
	}

	TranscribeRequest struct {
		Audio Audio
	}

	TranscribeResult struct {
		Text string
	}

	SpeakRequest struct {
		Text  string
		Voice string // хоосон бол өгөгдмөл voice
	}

	SpeakResult struct {
		Mime string
		Data string // base64 (audio/wav)
	}

	TranslateRequest struct {
		Text       string // Text эсвэл Audio-гийн аль нэг нь заавал
		Audio      *Audio
		TargetLang string // ISO код эсвэл хэлний нэр ("mn", "en", ...)
		Speak      bool   // үнэн бол орчуулгыг TTS-ээр дуут болгож хавсаргана
	}

	TranslateResult struct {
		SourceText string // audio оролттой үед STT-ийн үр дүн
		Translated string
		Audio      *SpeakResult // Speak=true үед
	}
)

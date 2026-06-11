// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ai нь Gemini-д суурилсан AI pipeline-ийг хэрэгжүүлнэ:
//
//	хэрэглэгчийн асуулт → Gemini (function calling) → backend tool гүйцэтгэл
//	→ үр дүнг Gemini руу буцаах → эцсийн Монгол хариулт
//
// AI ямар tool дуудахаа ШИЙДНЭ, backend ГҮЙЦЭТГЭНЭ — model хэзээ ч өөрөө
// код ажиллуулахгүй. Gemini бүх оролдлогын дараа ч амжилтгүй бол хэрэглэгчид
// Монгол fallback мессеж буцаана (хүсэлт унагахгүй).
package ai

import (
	"context"
)

type Usecase interface {
	// Run нь нэг чат хүсэлтийг pipeline-аар бүрэн гүйцэтгэж эцсийн
	// хариултыг буцаана. Gemini-ийн түр зуурын алдааг fallback мессежээр
	// (Degraded=true) намжаана; зөвхөн тохиргооны алдааг error болгоно.
	Run(ctx context.Context, req RunRequest) (RunResult, error)
}

type (
	// Turn нь өмнөх харилцааны нэг ээлж. Role: "user" | "model".
	Turn struct {
		Role string
		Text string
	}

	RunRequest struct {
		Prompt  string
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
)

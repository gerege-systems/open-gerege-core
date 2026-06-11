// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	ai "template/internal/business/usecases/ai"
)

// AIChatStep нь pipeline-ийн гүйцэтгэсэн нэг tool дуудлага — frontend
// "AI юу хийснийг" харуулахад ашиглана.
type AIChatStep struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args,omitempty"`
	Result map[string]any `json:"result,omitempty"`
}

// AIChatResponse нь POST /ai/chat-ийн data хэсэг.
type AIChatResponse struct {
	Reply    string       `json:"reply"`
	Steps    []AIChatStep `json:"steps,omitempty"`
	Degraded bool         `json:"degraded,omitempty"`
}

// FromAIRunResult нь usecase-ийн үр дүнг HTTP DTO руу буулгана.
func FromAIRunResult(res ai.RunResult) AIChatResponse {
	steps := make([]AIChatStep, 0, len(res.Steps))
	for _, s := range res.Steps {
		steps = append(steps, AIChatStep{Tool: s.Tool, Args: s.Args, Result: s.Result})
	}
	return AIChatResponse{Reply: res.Reply, Steps: steps, Degraded: res.Degraded}
}

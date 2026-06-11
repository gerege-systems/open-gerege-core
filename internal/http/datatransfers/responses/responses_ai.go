// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/domain"
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

// AIAudioOut нь base64 кодлогдсон дуут гаралт (ихэвчлэн audio/wav).
type AIAudioOut struct {
	Mime string `json:"mime"`
	Data string `json:"data"`
}

// AISTTResponse нь POST /ai/stt-ийн data хэсэг.
type AISTTResponse struct {
	Text string `json:"text"`
}

// AITranslateResponse нь POST /ai/translate-ийн data хэсэг.
type AITranslateResponse struct {
	SourceText string      `json:"source_text"`
	Translated string      `json:"translated"`
	Audio      *AIAudioOut `json:"audio,omitempty"`
}

// AIPromptResponse нь тохируулдаг нэг prompt давхарга.
type AIPromptResponse struct {
	Key       string     `json:"key"`
	Content   string     `json:"content"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// ToAIPromptList нь domain prompt-уудыг HTTP DTO руу буулгана.
func ToAIPromptList(list []domain.AIPrompt) []AIPromptResponse {
	out := make([]AIPromptResponse, 0, len(list))
	for _, p := range list {
		out = append(out, AIPromptResponse{Key: p.Key, Content: p.Content, UpdatedAt: p.UpdatedAt})
	}
	return out
}

// FromAITranslateResult нь usecase-ийн орчуулгын үр дүнг HTTP DTO руу буулгана.
func FromAITranslateResult(res ai.TranslateResult) AITranslateResponse {
	out := AITranslateResponse{SourceText: res.SourceText, Translated: res.Translated}
	if res.Audio != nil {
		out.Audio = &AIAudioOut{Mime: res.Audio.Mime, Data: res.Audio.Data}
	}
	return out
}

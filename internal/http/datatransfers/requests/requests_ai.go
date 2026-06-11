// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// AIChatTurn нь өмнөх харилцааны нэг ээлж. role: "user" | "model".
type AIChatTurn struct {
	Role string `json:"role" validate:"required,oneof=user model"`
	Text string `json:"text" validate:"required,max=4000"`
}

// AIChatRequest нь POST /ai/chat-ийн body. History нь сонголттой —
// frontend өмнөх ээлжүүдээ дамжуулж харилцааг үргэлжлүүлнэ (сервер
// талд чат төлөв хадгалдаггүй, stateless).
type AIChatRequest struct {
	Message string       `json:"message" validate:"required,max=4000"`
	History []AIChatTurn `json:"history" validate:"omitempty,max=20,dive"`
}

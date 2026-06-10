// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package auth нь /auth/* HTTP endpoint-уудыг үйлчилдэг — register,
// login, OTP, refresh, logout. Хэрэглэгчийн профайлын endpoint-ууд нь
// ах дүү package болох internal/http/handlers/v1/users-д байрладаг.
package auth

import (
	"template/internal/business/usecases/auth"
)

// Handler нь auth-handler-ийн нэгтгэл; endpoint бүрийн method-ууд
// өөрсдийн файлд (auth.register.go, auth.login.go, г.м.) тодорхойлогддог
// тул нэг endpoint-д хүрэх PR diff-үүд бусад руу нэвчдэггүй.
type Handler struct {
	usecase auth.Usecase
}

func NewHandler(usecase auth.Usecase) Handler {
	return Handler{usecase: usecase}
}

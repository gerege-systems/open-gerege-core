// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package auth нь /auth/* HTTP endpoint-уудыг үйлчилдэг — register,
// login, OTP, refresh, logout. Хэрэглэгчийн профайлын endpoint-ууд нь
// ах дүү package болох internal/http/handlers/v1/users-д байрладаг.
package auth

import (
	"template/internal/business/usecases/audit"
	"template/internal/business/usecases/auth"
)

// Handler нь auth-handler-ийн нэгтгэл; endpoint бүрийн method-ууд
// өөрсдийн файлд (auth.register.go, auth.login.go, г.м.) тодорхойлогддог
// тул нэг endpoint-д хүрэх PR diff-үүд бусад руу нэвчдэггүй.
//
// auditUC нь persisted hash-chained audit log-д бичих use case (eID нэвтрэлт
// амжилттай болоход best-effort бичлэг хийнэ). nil байж болно — тэр үед audit
// бичлэг алгасагдана (тестүүдэд эсвэл audit идэвхгүй орчинд).
type Handler struct {
	usecase auth.Usecase
	auditUC audit.Usecase
}

func NewHandler(usecase auth.Usecase) Handler {
	return Handler{usecase: usecase}
}

// NewHandlerWithAudit нь audit use case-ийг тарьж handler үүсгэнэ.
func NewHandlerWithAudit(usecase auth.Usecase, auditUC audit.Usecase) Handler {
	return Handler{usecase: usecase, auditUC: auditUC}
}

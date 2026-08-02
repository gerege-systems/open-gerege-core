// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package auth нь /auth/* HTTP endpoint-уудыг үйлчилдэг — register,
// login, OTP, refresh, logout. Хэрэглэгчийн профайлын endpoint-ууд нь
// ах дүү package болох internal/http/handlers/v1/users-д байрладаг.
package auth

import (
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/audit"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/auth"
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
	// wallet нь гар утасны нэвтрэлт (MobileStatus) амжилттай болоход иргэний
	// түрийвчийг нээх/олоход хэрэглэгдэнэ. nil байж болно — тэр үед хариунд
	// IBAN ирэхгүй, нэвтрэлт өөрөө хэвийн үргэлжилнэ.
	wallet WalletProvisioner
}

func NewHandler(usecase auth.Usecase) Handler {
	return Handler{usecase: usecase}
}

// WithWallet нь handler-т түрийвч нээгчийг холбоно (сервер угсрах үед).
func (h Handler) WithWallet(w WalletProvisioner) Handler {
	h.wallet = w
	return h
}

// NewHandlerWithAudit нь audit use case-ийг тарьж handler үүсгэнэ.
func NewHandlerWithAudit(usecase auth.Usecase, auditUC audit.Usecase) Handler {
	return Handler{usecase: usecase, auditUC: auditUC}
}

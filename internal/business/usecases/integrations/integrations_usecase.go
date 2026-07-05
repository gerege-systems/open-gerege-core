// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"context"
	"time"

	"template/internal/business/domain"
)

// Usecase нь хэрэглэгчийн гуравдагч этгээдийн интеграцийн токеныг удирдана.
// Токенууд storage-д шифрлэгдэж хадгалагдана.
type Usecase interface {
	// Connect нь нэг провайдерын токеныг (шифрлээд) хадгална эсвэл шинэчилнэ.
	Connect(ctx context.Context, req ConnectRequest) (domain.UserIntegration, error)
	// List нь хэрэглэгчийн холбосон провайдеруудыг буцаана (токенгүй —
	// зөвхөн provider + хугацаа). Токены утга хариунд орохгүй.
	List(ctx context.Context, userID string) ([]ConnectedProvider, error)
	// Disconnect нь нэг холболтыг устгана (idempotent).
	Disconnect(ctx context.Context, userID, provider string) error
	// Token нь нэг провайдерын ШИФРГҮЙ токеныг буцаана — ЗӨВХӨН server-тал (BFF)
	// провайдерын API руу хандахад ашиглана; browser руу хэзээ ч гарахгүй.
	Token(ctx context.Context, userID, provider string) (TokenData, error)
}

// TokenData нь decrypt хийсэн токен (server-тал BFF-д л өгнө).
type TokenData struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
}

// ConnectRequest нь usecase-ийн хилийн оролт. ExpiresAt сонголттой.
type ConnectRequest struct {
	UserID       string
	Provider     string
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
}

// ConnectedProvider нь List-ийн буцаалт — токенгүй, аюулгүй далайц.
type ConnectedProvider struct {
	Provider    string
	ExpiresAt   *time.Time
	ConnectedAt time.Time
}

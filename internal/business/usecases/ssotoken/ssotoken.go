// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ssotoken нь иргэний dgov-SSO OAuth токенуудыг хадгалж, хэрэгцээт үед
// (SSO eID proxy дуудахад) хүчинтэй access token-ыг буцаана — хугацаа дуусах
// дөхсөн бол refresh_token-оор шинэчилнэ (offline_access шаардана).
package ssotoken

import (
	"context"
	"fmt"
	"time"

	"template/internal/business/domain"
	_interface "template/internal/datasources/repositories/interface"
	"template/pkg/logger"
	"template/pkg/oidc"
)

// refreshSkew нь access token хугацаа дуусахаас хэр өмнө урьдчилан refresh хийхийг
// заана — сүлжээ/цагийн зөрүүнд тэсвэртэй байхын тулд.
const refreshSkew = 60 * time.Second

// Service нь токен хадгалалт + refresh-ийг зохион байгуулна.
type Service struct {
	repo _interface.SSOTokenRepository
	oidc *oidc.Client
}

// New нь token service үүсгэнэ.
func New(repo _interface.SSOTokenRepository, oidcClient *oidc.Client) *Service {
	return &Service{repo: repo, oidc: oidcClient}
}

// Store нь нэвтрэлтийн дараа токенуудыг хадгална. refresh_token хоосон бол
// (offline_access аваагүй / native урсгал) хадгалахгүй — refresh боломжгүй тул.
func (s *Service) Store(ctx context.Context, userID string, tok oidc.Tokens) error {
	if tok.RefreshToken == "" {
		return nil
	}
	expiresAt := expiryFrom(tok.ExpiresIn)
	return s.repo.Upsert(ctx, userID, domain.SSOToken{
		AccessToken:     tok.AccessToken,
		RefreshToken:    tok.RefreshToken,
		AccessExpiresAt: expiresAt,
	})
}

// ValidAccessToken нь хэрэглэгчийн хүчинтэй SSO access token-ыг буцаана; хугацаа
// дуусах дөхсөн бол refresh хийж, шинийг хадгалаад буцаана. Хадгалагдсан токен
// байхгүй бол domain.ErrSSOTokenNotFound (дахин нэвтрэх шаардлагатай).
func (s *Service) ValidAccessToken(ctx context.Context, userID string) (string, error) {
	stored, err := s.repo.Get(ctx, userID)
	if err != nil {
		return "", err // domain.ErrSSOTokenNotFound-ыг дуудагч руу дамжуулна
	}
	if time.Until(stored.AccessExpiresAt) > refreshSkew {
		return stored.AccessToken, nil
	}
	// Хугацаа дуусах дөхсөн — refresh.
	refreshed, rErr := s.oidc.Refresh(ctx, stored.RefreshToken)
	if rErr != nil {
		return "", fmt.Errorf("sso token refresh: %w", rErr)
	}
	if err := s.repo.Upsert(ctx, userID, domain.SSOToken{
		AccessToken:     refreshed.AccessToken,
		RefreshToken:    refreshed.RefreshToken,
		AccessExpiresAt: expiryFrom(refreshed.ExpiresIn),
	}); err != nil {
		// Хадгалж чадаагүй ч дуудлагыг нэг удаа гүйцээхийн тулд шинэ token-ыг
		// буцаана (дараагийн удаа дахин refresh хийнэ).
		logger.ErrorWithContext(ctx, "ssotoken: failed to persist refreshed token (non-fatal)", logger.Fields{"error": err.Error()})
	}
	return refreshed.AccessToken, nil
}

// expiryFrom нь expires_in (секунд)-ээс access token-ий дуусах агшинг гаргана.
// 0 буюу сөрөг бол одоо (даруй refresh хийлгэнэ).
func expiryFrom(expiresIn int) time.Time {
	if expiresIn <= 0 {
		return time.Now()
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}
